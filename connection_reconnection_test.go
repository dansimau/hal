package hal

import (
	"sync"
	"testing"
	"time"

	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

// waitForReconnection waits for the connection to successfully reconnect.
func waitForReconnection(t *testing.T, conn *Connection, expectedAttempts int) {
	t.Helper()

	waitFor(t, "reconnection", func() bool {
		return conn.GetReconnectAttempts() >= expectedAttempts
	}, func() {
		t.Logf("Reconnect attempts: %d (expected >= %d)", conn.GetReconnectAttempts(), expectedAttempts)
	})
}

// waitForEventSubscription waits for event subscriptions to be re-established.
func waitForEventSubscription(t *testing.T, server *hassws.Server, expectedCount int) {
	t.Helper()

	waitFor(t, "event subscription", func() bool {
		return server.GetSubscriptionCount() >= expectedCount
	}, func() {
		t.Logf("Subscription count: %d (expected >= %d)", server.GetSubscriptionCount(), expectedCount)
	})
}

func TestBasicReconnection(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	// Verify initial connection works by sending an event
	testEntity := NewEntity("test.entity")
	conn.RegisterEntities(testEntity)

	server.SendEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{
				EntityID: "test.entity",
				State:    "on",
			},
		},
	})

	waitFor(t, "initial state update", func() bool {
		return testEntity.GetState().State == "on"
	}, func() {
		t.Logf("Entity state: %v", testEntity.GetState())
	})

	// Force disconnect
	assert.NilError(t, server.DisconnectClient())

	// Wait for reconnection (first attempt)
	waitForReconnection(t, conn, 1)

	// Wait for subscription to be re-established
	waitForEventSubscription(t, server, 1)

	// Verify connection is working by sending another event
	server.SendEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{
				EntityID: "test.entity",
				State:    "off",
			},
		},
	})

	waitFor(t, "state update after reconnection", func() bool {
		return testEntity.GetState().State == "off"
	}, func() {
		t.Logf("Entity state: %v", testEntity.GetState())
	})
}

func TestServiceCallsFailDuringDisconnection(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	testEntity := NewLight("test.light")
	conn.RegisterEntities(testEntity)

	// Verify service calls work initially
	err := testEntity.TurnOn()
	assert.NilError(t, err)

	// Disconnect
	assert.NilError(t, server.DisconnectClient())

	// Brief wait for disconnection to be detected
	time.Sleep(50 * time.Millisecond)

	// Attempt service call - should fail
	err = testEntity.TurnOn()
	assert.ErrorIs(t, err, hassws.ErrNotConnected)

	// Wait for reconnection
	waitForReconnection(t, conn, 1)

	// Wait for subscription to be re-established
	waitForEventSubscription(t, server, 1)

	// Service calls should work again
	err = testEntity.TurnOn()
	assert.NilError(t, err)
}

func TestSubscriptionRestoredAfterReconnect(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	testEntity := NewEntity("test.entity")
	conn.RegisterEntities(testEntity)

	// Send event, verify received
	server.SendEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{
				EntityID: "test.entity",
				State:    "on",
			},
		},
	})

	waitFor(t, "initial event received", func() bool {
		return testEntity.GetState().State == "on"
	}, func() {})

	// Disconnect
	assert.NilError(t, server.DisconnectClient())

	// Wait for reconnection
	waitForReconnection(t, conn, 1)

	// Verify subscription re-established (should have at least 1 subscription)
	waitForEventSubscription(t, server, 1)

	// Send new event and verify it's received (proves subscription works)
	server.SendEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{
				EntityID: "test.entity",
				State:    "off",
			},
		},
	})

	waitFor(t, "event after reconnection", func() bool {
		return testEntity.GetState().State == "off"
	}, func() {})
}

func TestStateSyncAfterReconnect(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	testEntity := NewEntity("test.entity")
	conn.RegisterEntities(testEntity)

	// Change state while connected
	server.SendEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{
				EntityID: "test.entity",
				State:    "on",
			},
		},
	})

	waitFor(t, "state synced", func() bool {
		return testEntity.GetState().State == "on"
	}, func() {})

	// Disconnect
	assert.NilError(t, server.DisconnectClient())

	// Wait for reconnection
	waitForReconnection(t, conn, 1)

	// Wait for subscription to be re-established
	waitForEventSubscription(t, server, 1)

	// The reconnection process calls syncStates(), but since the mock server
	// returns empty state list, we can't directly test state re-sync.
	// This test verifies that reconnection completes successfully.

	// Verify connection works by sending new event
	server.SendEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{
				EntityID: "test.entity",
				State:    "off",
			},
		},
	})

	waitFor(t, "state after reconnect", func() bool {
		return testEntity.GetState().State == "off"
	}, func() {})
}

