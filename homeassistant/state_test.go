package homeassistant_test

import (
	"testing"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

func TestState_Update(t *testing.T) {
	t.Parallel()

	t.Run("updates state when new state is non-empty", func(t *testing.T) {
		t.Parallel()

		s := homeassistant.State{State: "off"}
		s.Update(homeassistant.State{State: "on"})

		assert.Equal(t, s.State, "on")
	})

	t.Run("leaves state unchanged when new state is empty", func(t *testing.T) {
		t.Parallel()

		s := homeassistant.State{State: "on"}
		s.Update(homeassistant.State{State: ""})

		assert.Equal(t, s.State, "on")
	})

	t.Run("merges attributes into existing map", func(t *testing.T) {
		t.Parallel()

		s := homeassistant.State{
			Attributes: map[string]any{"brightness": float64(100)},
		}
		s.Update(homeassistant.State{
			Attributes: map[string]any{"color": "red"},
		})

		assert.Equal(t, s.Attributes["brightness"], float64(100))
		assert.Equal(t, s.Attributes["color"], "red")
	})

	t.Run("overwrites existing attribute values", func(t *testing.T) {
		t.Parallel()

		s := homeassistant.State{
			Attributes: map[string]any{"brightness": float64(100)},
		}
		s.Update(homeassistant.State{
			Attributes: map[string]any{"brightness": float64(200)},
		})

		assert.Equal(t, s.Attributes["brightness"], float64(200))
	})

	t.Run("initialises attributes map when nil", func(t *testing.T) {
		t.Parallel()

		s := homeassistant.State{}
		s.Update(homeassistant.State{
			Attributes: map[string]any{"brightness": float64(50)},
		})

		assert.Equal(t, s.Attributes["brightness"], float64(50))
	})

	t.Run("updates timestamps when set", func(t *testing.T) {
		t.Parallel()

		changed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		reported := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)
		updated := time.Date(2026, 1, 2, 3, 4, 7, 0, time.UTC)

		s := homeassistant.State{}
		s.Update(homeassistant.State{
			LastChanged:  changed,
			LastReported: reported,
			LastUpdated:  updated,
		})

		assert.Equal(t, s.LastChanged, changed)
		assert.Equal(t, s.LastReported, reported)
		assert.Equal(t, s.LastUpdated, updated)
	})

	t.Run("leaves timestamps unchanged when zero", func(t *testing.T) {
		t.Parallel()

		changed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

		s := homeassistant.State{LastChanged: changed}
		s.Update(homeassistant.State{})

		assert.Equal(t, s.LastChanged, changed)
	})
}
