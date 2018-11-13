package skytraq

import (
	"bytes"
	"encoding/binary"
	"github.com/jd3nn1s/serial"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

type MockSerialPort struct {
	ReadBuf    bytes.Buffer
	WriteBuf   bytes.Buffer
	writeLimit int
	readLimit  int
	closed     bool
}

var versionData = []byte{0, 0, 1, 2, 3, 0, 4, 5, 6, 0, 7, 8, 9}
var navData []byte

func init() {
	buf := bytes.Buffer{}

	buf.Write([]byte{byte(Fix3D), 0, 3, 0, 0, 0, 0, 0})
	binary.Write(&buf, binary.BigEndian, uint32(1))
	binary.Write(&buf, binary.BigEndian, uint32(2))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint32(3))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint16(4))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint32(5))
	binary.Write(&buf, binary.BigEndian, uint32(6))
	binary.Write(&buf, binary.BigEndian, uint32(7))

	navData = buf.Bytes()
}

func (port *MockSerialPort) Flush() error {
	// ignore flush so that test data is not disturbed
	return nil
}

func (port *MockSerialPort) Read(p []byte) (n int, err error) {
	if port.readLimit > 0 && len(p) > port.readLimit {
		p = p[:port.readLimit]
	}
	return port.ReadBuf.Read(p)
}

func (port *MockSerialPort) Write(p []byte) (n int, err error) {
	if port.writeLimit > 0 {
		spaceLeft := port.WriteBuf.Len() - port.writeLimit
		if spaceLeft < 0 {
			spaceLeft = 0
		}
		if spaceLeft < len(p) {
			return port.WriteBuf.Write(p[:spaceLeft])
		}
	}
	return port.WriteBuf.Write(p)
}

func (port *MockSerialPort) Close() error {
	if port.closed {
		return errors.New("already closed")
	}
	port.closed = true
	return nil
}

func addACK(m *MockSerialPort, id MessageID) {

}

func connection() (*Connection, *MockSerialPort) {
	m := MockSerialPort{}
	return &Connection{
		port: &m,
	}, &m
}

func TestWriteBytes(t *testing.T) {
	c, m := connection()
	testData := []byte{0x1, 0x2, 0x3}
	err := c.writeBytes(testData)
	assert.NoError(t, err)

	readBuf := make([]byte, len(testData)+1)
	n, err := m.WriteBuf.Read(readBuf)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, readBuf[:len(testData)])
}

func TestWriteBytesTruncated(t *testing.T) {
	c, m := connection()
	testData := []byte{0x1, 0x2, 0x3}
	m.writeLimit = 1
	err := c.writeBytes(testData)
	assert.Error(t, err)
}

func TestReadBytes(t *testing.T) {
	c, m := connection()
	testData := []byte{0x1, 0x2, 0x3}
	m.ReadBuf.Write(testData)

	readData := make([]byte, 3)
	assert.NoError(t, c.readBytes(readData))
	assert.Equal(t, testData, readData)
}

func TestReadSplitBytes(t *testing.T) {
	c, m := connection()
	m.readLimit = 2
	testData := []byte{0x1, 0x2, 0x3}
	m.ReadBuf.Write(testData)

	readData := make([]byte, 3)
	assert.NoError(t, c.readBytes(readData))
	assert.Equal(t, testData, readData)
}

func frameData(id MessageID, data []byte, chksum byte) []byte {
	if chksum == 0 {
		chksum = checksum(id, data)
	}

	buf := bytes.Buffer{}
	buf.Write([]byte{0xa0, 0xa1})
	binary.Write(&buf, binary.BigEndian, uint16(len(data)+1))
	buf.WriteByte(byte(id))
	buf.Write(data)
	buf.Write([]byte{chksum, 0x0d, 0x0a})
	return buf.Bytes()
}

func TestReadFrame(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write(frameData(ResponseSoftwareVersion, versionData, 0))

	f, err := c.ReadFrame()
	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, ResponseSoftwareVersion, f.ID)
	assert.Equal(t, versionData, f.Data)
}

func TestReadFrameWrongChecksum(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write(frameData(ResponseSoftwareVersion, versionData, 5))

	_, err := c.ReadFrame()
	assert.Error(t, err)
}

func TestReadFrameMisaligned2(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write([]byte{0xff, 0xff})
	m.ReadBuf.Write(frameData(ResponseACK, []byte{}, 0))

	f, err := c.ReadFrame()
	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, ResponseACK, f.ID)
	assert.Equal(t, 0, len(f.Data))
}

func TestReadFrameMisaligned3(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write([]byte{0xff, 0xff, 0xff})
	m.ReadBuf.Write(frameData(ResponseACK, []byte{}, 0))

	f, err := c.ReadFrame()
	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, ResponseACK, f.ID)
	assert.Equal(t, 0, len(f.Data))
}

func TestReadFrameMisaligned9(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	m.ReadBuf.Write(frameData(ResponseACK, []byte{}, 0))

	f, err := c.ReadFrame()
	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, ResponseACK, f.ID)
	assert.Equal(t, 0, len(f.Data))
}

func TestReadFrameMisalignedWithFirstMarker(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write([]byte{0xa0})
	m.ReadBuf.Write(frameData(ResponseACK, []byte{}, 0))

	f, err := c.ReadFrame()
	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, ResponseACK, f.ID)
	assert.Equal(t, 0, len(f.Data))
}

