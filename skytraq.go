package skytraq

import (
	"time"

	"github.com/sirupsen/logrus"
)

type MessageCallbacks struct {
	SoftwareVersion func(SoftwareVersion)
	NavData func(NavData)
}

func Run(cb MessageCallbacks) {
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
			c = Connect()
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
				cb.SoftwareVersion(f.softwareVersion())
			}
		case ResponseNavData:
			if cb.NavData != nil {
				cb.NavData(f.navData())
			}
		}
	}
}
