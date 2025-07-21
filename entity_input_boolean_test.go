package hal

import (
	"testing"

	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

func TestNewInputBoolean(t *testing.T) {
	inputBoolean := NewInputBoolean("input_boolean.test")
	assert.Equal(t, inputBoolean.GetID(), "input_boolean.test")
}

func TestInputBoolean_IsOff(t *testing.T) {
	t.Run("returns true when state is off", func(t *testing.T) {
		inputBoolean := NewInputBoolean("input_boolean.test")
		state := homeassistant.State{State: "off"}
		inputBoolean.SetState(state)

		assert.Equal(t, inputBoolean.IsOff(), true)
	})

	t.Run("returns false when state is on", func(t *testing.T) {
		inputBoolean := NewInputBoolean("input_boolean.test")
		state := homeassistant.State{State: "on"}
		inputBoolean.SetState(state)

		assert.Equal(t, inputBoolean.IsOff(), false)
	})

	t.Run("returns false when state is other value", func(t *testing.T) {
		inputBoolean := NewInputBoolean("input_boolean.test")
		state := homeassistant.State{State: "unavailable"}
		inputBoolean.SetState(state)

		assert.Equal(t, inputBoolean.IsOff(), false)
	})
}

func TestInputBoolean_IsOn(t *testing.T) {
	t.Run("returns true when state is on", func(t *testing.T) {
		inputBoolean := NewInputBoolean("input_boolean.test")
		state := homeassistant.State{State: "on"}
		inputBoolean.SetState(state)

		assert.Equal(t, inputBoolean.IsOn(), true)
	})

	t.Run("returns false when state is off", func(t *testing.T) {
		inputBoolean := NewInputBoolean("input_boolean.test")
		state := homeassistant.State{State: "off"}
		inputBoolean.SetState(state)

		assert.Equal(t, inputBoolean.IsOn(), false)
	})

	t.Run("returns false when state is other value", func(t *testing.T) {
		inputBoolean := NewInputBoolean("input_boolean.test")
		state := homeassistant.State{State: "unknown"}
		inputBoolean.SetState(state)

		assert.Equal(t, inputBoolean.IsOn(), false)
	})
}

func TestInputBoolean_TurnOn(t *testing.T) {
	t.Run("returns error when not registered", func(t *testing.T) {
		inputBoolean := NewInputBoolean("input_boolean.test")
		err := inputBoolean.TurnOn()
		assert.Equal(t, err, ErrEntityNotRegistered)
	})

	t.Run("accepts attributes", func(t *testing.T) {
		inputBoolean := NewInputBoolean("input_boolean.test")
		err := inputBoolean.TurnOn(map[string]any{"custom": "value"})
		assert.Equal(t, err, ErrEntityNotRegistered)
	})
}

func TestInputBoolean_TurnOff(t *testing.T) {
	t.Run("returns error when not registered", func(t *testing.T) {
		inputBoolean := NewInputBoolean("input_boolean.test")
		err := inputBoolean.TurnOff()
		assert.Equal(t, err, ErrEntityNotRegistered)
	})
}