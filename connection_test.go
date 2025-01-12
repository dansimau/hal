package hal_test

import (
	"testing"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/testutil"
	"github.com/davecgh/go-spew/spew"
)

func TestConnection(t *testing.T) {
	t.Parallel()

	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	// Create test entity and register it
	entity := hal.NewEntity("test.entity")
	conn.RegisterEntities(entity)

	// Send state change event
	server.SendStateChangeEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{State: "on"},
		},
	})

	testutil.WaitFor(t, "verify entity state was updated", func() bool {
		return entity.GetState().State == "on"
	}, func() {
		spew.Dump(entity.GetID(), entity.GetState())
	})
}
