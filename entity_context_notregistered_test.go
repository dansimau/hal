package hal_test

import (
	"context"
	"testing"

	"github.com/dansimau/hal"
	"gotest.tools/v3/assert"
)

// When an entity is not bound to a connection, any method that would call a
// Home Assistant service must return ErrEntityNotRegistered rather than
// dereferencing a nil connection. The non-context variants are covered
// elsewhere; these exercise the context-aware variants.

func TestLight_TurnOnContext_NotRegistered(t *testing.T) {
	t.Parallel()

	light := hal.NewLight("light.test")
	err := light.TurnOnContext(context.Background())
	assert.Equal(t, err, hal.ErrEntityNotRegistered)

	// With attributes too.
	err = light.TurnOnContext(context.Background(), map[string]any{"brightness": 128})
	assert.Equal(t, err, hal.ErrEntityNotRegistered)
}

func TestLight_TurnOffContext_NotRegistered(t *testing.T) {
	t.Parallel()

	light := hal.NewLight("light.test")
	err := light.TurnOffContext(context.Background())
	assert.Equal(t, err, hal.ErrEntityNotRegistered)
}

func TestLightGroup_TurnOnContext_NotRegistered(t *testing.T) {
	t.Parallel()

	lg := hal.LightGroup{hal.NewLight("light.1"), hal.NewLight("light.2")}
	err := lg.TurnOnContext(context.Background())
	// Two unregistered lights produce a joined error mentioning the cause.
	assert.ErrorContains(t, err, "entity not registered")
}

func TestLightGroup_TurnOffContext_NotRegistered(t *testing.T) {
	t.Parallel()

	lg := hal.LightGroup{hal.NewLight("light.1"), hal.NewLight("light.2")}
	err := lg.TurnOffContext(context.Background())
	assert.ErrorContains(t, err, "entity not registered")
}

// A single-member group returns the underlying error unwrapped, so
// errors.Is still matches ErrEntityNotRegistered.
func TestLightGroup_TurnOnContext_SingleMemberNotRegistered(t *testing.T) {
	t.Parallel()

	lg := hal.LightGroup{hal.NewLight("light.only")}
	err := lg.TurnOnContext(context.Background())
	assert.Equal(t, err, hal.ErrEntityNotRegistered)
}