func TestMultipleDisconnectReconnectCycles(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	testEntity := NewEntity("test.entity")
	conn.RegisterEntities(testEntity)

	for i := 1; i <= 3; i++ {
		// Disconnect
		assert.NilError(t, server.DisconnectClient())

		// Wait for reconnection
		waitForReconnection(t, conn, i)

		// Wait for subscription to be re-established
		waitForEventSubscription(t, server, 1)

		// Verify functionality by sending event
		server.SendEvent(homeassistant.Event{
			EventData: homeassistant.EventData{
				EntityID: "test.entity",
				NewState: &homeassistant.State{
					EntityID: "test.entity",
					State:    "cycle" + string(rune('0'+i)),
				},
			},
		})

		waitFor(t, "event in cycle", func() bool {
			state := testEntity.GetState().State
			return state == "cycle"+string(rune('0'+i))
		}, func() {
			t.Logf("Cycle %d - Entity state: %v", i, testEntity.GetState().State)
		})
	}

	// Verify we had exactly 3 reconnection attempts
	assert.Equal(t, 3, conn.GetReconnectAttempts())
}

func TestConcurrentServiceCallsDuringReconnection(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	testEntity := NewLight("test.light")
	conn.RegisterEntities(testEntity)

	// Disconnect
	assert.NilError(t, server.DisconnectClient())

	// Brief wait for disconnection
	time.Sleep(50 * time.Millisecond)

	// Launch multiple goroutines calling services
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := testEntity.TurnOn()
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Verify all calls returned ErrNotConnected
	errorCount := 0
	for err := range errChan {
		assert.ErrorIs(t, err, hassws.ErrNotConnected)
		errorCount++
	}

	// All 10 calls should have failed
	assert.Assert(t, errorCount == 10, "Expected 10 errors, got %d", errorCount)
}

func TestShutdownDuringReconnection(t *testing.T) {
	conn, server, cleanup := newClientServerWithConfig(t, Config{
		DatabasePath:      ":memory:",
		ReconnectInterval: 1 * time.Second, // Slow retry for this test
	})
	defer cleanup()

	// Disconnect
	assert.NilError(t, server.DisconnectClient())

	// Brief wait for disconnection detection
	time.Sleep(100 * time.Millisecond)

	// Record reconnect attempts before shutdown
	attemptsBefore := conn.GetReconnectAttempts()

	// Call Close() while reconnection is pending
	done := make(chan struct{})
	go func() {
		conn.Close()
		close(done)
	}()

	// Verify shutdown completes within reasonable time
	select {
	case <-done:
		// Success - shutdown completed
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown took too long")
	}

	// Verify no additional reconnection attempts after shutdown
	time.Sleep(200 * time.Millisecond)
	attemptsAfter := conn.GetReconnectAttempts()
	assert.Equal(t, attemptsBefore, attemptsAfter, "Reconnection attempts should not increase after shutdown")
}

func TestReconnectionAttemptCounter(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	// Initial state - no reconnection attempts
	assert.Equal(t, 0, conn.GetReconnectAttempts())

	// First disconnect/reconnect
	assert.NilError(t, server.DisconnectClient())
	waitForReconnection(t, conn, 1)
	waitForEventSubscription(t, server, 1)
	assert.Equal(t, 1, conn.GetReconnectAttempts())

	// Second disconnect/reconnect
	assert.NilError(t, server.DisconnectClient())
	waitForReconnection(t, conn, 2)
	waitForEventSubscription(t, server, 1)
	assert.Equal(t, 2, conn.GetReconnectAttempts())

	// Reset counter
	conn.ResetReconnectAttempts()
	assert.Equal(t, 0, conn.GetReconnectAttempts())

	// Third disconnect/reconnect (counter starts from 0 again)
	assert.NilError(t, server.DisconnectClient())
	waitForReconnection(t, conn, 1)
	waitForEventSubscription(t, server, 1)
	assert.Equal(t, 1, conn.GetReconnectAttempts())
}