func TestReadFrameMisalignedWith5FirstMarkers(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write([]byte{0xa0, 0xa0, 0xa0, 0xa0, 0xa0})
	m.ReadBuf.Write(frameData(ResponseACK, []byte{}, 0))

	f, err := c.ReadFrame()
	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, ResponseACK, f.ID)
	assert.Equal(t, 0, len(f.Data))
}

func TestReadACK(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write(frameData(ResponseACK, []byte{2}, 0))
	assert.NoError(t, c.readACK(2))
}

func TestReadACKWrongIDFirst(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write(frameData(ResponseACK, []byte{2}, 0))
	m.ReadBuf.Write(frameData(ResponseACK, []byte{3}, 0))
	assert.NoError(t, c.readACK(3))
}

func TestReadACKWrongType(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write(frameData(ResponseNavData, []byte{2}, 0))
	m.ReadBuf.Write(frameData(ResponseACK, []byte{3}, 0))
	assert.NoError(t, c.readACK(3))
}

func TestReadACKMaxWrongType(t *testing.T) {
	c, m := connection()

	for i := 0; i < maxIncorrectMessageIDCount+1; i++ {
		m.ReadBuf.Write(frameData(ResponseNavData, []byte{2}, 0))
	}
	m.ReadBuf.Write(frameData(ResponseACK, []byte{3}, 0))
	assert.EqualError(t, c.readACK(3),
		"too many irrelevant messages while waiting for ACK/NACK for message ID 3")
}

func TestReadNACK(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write(frameData(ResponseNACK, []byte{2}, 0))
	assert.EqualError(t, c.readACK(2), "received NACK for ID 2 on attempt to send 2")
}

func TestReadNACKWrongIDFirst(t *testing.T) {
	c, m := connection()

	m.ReadBuf.Write(frameData(ResponseNACK, []byte{3}, 0))
	m.ReadBuf.Write(frameData(ResponseNACK, []byte{2}, 0))
	assert.EqualError(t, c.readACK(2), "received NACK for ID 2 on attempt to send 2")
}

func TestInternalWriteFrame(t *testing.T) {
	c, m := connection()

	assert.NoError(t, c.writeFrame(&Frame{
		ID:   CommandQuerySoftwareVersion,
		Data: []byte{1},
	}))

	assert.Equal(t, 9, m.WriteBuf.Len())
	assert.Equal(t,
		frameData(CommandQuerySoftwareVersion, []byte{1}, 0),
		m.WriteBuf.Bytes())
}

func TestInternalWriteFrameFail(t *testing.T) {
	c, m := connection()
	m.writeLimit = 7

	assert.Error(t, c.writeFrame(&Frame{
		ID:   CommandQuerySoftwareVersion,
		Data: []byte{1},
	}))
}

func TestWriteFrame(t *testing.T) {
	c, m := connection()
	oldMaxWriteRetries := maxWriteRetries
	defer func() {
		maxWriteRetries = oldMaxWriteRetries
	}()
	maxWriteRetries = 1

	m.ReadBuf.Write(frameData(ResponseACK, []byte{byte(CommandQuerySoftwareVersion)}, 0))
	assert.NoError(t, c.WriteFrame(&Frame{
		ID:   CommandQuerySoftwareVersion,
		Data: []byte{1},
	}))
}

func TestWriteFrameNoAck(t *testing.T) {
	c, _ := connection()
	oldMaxWriteRetries := maxWriteRetries
	defer func() {
		maxWriteRetries = oldMaxWriteRetries
	}()

	maxWriteRetries = 1
	assert.Error(t, c.WriteFrame(&Frame{
		ID:   CommandQuerySoftwareVersion,
		Data: []byte{1},
	}))
}

func TestWriteFrameNack(t *testing.T) {
	c, m := connection()
	oldMaxWriteRetries := maxWriteRetries
	defer func() {
		maxWriteRetries = oldMaxWriteRetries
	}()
	maxWriteRetries = 1

	m.ReadBuf.Write(frameData(ResponseNACK, []byte{byte(CommandQuerySoftwareVersion)}, 0))
	assert.Error(t, c.WriteFrame(&Frame{
		ID:   CommandQuerySoftwareVersion,
		Data: []byte{1},
	}))
}

func TestWriteFrameRetry(t *testing.T) {
	c, m := connection()
	m.ReadBuf.Write(frameData(ResponseNACK, []byte{byte(CommandQuerySoftwareVersion)}, 0))
	m.ReadBuf.Write(frameData(ResponseACK, []byte{byte(CommandQuerySoftwareVersion)}, 0))
	assert.NoError(t, c.WriteFrame(&Frame{
		ID:   CommandQuerySoftwareVersion,
		Data: []byte{1},
	}))
	commandBuf := frameData(CommandQuerySoftwareVersion, []byte{1}, 0)
	// write buffer should contain two sent commands
	assert.Equal(t, m.WriteBuf.Bytes(), bytes.Join([][]byte{commandBuf, commandBuf}, []byte{}))
}

func TestConnectClose(t *testing.T) {
	oldOpenPort := openPort
	defer func() {
		openPort = oldOpenPort
	}()
	m := MockSerialPort{}
	openPort = func(config *serial.Config) (SerialPort, error) {
		return &m, nil
	}

	m.ReadBuf.Write(frameData(ResponseACK, []byte{byte(CommandQuerySoftwareVersion)}, 0))
	m.ReadBuf.Write(frameData(ResponseSoftwareVersion, versionData, 0))
	c, err := Connect("fakeport")
	assert.NoError(t, err)

	assert.NoError(t, c.Close())
	assert.Error(t, c.Close())
}
