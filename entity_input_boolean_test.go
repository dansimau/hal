package hal_test

import (
	"testing"

	"github.com/dansimau/hal"
	"gotest.tools/v3/assert"
)

func TestNewInputBoolean(t *testing.T) {
	t.Parallel()

	inputBoolean := hal.NewInputBoolean("input_boolean.test")
	assert.Equal(t, inputBoolean.GetID(), "input_boolean.test")
}

func TestInputBoolean_IsOff(t *testing.T) {
	t.Parallel()

	runStateCheckCases(t, func() *hal.InputBoolean {
		return hal.NewInputBoolean("input_boolean.test")
	}, (*hal.InputBoolean).IsOff, isOffCases(stateUnavailable))
}

func TestInputBoolean_IsOn(t *testing.T) {
	t.Parallel()

	runStateCheckCases(t, func() *hal.InputBoolean {
		return hal.NewInputBoolean("input_boolean.test")
	}, (*hal.InputBoolean).IsOn, isOnCases("unknown"))
}

func TestInputBoolean_TurnOn(t *testing.T) {
	t.Parallel()
	t.Run("returns error when not registered", func(t *testing.T) {
		t.Parallel()

		inputBoolean := hal.NewInputBoolean("input_boolean.test")
		err := inputBoolean.TurnOn()
		assert.Equal(t, err, hal.ErrEntityNotRegistered)
	})

	t.Run("accepts attributes", func(t *testing.T) {
		t.Parallel()

		inputBoolean := hal.NewInputBoolean("input_boolean.test")
		err := inputBoolean.TurnOn(map[string]any{"custom": "value"})
		assert.Equal(t, err, hal.ErrEntityNotRegistered)
	})
}

func TestInputBoolean_TurnOff(t *testing.T) {
	t.Parallel()
	t.Run("returns error when not registered", func(t *testing.T) {
		t.Parallel()

		inputBoolean := hal.NewInputBoolean("input_boolean.test")
		err := inputBoolean.TurnOff()
		assert.Equal(t, err, hal.ErrEntityNotRegistered)
	})
}
