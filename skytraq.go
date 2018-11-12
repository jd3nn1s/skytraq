package skytraq

import (
	"time"

	"github.com/sirupsen/logrus"
)

func Run() {
	// skytraq_read_software_version

	var err error
	var f *Frame
	var c *Connection
	for {
		if err != nil {
			logrus.Error("reconnecting due to error", err)
			c.Close()
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

			f, err = c.ReadFrame()
			if err != nil {
				continue
			}

			logrus.Info(f.softwareVersion())
		}

		// SKYTRAQ_RESPONSE_NAVIGATION_DATA
		f, err = c.ReadFrame()
		if err != nil {
			continue
		}
		if f.ID == ResponseNavData {

		}
	}
}
