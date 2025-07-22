package hal

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestNewSunTimes(t *testing.T) {
	t.Parallel()

	config := LocationConfig{
		Latitude:  37.7749,
		Longitude: -122.4194,
	}

	sunTimes := NewSunTimes(config)
	assert.Assert(t, sunTimes != nil)
	assert.Equal(t, sunTimes.location.Latitude, 37.7749)
	assert.Equal(t, sunTimes.location.Longitude, -122.4194)
}
