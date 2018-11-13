package skytraq

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestStart(t *testing.T) {
	c, m := connection()
	m.ReadBuf.Write(frameData(ResponseSoftwareVersion, versionData, 0))
	m.ReadBuf.Write(frameData(ResponseNavData, navData, 0))

	cbResult := struct {
		SoftwareVersion bool
		NavData         bool
	}{}
	err := c.Start(MessageCallbacks{
		SoftwareVersion: func(version SoftwareVersion) {
			cbResult.SoftwareVersion = true
			assert.Equal(t, 2007, version.Revision.Major)
		},
		NavData: func(data NavData) {
			cbResult.NavData = true
			assert.Equal(t, 3, data.SatelliteCount)
		},
	})
	assert.Error(t, err)
	assert.Equal(t, io.EOF, errors.Cause(err))
	assert.True(t, cbResult.SoftwareVersion)
	assert.True(t, cbResult.NavData)
}

