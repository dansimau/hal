package hal_test

import (
	"testing"

	"github.com/dansimau/hal"
	"gotest.tools/v3/assert"
)

func TestNewBinarySensor(t *testing.T) {
	t.Parallel()

	sensor := hal.NewBinarySensor("binary_sensor.test")
	assert.Equal(t, sensor.GetID(), "binary_sensor.test")
}

func TestBinarySensor_IsOff(t *testing.T) {
	t.Parallel()

	runStateCheckCases(t, func() *hal.BinarySensor {
		return hal.NewBinarySensor("binary_sensor.test")
	}, (*hal.BinarySensor).IsOff, isOffCases(stateUnavailable))
}

func TestBinarySensor_IsOn(t *testing.T) {
	t.Parallel()

	runStateCheckCases(t, func() *hal.BinarySensor {
		return hal.NewBinarySensor("binary_sensor.test")
	}, (*hal.BinarySensor).IsOn, isOnCases("unknown"))
}
