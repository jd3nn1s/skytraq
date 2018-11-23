package skytraq

import (
	"context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Callbacks struct {
	SoftwareVersion func(SoftwareVersion)
	NavData         func(NavData)
}

func (c *Connection) Start(ctx context.Context, cb Callbacks) error {
	for {
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
					return errors.Wrapf(err, "error when converting to NavData structure")
				}
				cb.NavData(navData)
			}
		}

		select {
		case <-ctx.Done():
			logrus.Infof("gps: context: %v", ctx.Err())
			return nil
		default:
		}
	}
}
