package skytraq

import (
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type MessageID byte
type Frame struct {
	ID   MessageID
	Data []byte
}

type Version struct {
	Major int
	Minor int
	Patch int
}

type SoftwareVersion struct {
	Kernel   Version
	ODM      Version
	Revision Version
}

type FixMode uint8

type NavData struct {
	Fix            FixMode
	SatelliteCount int
	Latitude       int
	Longitude      int
	Altitude       int
	VX             int
	VY             int
	VZ             int
	HDOP           int
}

func (sv Version) String() string {
	return fmt.Sprintf("%v.%v.%v", sv.Major, sv.Minor, sv.Patch)
}

func (sv SoftwareVersion) String() string {
	return fmt.Sprintf("GPS kernel version: %v - ODM version: %v - Revision: %v",
		sv.Kernel, sv.ODM, sv.Revision)
}

const (
	ResponseSoftwareVersion MessageID = 0x80
	ResponseSoftwareCRC     MessageID = 0x81
	ResponseACK             MessageID = 0x83
	ResponseNACK            MessageID = 0x84
	ResponsePositionRate    MessageID = 0x86
	ResponseNavData         MessageID = 0xA8
	ResponseEphemerisData   MessageID = 0xB1
	ResponsePowerMode       MessageID = 0xB9
)

const (
	CommandSystemRestart        MessageID = 0x01
	CommandQuerySoftwareVersion MessageID = 0x02
	CommandQuerySoftwareCRC     MessageID = 0x03
	CommandQueryPositionRate    MessageID = 0x10
	CommandQueryPowerMode       MessageID = 0x15
	CommandGetEphermeris        MessageID = 0x30
)

const (
	FixNone       FixMode = 0
	Fix2D                 = 1
	Fix3D                 = 2
	Fix3DAndDGNSS         = 3
)

func checksum(id MessageID, data []byte) byte {
	cs := byte(id)
	for _, v := range data {
		cs = cs ^ v
	}
	return cs
}

func (f *Frame) ackMessageID() MessageID {
	return MessageID(f.Data[0])
}

func (f *Frame) softwareVersion() (SoftwareVersion, error) {
	const expectedLen = 13
	if len(f.Data) != expectedLen {
		logrus.WithField("length", len(f.Data)).
			WithField("expectedLen", expectedLen).Error("expecting more data")
		return SoftwareVersion{}, errors.Errorf("softwareVersion conversion requires %v bytes but received %v",
			expectedLen, len(f.Data))
	}
	return SoftwareVersion{
		Kernel:   Version{int(f.Data[2]), int(f.Data[3]), int(f.Data[4])},
		ODM:      Version{int(f.Data[6]), int(f.Data[7]), int(f.Data[8])},
		Revision: Version{2000 + int(f.Data[10]), int(f.Data[11]), int(f.Data[12])},
	}, nil
}

func (f *Frame) navData() (NavData, error) {
	const expectedLen = 58
	if len(f.Data) != expectedLen {
		logrus.WithField("length", len(f.Data)).
			WithField("expectedLen", expectedLen).Error("expecting more data")
		return NavData{}, errors.Errorf("navdata conversion requires %v bytes but received %v", expectedLen,
			len(f.Data))
	}

	return NavData{
		Fix:            FixMode(f.Data[0]),
		SatelliteCount: int(f.Data[2]),
		Latitude:       int(binary.BigEndian.Uint32(f.Data[8:12])),
		Longitude:      int(binary.BigEndian.Uint32(f.Data[12:16])),
		Altitude:       int(binary.BigEndian.Uint32(f.Data[20:24])),
		HDOP:           int(binary.BigEndian.Uint16(f.Data[28:30])),
		VX:             int(binary.BigEndian.Uint32(f.Data[46:50])),
		VY:             int(binary.BigEndian.Uint32(f.Data[50:54])),
		VZ:             int(binary.BigEndian.Uint32(f.Data[54:58])),
	}, nil
}
