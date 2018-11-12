package skytraq

import (
	"github.com/pkg/errors"
)

type MessageCallbacks struct {
	SoftwareVersion func(SoftwareVersion)
	NavData func(NavData)
}

func (c *Connection) Start(cb MessageCallbacks) error {
	for  {
		f, err := c.ReadFrame()
		if err != nil {
			return err
		}

		// successfully read frame, dispatch it
		switch f.ID {
		case ResponseSoftwareVersion:
			if cb.SoftwareVersion != nil {
				version, err := f.softwareVersion()
				if err != nil {
					return errors.Wrapf(err, "error when converting to SoftwareVersion structure")
				}
				cb.SoftwareVersion(version)
			}
		case ResponseNavData:
			if cb.NavData != nil {
				navData, err := f.navData()
				if err != nil {
					return errors.Wrapf(err,"error when converting to NavData structure")
				}
				cb.NavData(navData)
			}
		}
	}
}
