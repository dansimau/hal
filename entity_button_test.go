package hal_test

import (
	"testing"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

func TestNewButton(t *testing.T) {
	t.Parallel()

	button := hal.NewButton("event.button")
	assert.Equal(t, button.GetID(), "event.button")
}

func TestButton_Name(t *testing.T) {
	t.Parallel()

	button := hal.NewButton("event.button")
	assert.Equal(t, button.Name(), "event.button")
}

func TestButton_Entities(t *testing.T) {
	t.Parallel()

	button := hal.NewButton("event.button")
	entities := button.Entities()

	assert.Equal(t, len(entities), 1)
	assert.Equal(t, entities[0].GetID(), "event.button")
}

func TestButton_Action(t *testing.T) {
	t.Parallel()

	t.Run("ignores non-initial-press events", func(t *testing.T) {
		t.Parallel()

		button := hal.NewButton("event.button")
		button.SetState(homeassistant.State{
			Attributes: map[string]any{"event_type": "long_press"},
		})

		button.Action(button)

		assert.Equal(t, button.PressedTimes(), int32(0))
	})

	t.Run("counts an initial press", func(t *testing.T) {
		t.Parallel()

		button := hal.NewButton("event.button")
		button.SetState(homeassistant.State{
			Attributes: map[string]any{"event_type": "initial_press"},
		})

		button.Action(button)

		assert.Equal(t, button.PressedTimes(), int32(1))
	})

	t.Run("counts rapid repeat presses cumulatively", func(t *testing.T) {
		t.Parallel()

		button := hal.NewButton("event.button")
		button.SetState(homeassistant.State{
			Attributes: map[string]any{"event_type": "initial_press"},
		})

		button.Action(button)
		button.Action(button)
		button.Action(button)

		assert.Equal(t, button.PressedTimes(), int32(3))
	})
}
