package hal_test

import (
	"testing"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

func TestNewBinarySensor(t *testing.T) {
	t.Parallel()

	sensor := hal.NewBinarySensor("binary_sensor.test")
	assert.Equal(t, sensor.GetID(), "binary_sensor.test")
}

func TestBinarySensor_IsOff(t *testing.T) {
	t.Parallel()

	t.Run("returns true when state is off", func(t *testing.T) {
		t.Parallel()
		sensor := hal.NewBinarySensor("binary_sensor.test")
		state := homeassistant.State{State: "off"}
		sensor.SetState(state)

		assert.Equal(t, sensor.IsOff(), true)
	})

	t.Run("returns false when state is on", func(t *testing.T) {
		t.Parallel()
		sensor := hal.NewBinarySensor("binary_sensor.test")
		state := homeassistant.State{State: "on"}
		sensor.SetState(state)

		assert.Equal(t, sensor.IsOff(), false)
	})

	t.Run("returns false when state is other value", func(t *testing.T) {
		t.Parallel()
		sensor := hal.NewBinarySensor("binary_sensor.test")
		state := homeassistant.State{State: "unavailable"}
		sensor.SetState(state)

		assert.Equal(t, sensor.IsOff(), false)
	})
}

func TestBinarySensor_IsOn(t *testing.T) {
	t.Parallel()

	t.Run("returns true when state is on", func(t *testing.T) {
		t.Parallel()
		sensor := hal.NewBinarySensor("binary_sensor.test")
		state := homeassistant.State{State: "on"}
		sensor.SetState(state)

		assert.Equal(t, sensor.IsOn(), true)
	})

	t.Run("returns false when state is off", func(t *testing.T) {
		t.Parallel()
		sensor := hal.NewBinarySensor("binary_sensor.test")
		state := homeassistant.State{State: "off"}
		sensor.SetState(state)

		assert.Equal(t, sensor.IsOn(), false)
	})

	t.Run("returns false when state is other value", func(t *testing.T) {
		t.Parallel()
		sensor := hal.NewBinarySensor("binary_sensor.test")
		state := homeassistant.State{State: "unknown"}
		sensor.SetState(state)

		assert.Equal(t, sensor.IsOn(), false)
	})
}
