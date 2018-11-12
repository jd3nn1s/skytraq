package skytraq

import (
	"bytes"
	"github.com/jd3nn1s/serial"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	DataMaxSize   = 65535
	EndMarkerSize = 3
	baud          = 230400

	maxIncorrectMessageIDCount = 5
	maxWriteRetries = 3
)

// All binary protocol data is big endian

type Connection struct {
	portConfig *serial.Config
	port       *serial.Port

	// max data size + checksum + end of sequence marker
	buf [DataMaxSize + EndMarkerSize]byte
}

func Connect() *Connection {
	c := &serial.Config{
		Name:        "/dev/ttyAMA0",
		Baud:        baud,
		ReadTimeout: 10 * time.Second,
	}

	conn := Connection{
		portConfig: c,
	}

	_ = conn.Open()
	return &conn
}

func (c *Connection) Open() error {
	var err error
	c.port, err = serial.OpenPort(c.portConfig)
	if err != nil {
		return err
	}
	return c.port.Flush()
}

func (c *Connection) Close() error {
	if c.port != nil {
		return c.port.Close()
	}
	return nil
}

// A canonical read from a serial port reads a complete "line" from the port. A line is not always
// the requested size and therefore this function will perform additional reads until the supplied
// buffer is full.
func (c *Connection) readBytes(buf []byte) error {
	targetSize := len(buf)
	startPos := 0
	for {
		readingSize := targetSize - startPos
		logrus.Debugf("reading buffer size: %v", readingSize)
		if n, err := c.port.Read(buf[startPos:targetSize]); err != nil {
			return errors.Wrapf(err, "unable to read data and end of frame")
		} else {
			if n == 0 {
				return errors.New("EOF")
			}
			startPos += n
			if startPos == targetSize {
				break
			}
			logrus.Debugf("incomplete read: wanted: %v received: %v bufsize: %v", readingSize, n, len(buf))
		}
	}
	return nil
}

// Read a Skytraq frame from the open connection. As devices can be continuously sending data it is
// possible that an incomplete frame could be received. This data is ignored and the read will still
// succeed.
func (c *Connection) ReadFrame() (*Frame, error) {
	// pre-amble and length of data
	var startBuf [4]byte

PreambleFind:
	for {
		preamblePos := 0
		if err := c.readBytes(startBuf[:]); err != nil {
			return nil, errors.Wrapf(err, "unable to read start of frame")
		}

		for ;startBuf[preamblePos] != 0xa0; preamblePos++ {
			if preamblePos == 3 {
				continue PreambleFind
			}
		}
		if preamblePos > 0 {
			logrus.WithField("offset", preamblePos).Info("misaligned data received")
			for copyPos := preamblePos; copyPos <= 3; copyPos++ {
				startBuf[copyPos-preamblePos] = startBuf[copyPos]
			}
			c.readBytes(startBuf[preamblePos:])
		}

		// TODO: preamble could be at pos 1+
		if startBuf[1] != 0xa1 {
			continue PreambleFind
		}
		break PreambleFind
	}

	size := int(uint16(startBuf[2])<<8 + uint16(startBuf[3]))
	logrus.WithField("payloadSize", size).Debug()
	tmpBuf := c.buf[:size+EndMarkerSize]
	if err := c.readBytes(tmpBuf); err != nil {
		return nil, errors.Wrapf(err, "unable to read data and end of frame")
	}

	if !bytes.Equal(c.buf[size+1:size+3], []byte{0x0d, 0x0a}) {
		return nil, errors.New("could not find end of frame marker")
	}

	f := &Frame{
		ID:   MessageID(c.buf[0]),
		Data: c.buf[1:size],
	}
	cs := f.checksum()
	logrus.WithField("checksum", cs).Debug()
	if cs != c.buf[size] {
		return nil, errors.Errorf("expected checksum %v but found %v", cs, c.buf[size])
	}
	logrus.Debugf("found frame %+v", f)
	return f, nil
}

func (c *Connection) writeFrame(f *Frame) error {
	lenPayload := len(f.Data) + 1 // includes ID
	startSendBuf := [5]byte{
		0xa0,
		0xa1,
		byte(lenPayload>>8) & 0xff,
		byte(lenPayload & 0xff),
		byte(f.ID),
	}

	if err := c.writeBytes(startSendBuf[:]); err != nil {
		return err
	}

	logrus.Infof("sending message ID %X with %v", f.ID, f.Data)
	if err := c.writeBytes(f.Data); err != nil {
		return err
	}

	endSendBuf := [3]byte{
		f.checksum(),
		0x0d,
		0x0a,
	}
	if err := c.writeBytes(endSendBuf[:]); err != nil {
		return err
	}
	return nil
}

func (c *Connection) writeBytes(buf []byte) error {
	s, err := c.port.Write(buf)
	if s != len(buf) || err != nil {
		if err != nil {
			return err
		}
		return errors.New("did not write expected number of bytes")
	}
	return nil
}

// Write a frame to an open connection. Confirms receipt of the frame by looking for an ACK frame
// from the device. If a NACK is received, or an error occurs, the frame send is retried.
//
// If while waiting for a non-ACK/NACK frame a different type is received, it is ignored. If too
// many non-ACK/NACK frames are received an error is returned.
func (c *Connection) WriteFrame(f *Frame) error {
	var err error
	retries := maxWriteRetries

	for ; retries > 0; retries-- {
		if err != nil {
			logrus.Error(err)
		}
		if err := c.writeFrame(f); err != nil {
			logrus.Error(err)
			return err
		}
		err = c.readACK(f.ID)
		if err == nil {
			break
		}
	}

	if retries == 0 {
		return errors.Wrapf(err, "exceeded retries")
	} else if retries < maxWriteRetries {
		logrus.WithField("retryCount", maxWriteRetries - retries).Warn("write frame successful after retry")
	}
	return nil
}

// Read until an ACK or NACK is received for the supplied message ID, or until there have been too many
// irrelevant messages
func (c *Connection) readACK(id MessageID) error {
	irrelevantFrameCount := 0
	for {
		respFrame, err := c.ReadFrame()
		if err != nil {
			return errors.Wrapf(err, "error when reading response frame")
		}
		switch respFrame.ID {
		case ResponseACK:
			if respFrame.ackMessageID() == id {
				logrus.WithField("messageID", id).Debug("received expected ACK")
				return nil
			}
			logrus.WithField("messageID", respFrame.ackMessageID()).
				WithField("irrelevantFrameCount", irrelevantFrameCount).Warn("unexpected ACK")
			irrelevantFrameCount++
		case ResponseNACK:
			if respFrame.ackMessageID() == id {
				return errors.Errorf("received NACK for ID %v on attempt to send %v", respFrame.ackMessageID(), id)
			}
			logrus.WithField("messageID", respFrame.ackMessageID()).
				WithField("irrelevantFrameCount", irrelevantFrameCount).Warn("unexpected NACK")
			irrelevantFrameCount++
		default:
			logrus.WithField("messageID", respFrame.ID).Debug("ignoring non-ACK/NACK frame", respFrame.ID)
			irrelevantFrameCount++
		}
		if irrelevantFrameCount > maxIncorrectMessageIDCount {
			return errors.Errorf("too many irrelevant messages while waiting for ACK/NACK for message ID %v", id)
		}
	}
}