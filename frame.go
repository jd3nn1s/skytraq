package skytraq

import (
	"fmt"
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
	ResponseSoftwareCRC               = 0x81
	ResponseACK                       = 0x83
	ResponseNACK                      = 0x84
	ResponsePositionRate              = 0x86
	ResponseNavData                   = 0xA8
	ResponseEphemerisData             = 0xB1
	ResponsePowerMode                 = 0xB9
)

const (
	CommandSystemRestart        MessageID = 0x01
	CommandQuerySoftwareVersion           = 0x02
	CommandQuerySoftwareCRC               = 0x03
	CommandQueryPositionRate              = 0x10
	CommandQueryPowerMode                 = 0x15
	CommandGetEphermeris                  = 0x30
)

const (
	FixNone       FixMode = 0
	Fix2D                 = 1
	Fix3D                 = 2
	Fix3DAndDGNSS         = 3
)

func (f *Frame) checksum() byte {
	cs := byte(f.ID)
	for _, v := range f.Data {
		cs = cs ^ v
	}
	return cs
}

func (f *Frame) ackMessageID() MessageID {
	return MessageID(f.Data[0])
}

func (f *Frame) softwareVersion() SoftwareVersion {
	const expectedLen = 13
	if len(f.Data) != expectedLen {
		logrus.WithField("length", len(f.Data)).
			WithField("expectedLen", expectedLen).Error("expecting more data")
		return SoftwareVersion{}
	}
	return SoftwareVersion{
		Kernel:   Version{int(f.Data[2]), int(f.Data[3]), int(f.Data[4])},
		ODM:      Version{int(f.Data[6]), int(f.Data[7]), int(f.Data[8])},
		Revision: Version{2000 + int(f.Data[10]), int(f.Data[11]), int(f.Data[12])},
	}
}

func (f *Frame) navData() NavData {
	const expectedLen = 58
	if len(f.Data) != expectedLen {
		logrus.WithField("length", len(f.Data)).
			WithField("expectedLen", expectedLen).Error("expecting more data")
		return NavData{}
	}
	return NavData{
		Fix:            FixMode(f.Data[0]),
		SatelliteCount: int(f.Data[2]),
		Latitude:       bytesToInt32(f.Data[8:12]),
		Longitude:      bytesToInt32(f.Data[12:16]),
		Altitude:       bytesToInt32(f.Data[20:24]),
		HDOP:           bytesToInt16(f.Data[28:30]),
		VX:             bytesToInt32(f.Data[46:50]),
		VY:             bytesToInt32(f.Data[50:54]),
		VZ:             bytesToInt32(f.Data[54:58]),
	}
}

func bytesToInt32(b []byte) int {
	return int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
}

func bytesToInt16(b []byte) int {
	return int(b[0])<<8 | int(b[1])
}
