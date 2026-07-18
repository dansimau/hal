package hal_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/testutil"
	"github.com/davecgh/go-spew/spew"
	"gotest.tools/v3/assert"
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
			WithAction(func(_ context.Context, _ hal.EntityInterface) {
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

// TestEventsProcessedInOrder verifies that state change events are delivered to
// the handler in the order Home Assistant sent them. A regression here (e.g.
// dispatching each frame in its own goroutine) would let a rapid on/off sequence
// be reordered, leaving local state out of sync with reality.
func TestEventsProcessedInOrder(t *testing.T) {
	t.Parallel()

	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	testEntity := hal.NewEntity("test.entity")
	conn.RegisterEntities(testEntity)

	var (
		mu       sync.Mutex
		observed []string
	)

	conn.RegisterAutomations(
		hal.NewAutomation().
			WithName("test.order").
			WithEntities(testEntity).
			WithAction(func(_ context.Context, e hal.EntityInterface) {
				mu.Lock()
				observed = append(observed, e.GetState().State)
				mu.Unlock()
			}),
	)

	const n = 50

	expected := make([]string, 0, n)

	for i := range n {
		state := "on"
		if i%2 == 1 {
			state = "off"
		}

		expected = append(expected, state)

		// Zero timestamps so the staleness guard never applies, isolating the
		// ordering behaviour of the transport layer.
		server.SendEvent(homeassistant.Event{
			EventType: "state_changed",
			EventData: homeassistant.EventData{
				EntityID: "test.entity",
				NewState: &homeassistant.State{State: state},
			},
		})
	}

	testutil.WaitFor(t, "verify all events processed", func() bool {
		mu.Lock()
		defer mu.Unlock()

		return len(observed) == n
	}, func() {
		mu.Lock()
		defer mu.Unlock()

		spew.Dump(observed)
	})

	mu.Lock()
	defer mu.Unlock()

	assert.DeepEqual(t, expected, observed)
	assert.Equal(t, expected[n-1], testEntity.GetState().State)
}

// TestReaderNotBlockedByBlockedHandler verifies that a slow/blocked subscription
// handler cannot stall the websocket read loop. A service call's result is
// delivered by the same read loop, so if the reader could block handing a backlog
// of events to a stuck handler, the call could never receive its result. Here one
// event blocks the handler while a backlog larger than the channel buffer piles
// up behind it; a concurrent service call must still complete.
func TestReaderNotBlockedByBlockedHandler(t *testing.T) {
	t.Parallel()

	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	testEntity := hal.NewEntity("test.entity")
	light := hal.NewLight("light.test")
	conn.RegisterEntities(testEntity, light)

	gate := make(chan struct{})

	var blocked atomic.Bool

	// Released before cleanup (defers run LIFO) so the handler goroutine unblocks.
	defer close(gate)

	conn.RegisterAutomations(
		hal.NewAutomation().
			WithName("test.block").
			WithEntities(testEntity).
			WithAction(func(_ context.Context, _ hal.EntityInterface) {
				// Block the handler on the very first event; later events must
				// still be read off the socket while it waits here.
				if blocked.CompareAndSwap(false, true) {
					<-gate
				}
			}),
	)

	// Far more events than eventChannelBufferSize (256), so the backlog would
	// overflow a buffered channel and stall the reader without a drainer.
	for range 300 {
		server.SendEvent(homeassistant.Event{
			EventType: "state_changed",
			EventData: homeassistant.EventData{
				EntityID: "test.entity",
				NewState: &homeassistant.State{State: "on"},
			},
		})
	}

	testutil.WaitFor(t, "verify handler is blocked", func() bool {
		return blocked.Load()
	}, func() {})

	// Give the reader time to take the whole backlog off the socket.
	time.Sleep(200 * time.Millisecond)

	// The service call's result flows back through the same read loop, which must
	// not be stuck behind the backlog feeding the blocked handler.
	done := make(chan error, 1)
	go func() {
		done <- light.TurnOn()
	}()

	select {
	case err := <-done:
		assert.NilError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("service call blocked behind event backlog: read loop stalled")
	}
}

// TestStaleStateIgnored verifies that a state update carrying an older
// LastUpdated than the currently held state is dropped, so an out-of-order event
// can never overwrite newer state.
func TestStaleStateIgnored(t *testing.T) {
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
			WithAction(func(_ context.Context, _ hal.EntityInterface) {
				automationTriggered.Add(1)
			}),
	)

	t1 := time.Date(2026, 7, 17, 23, 0, 15, 0, time.UTC)
	t2 := time.Date(2026, 7, 17, 23, 0, 23, 0, time.UTC)
	t3 := time.Date(2026, 7, 17, 23, 0, 30, 0, time.UTC)

	// Newer "off" state (the truth).
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{State: "off", LastUpdated: t2},
		},
	})

	// Older "on" state arriving out of order - must be dropped.
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{State: "on", LastUpdated: t1},
		},
	})

	// Newer "off" barrier: events are delivered in order, so once this fires the
	// stale "on" above has definitely been processed (and dropped).
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{State: "off", LastUpdated: t3},
		},
	})

	testutil.WaitFor(t, "verify barrier event processed", func() bool {
		return automationTriggered.Load() == 2
	}, func() {
		spew.Dump(automationTriggered.Load(), testEntity.GetState())
	})

	// The stale "on" neither changed the state nor fired an automation.
	assert.Equal(t, "off", testEntity.GetState().State)
	assert.Equal(t, int32(2), automationTriggered.Load())
}

// TestNilStateLeavesStateUnchanged verifies that an event carrying a nil
// NewState is ignored rather than wiping the stored state.
func TestNilStateLeavesStateUnchanged(t *testing.T) {
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
			WithAction(func(_ context.Context, _ hal.EntityInterface) {
				automationTriggered.Add(1)
			}),
	)

	t1 := time.Date(2026, 7, 17, 23, 0, 15, 0, time.UTC)
	t2 := time.Date(2026, 7, 17, 23, 0, 30, 0, time.UTC)

	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{State: "on", LastUpdated: t1},
		},
	})

	// nil NewState must be ignored, not overwrite the stored state.
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: nil,
		},
	})

	// Newer barrier so we can deterministically wait for the nil event to have
	// been processed.
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{State: "on", LastUpdated: t2},
		},
	})

	testutil.WaitFor(t, "verify barrier event processed", func() bool {
		return automationTriggered.Load() == 2
	}, func() {
		spew.Dump(automationTriggered.Load(), testEntity.GetState())
	})

	// State survived the nil event, and the nil event fired no automation.
	assert.Equal(t, "on", testEntity.GetState().State)
	assert.Equal(t, int32(2), automationTriggered.Load())
}
