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
	return c.port.Close()
}

func (c *Connection) ReadFrame() (*Frame, error) {

	// pre-amble and length of data
	var startBuf [4]byte

	PreambleFind:
	for {
		preamblePos := 0
		if _, err := c.port.Read(startBuf[:]); err != nil {
			return nil, errors.Wrapf(err, "unable to read start of frame")
		}

		for startBuf[preamblePos] != 0xa0 {
			logrus.Errorf("pos: %d value: %v", preamblePos, startBuf[preamblePos])
			if preamblePos == 3 {
				//return nil, errors.Errorf("unable to find preamble byte 1")
				//logrus.Error("unable to find preamble byte 1")
				continue PreambleFind
			}
			preamblePos++
		}
		if preamblePos > 0 {
			logrus.WithField("offset", preamblePos).Error("misaligned data received")
			logrus.Errorf("before: %v", startBuf)
			for copyPos := preamblePos; copyPos <= 3; copyPos++ {
				startBuf[copyPos-preamblePos] = startBuf[copyPos]
			}
			c.port.Read(startBuf[preamblePos:])
			logrus.Errorf("after: %v", startBuf)
		}

		if startBuf[1] != 0xa1 {
			continue PreambleFind
			//return nil, errors.Errorf("unable to find preamble byte 2")
		}
		break PreambleFind
	}

	size := uint16(startBuf[2])<<8 + uint16(startBuf[3])
	logrus.Errorf("size: %v", size)
	if _, err := c.port.Read(c.buf[:size+EndMarkerSize]); err != nil {
		return nil, errors.Wrapf(err, "unable to read data and end of frame")
	}

	logrus.Errorf("buf: %v", c.buf[:size+EndMarkerSize+3])
	if !bytes.Equal(c.buf[size+1:size+3], []byte{0x0d, 0x0a}) {
		return nil, errors.New("could not find end of frame marker")
	}

	f := &Frame{
		ID:   MessageID(c.buf[0]),
		Data: c.buf[1 : size],
	}
	cs := f.checksum()
	logrus.Errorf("checksum: %v", cs)
	if cs != c.buf[size] {
		return nil, errors.Errorf("expected checksum %v but found %v", cs, c.buf[size])
	}
	logrus.Errorf("found frame %+v", f)

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

	logrus.Errorf("sendbytes: %v", startSendBuf)
	logrus.Errorf("sendbytes: %v", f.Data)
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

func (c *Connection) WriteFrame(f *Frame) error {
	var err error
	retries := 3
	for ; retries > 0; retries-- {
		if err != nil {
			logrus.Error(err)
		}
		if err := c.writeFrame(f); err != nil {
			logrus.Error(err)
			return err
		}

		respFrame, err := c.ReadFrame()
		if err != nil {
			err = errors.Wrapf(err, "error when reading response frame")
			logrus.Error(err)
			continue
		}
		switch respFrame.ID {
		case ResponseACK:
			break
		case ResponseNACK:
			err = errors.Errorf("received NACK on attempt to send %v", f.ID)
			logrus.Error(err)
			continue
		default:
			err = errors.Errorf("unexpected message ID for response frame: %v", respFrame.ID)
			continue
		}

		if f.ackMessageID() != f.ID {
			err = errors.Errorf("expected ack message ID %v but received %v", f.ID, f.ackMessageID())
			continue
		}
	}
	if retries == 0 {
		return errors.Wrapf(err, "exceeded retries")
	}
	return nil
}
