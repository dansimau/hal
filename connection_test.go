package hal_test

import (
	"sync/atomic"
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
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
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

func TestLoopProtection(t *testing.T) {
	t.Parallel()

	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	var automationTriggered atomic.Int32

	testEntity := hal.NewEntity("test.entity")
	conn.RegisterEntities(testEntity)

	conn.RegisterAutomations(
		hal.NewAutomation().
			WithName("test.automation").
			WithEntities(testEntity).
			WithAction(func(_ hal.EntityInterface) {
				automationTriggered.Add(1)
			}),
	)

	// This one should be ignored because it is from the same user
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{State: "off"},
		},
		Context: homeassistant.EventMessageContext{
			UserID: testutil.TestUserID,
		},
	})

	// This one should cause the automation to be triggered
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{State: "on"},
		},
	})

	testutil.WaitFor(t, "verify automation was triggered", func() bool {
		return automationTriggered.Load() == 1
	}, func() {
		spew.Dump(automationTriggered.Load())
	})
}
