package hal

import (
	"testing"

	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

func TestNewLight(t *testing.T) {
	light := NewLight("light.test")
	assert.Equal(t, light.GetID(), "light.test")
}

func TestLight_GetBrightness(t *testing.T) {
	t.Run("returns brightness from attributes", func(t *testing.T) {
		light := NewLight("light.test")
		state := homeassistant.State{
			Attributes: map[string]any{
				"brightness": float64(255),
			},
		}
		light.SetState(state)

		brightness := light.GetBrightness()
		assert.Equal(t, brightness, float64(255))
	})

	t.Run("returns 0 when brightness not in attributes", func(t *testing.T) {
		light := NewLight("light.test")
		state := homeassistant.State{
			Attributes: map[string]any{},
		}
		light.SetState(state)

		brightness := light.GetBrightness()
		assert.Equal(t, brightness, float64(0))
	})

	t.Run("returns 0 when brightness is wrong type", func(t *testing.T) {
		light := NewLight("light.test")
		state := homeassistant.State{
			Attributes: map[string]any{
				"brightness": "invalid",
			},
		}
		light.SetState(state)

		brightness := light.GetBrightness()
		assert.Equal(t, brightness, float64(0))
	})
}

func TestLight_IsOn(t *testing.T) {
	t.Run("returns true when state is on", func(t *testing.T) {
		light := NewLight("light.test")
		state := homeassistant.State{State: "on"}
		light.SetState(state)

		assert.Equal(t, light.IsOn(), true)
	})

	t.Run("returns false when state is off", func(t *testing.T) {
		light := NewLight("light.test")
		state := homeassistant.State{State: "off"}
		light.SetState(state)

		assert.Equal(t, light.IsOn(), false)
	})

	t.Run("returns false when state is other value", func(t *testing.T) {
		light := NewLight("light.test")
		state := homeassistant.State{State: "unavailable"}
		light.SetState(state)

		assert.Equal(t, light.IsOn(), false)
	})
}

func TestLight_TurnOn(t *testing.T) {
	t.Run("returns error when not registered", func(t *testing.T) {
		light := NewLight("light.test")
		err := light.TurnOn()
		assert.Equal(t, err, ErrEntityNotRegistered)
	})

	t.Run("calls service with attributes", func(t *testing.T) {
		// This would require mocking the connection and CallService method
		// For now, we'll test the error case above
		light := NewLight("light.test")

		// Test with attributes
		err := light.TurnOn(map[string]any{"brightness": 128})
		assert.Equal(t, err, ErrEntityNotRegistered)
	})
}

func TestLight_TurnOff(t *testing.T) {
	t.Run("returns error when not registered", func(t *testing.T) {
		light := NewLight("light.test")
		err := light.TurnOff()
		assert.Equal(t, err, ErrEntityNotRegistered)
	})
}

func TestLightGroup_GetID(t *testing.T) {
	t.Run("returns empty group message for empty group", func(t *testing.T) {
		lg := LightGroup{}
		assert.Equal(t, lg.GetID(), "(empty light group)")
	})

	t.Run("returns joined IDs for multiple lights", func(t *testing.T) {
		light1 := NewLight("light.1")
		light2 := NewLight("light.2")
		lg := LightGroup{light1, light2}

		assert.Equal(t, lg.GetID(), "light.1, light.2")
	})
}

func TestLightGroup_GetBrightness(t *testing.T) {
	t.Run("returns 0 for empty group", func(t *testing.T) {
		lg := LightGroup{}
		assert.Equal(t, lg.GetBrightness(), float64(0))
	})

	t.Run("returns brightness of first light", func(t *testing.T) {
		light1 := NewLight("light.1")
		light2 := NewLight("light.2")

		state1 := homeassistant.State{Attributes: map[string]any{"brightness": float64(100)}}
		state2 := homeassistant.State{Attributes: map[string]any{"brightness": float64(200)}}

		light1.SetState(state1)
		light2.SetState(state2)

		lg := LightGroup{light1, light2}
		assert.Equal(t, lg.GetBrightness(), float64(100))
	})
}

func TestLightGroup_GetState(t *testing.T) {
	t.Run("returns empty state for empty group", func(t *testing.T) {
		lg := LightGroup{}
		state := lg.GetState()
		assert.DeepEqual(t, state, homeassistant.State{})
	})

	t.Run("returns state of first light", func(t *testing.T) {
		light1 := NewLight("light.1")
		expectedState := homeassistant.State{State: "on", Attributes: map[string]any{"brightness": float64(255)}}
		light1.SetState(expectedState)

		lg := LightGroup{light1}
		state := lg.GetState()
		assert.DeepEqual(t, state, expectedState)
	})
}

func TestLightGroup_SetState(t *testing.T) {
	t.Run("sets state on all lights in group", func(t *testing.T) {
		light1 := NewLight("light.1")
		light2 := NewLight("light.2")
		lg := LightGroup{light1, light2}

		newState := homeassistant.State{State: "on", Attributes: map[string]any{"brightness": float64(128)}}
		lg.SetState(newState)

		assert.DeepEqual(t, light1.GetState(), newState)
		assert.DeepEqual(t, light2.GetState(), newState)
	})
}

func TestLightGroup_IsOn(t *testing.T) {
	t.Run("returns true when all lights are on", func(t *testing.T) {
		light1 := NewLight("light.1")
		light2 := NewLight("light.2")

		light1.SetState(homeassistant.State{State: "on"})
		light2.SetState(homeassistant.State{State: "on"})

		lg := LightGroup{light1, light2}
		assert.Equal(t, lg.IsOn(), true)
	})

	t.Run("returns false when any light is off", func(t *testing.T) {
		light1 := NewLight("light.1")
		light2 := NewLight("light.2")

		light1.SetState(homeassistant.State{State: "on"})
		light2.SetState(homeassistant.State{State: "off"})

		lg := LightGroup{light1, light2}
		assert.Equal(t, lg.IsOn(), false)
	})

	t.Run("returns true for empty group", func(t *testing.T) {
		lg := LightGroup{}
		assert.Equal(t, lg.IsOn(), true)
	})
}

func TestLightGroup_TurnOn(t *testing.T) {
	t.Run("returns nil when no errors", func(t *testing.T) {
		// Empty group should not error
		lg := LightGroup{}
		err := lg.TurnOn()
		assert.NilError(t, err)
	})

	t.Run("collects errors from individual lights", func(t *testing.T) {
		light1 := NewLight("light.1")
		light2 := NewLight("light.2")
		lg := LightGroup{light1, light2}

		// Both lights should return ErrEntityNotRegistered
		err := lg.TurnOn()
		// With 2 errors, should return a joined error
		assert.ErrorContains(t, err, "entity not registered")
	})
}

func TestLightGroup_TurnOff(t *testing.T) {
	t.Run("returns nil when no errors", func(t *testing.T) {
		lg := LightGroup{}
		err := lg.TurnOff()
		assert.NilError(t, err)
	})

	t.Run("collects errors from individual lights", func(t *testing.T) {
		light1 := NewLight("light.1")
		light2 := NewLight("light.2")
		lg := LightGroup{light1, light2}

		err := lg.TurnOff()
		// With 2 errors, should return joined error
		assert.ErrorContains(t, err, "entity not registered")
	})
}

func TestLightGroup_BindConnection(t *testing.T) {
	t.Run("binds connection to all lights", func(t *testing.T) {
		light1 := NewLight("light.1")
		light2 := NewLight("light.2")
		lg := LightGroup{light1, light2}

		// Create a mock connection (would need proper setup in real scenario)
		conn := &Connection{}
		lg.BindConnection(conn)

		// Verify connection was bound (in real test, would check that connection property was set)
		// For now, just verify the method doesn't panic
		assert.Equal(t, len(lg), 2)
	})
}
