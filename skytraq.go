package skytraq

import (
	"time"

	"github.com/sirupsen/logrus"
)

type MessageCallbacks struct {
	SoftwareVersion func(SoftwareVersion)
	NavData func(NavData)
}

func Run(portName string, cb MessageCallbacks) {
	var err error
	var f *Frame
	var c *Connection
	for {
		if err != nil {
			logrus.Error("reconnecting due to error ", err)
			if c != nil {
				c.Close()
			}
			c = nil
			time.Sleep(time.Second)
		}
		if c == nil {
			c = Connect(portName)
			err = c.Open()
			if err != nil {
				continue
			}

			err = c.WriteFrame(&Frame{
				ID:   CommandQuerySoftwareVersion,
				Data: []byte{1},
			})
			if err != nil {
				continue
			}
		}

		f, err = c.ReadFrame()
		if err != nil {
			continue
		}

		// successfully read frame, dispatch it
		switch f.ID {
		case ResponseSoftwareVersion:
			if cb.SoftwareVersion != nil {
				version, err := f.softwareVersion()
				if err != nil {
					logrus.WithField("err", err).Error("error when converting to SoftwareVersion structure")
					continue
				}
				cb.SoftwareVersion(version)
			}
		case ResponseNavData:
			if cb.NavData != nil {
				navData, err := f.navData()
				if err != nil {
					logrus.WithField("err", err).Error("error when converting to NavData structure")
					continue
				}
				cb.NavData(navData)
			}
		}
	}
}
