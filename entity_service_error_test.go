package hal_test

import (
	"context"
	"testing"

	"github.com/dansimau/hal"
	"gotest.tools/v3/assert"
)

// TestEntityServiceCallErrors exercises the service-call error branches: the
// entities are registered (so the nil-connection guard passes) but the
// connection's client is never connected, so CallService returns an error.
func TestEntityServiceCallErrors(t *testing.T) {
	t.Parallel()

	conn := hal.NewConnection(hal.Config{DatabasePath: ":memory:"})

	light := hal.NewLight("light.err")
	sw := hal.NewInputBoolean("input_boolean.err")
	conn.RegisterEntities(light, sw)

	ctx := context.Background()

	assert.Assert(t, light.TurnOn() != nil)
	assert.Assert(t, light.TurnOnContext(ctx) != nil)
	assert.Assert(t, light.TurnOff() != nil)
	assert.Assert(t, light.TurnOffContext(ctx) != nil)

	assert.Assert(t, sw.TurnOn() != nil)
	assert.Assert(t, sw.TurnOff() != nil)
}

// TestLightGroupServiceCallErrors exercises the LightGroup error aggregation
// paths against registered-but-disconnected lights.
func TestLightGroupServiceCallErrors(t *testing.T) {
	t.Parallel()

	conn := hal.NewConnection(hal.Config{DatabasePath: ":memory:"})

	light1 := hal.NewLight("light.g1")
	light2 := hal.NewLight("light.g2")
	conn.RegisterEntities(light1, light2)

	group := hal.LightGroup{light1, light2}

	ctx := context.Background()

	// Two failing lights exercise the errors.Join branch.
	assert.Assert(t, group.TurnOn() != nil)
	assert.Assert(t, group.TurnOnContext(ctx) != nil)
	assert.Assert(t, group.TurnOff() != nil)
	assert.Assert(t, group.TurnOffContext(ctx) != nil)
}
