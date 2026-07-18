package hal_test

import (
	"testing"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

func TestNewLightSensor(t *testing.T) {
	t.Parallel()

	sensor := hal.NewLightSensor("sensor.light_level")
	assert.Equal(t, sensor.GetID(), "sensor.light_level")
}

func TestLightSensor_Level(t *testing.T) {
	t.Parallel()

	t.Run("returns parsed integer level", func(t *testing.T) {
		t.Parallel()

		sensor := hal.NewLightSensor("sensor.light_level")
		sensor.SetState(homeassistant.State{State: "42"})

		assert.Equal(t, sensor.Level(), 42)
	})

	t.Run("returns 0 when state is not a number", func(t *testing.T) {
		t.Parallel()

		sensor := hal.NewLightSensor("sensor.light_level")
		sensor.SetState(homeassistant.State{State: "unavailable"})

		assert.Equal(t, sensor.Level(), 0)
	})

	t.Run("returns 0 when state is empty", func(t *testing.T) {
		t.Parallel()

		sensor := hal.NewLightSensor("sensor.light_level")

		assert.Equal(t, sensor.Level(), 0)
	})
}
