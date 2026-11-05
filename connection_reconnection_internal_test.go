package hal

import (
	"strconv"
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

// waitForEventSubscription waits for at least one event subscription to be
// (re-)established on the server.
func waitForEventSubscription(t *testing.T, server *hassws.Server) {
	t.Helper()

	waitFor(t, "event subscription", func() bool {
		return server.GetSubscriptionCount() >= 1
	}, func() {
		t.Logf("Subscription count: %d (expected >= 1)", server.GetSubscriptionCount())
	})
}

// sendStateAndWait sends a state change event for the test entity and waits
// until the entity reflects the new state.
func sendStateAndWait(t *testing.T, server *hassws.Server, entity *Entity, state, waitName string) {
	t.Helper()

	server.SendEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: testEntityID,
			NewState: &homeassistant.State{
				EntityID: testEntityID,
				State:    state,
			},
		},
	})

	waitFor(t, waitName, func() bool {
		return entity.GetState().State == state
	}, func() {
		t.Logf("Entity state: %v", entity.GetState())
	})
}

//nolint:paralleltest // NewConnection sets the global logger database
func TestBasicReconnection(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	// Verify initial connection works by sending an event
	testEntity := NewEntity(testEntityID)
	conn.RegisterEntities(testEntity)

	sendStateAndWait(t, server, testEntity, stateOn, "initial state update")

	// Force disconnect
	assert.NilError(t, server.DisconnectClient())

	// Wait for reconnection (first attempt)
	waitForReconnection(t, conn, 1)

	// Wait for subscription to be re-established
	waitForEventSubscription(t, server)

	// Verify connection is working by sending another event
	sendStateAndWait(t, server, testEntity, stateOff, "state update after reconnection")
}

//nolint:paralleltest // NewConnection sets the global logger database
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
	waitForEventSubscription(t, server)

	// Service calls should work again
	err = testEntity.TurnOn()
	assert.NilError(t, err)
}

//nolint:paralleltest // NewConnection sets the global logger database
func TestSubscriptionRestoredAfterReconnect(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	testEntity := NewEntity(testEntityID)
	conn.RegisterEntities(testEntity)

	// Send event, verify received
	sendStateAndWait(t, server, testEntity, stateOn, "initial event received")

	// Disconnect
	assert.NilError(t, server.DisconnectClient())

	// Wait for reconnection
	waitForReconnection(t, conn, 1)

	// Verify subscription re-established (should have at least 1 subscription)
	waitForEventSubscription(t, server)

	// Send new event and verify it's received (proves subscription works)
	sendStateAndWait(t, server, testEntity, stateOff, "event after reconnection")
}

//nolint:paralleltest // NewConnection sets the global logger database
func TestStateSyncAfterReconnect(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	testEntity := NewEntity(testEntityID)
	conn.RegisterEntities(testEntity)

	// Change state while connected
	sendStateAndWait(t, server, testEntity, stateOn, "state synced")

	// Disconnect
	assert.NilError(t, server.DisconnectClient())

	// Wait for reconnection
	waitForReconnection(t, conn, 1)

	// Wait for subscription to be re-established
	waitForEventSubscription(t, server)

	// The reconnection process calls syncStates(), but since the mock server
	// returns empty state list, we can't directly test state re-sync.
	// This test verifies that reconnection completes successfully.

	// Verify connection works by sending new event
	sendStateAndWait(t, server, testEntity, stateOff, "state after reconnect")
}

//nolint:paralleltest // NewConnection sets the global logger database
func TestMultipleDisconnectReconnectCycles(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	testEntity := NewEntity(testEntityID)
	conn.RegisterEntities(testEntity)

	const cycles = 3

	for cycle := 1; cycle <= cycles; cycle++ {
		// Disconnect
		assert.NilError(t, server.DisconnectClient())

		// Wait for reconnection
		waitForReconnection(t, conn, cycle)

		// Wait for subscription to be re-established
		waitForEventSubscription(t, server)

		// Verify functionality by sending event
		sendStateAndWait(t, server, testEntity, "cycle"+strconv.Itoa(cycle), "event in cycle")
	}

	// Verify we had exactly 3 reconnection attempts
	assert.Equal(t, cycles, conn.GetReconnectAttempts())
}

//nolint:paralleltest // NewConnection sets the global logger database
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

	for range 10 {
		wg.Go(func() {
			err := testEntity.TurnOn()
			if err != nil {
				errChan <- err
			}
		})
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

//nolint:paralleltest // NewConnection sets the global logger database
func TestShutdownDuringReconnection(t *testing.T) {
	conn, server, cleanup := newClientServerWithConfig(t, Config{
		DatabasePath:      memoryDatabasePath,
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

//nolint:paralleltest // NewConnection sets the global logger database
func TestReconnectionAttemptCounter(t *testing.T) {
	conn, server, cleanup := newFastReconnectClientServer(t)
	defer cleanup()

	// Initial state - no reconnection attempts
	assert.Equal(t, 0, conn.GetReconnectAttempts())

	// First disconnect/reconnect
	assert.NilError(t, server.DisconnectClient())
	waitForReconnection(t, conn, 1)
	waitForEventSubscription(t, server)
	assert.Equal(t, 1, conn.GetReconnectAttempts())

	// Second disconnect/reconnect
	assert.NilError(t, server.DisconnectClient())
	waitForReconnection(t, conn, 2)
	waitForEventSubscription(t, server)
	assert.Equal(t, 2, conn.GetReconnectAttempts())

	// Reset counter
	conn.ResetReconnectAttempts()
	assert.Equal(t, 0, conn.GetReconnectAttempts())

	// Third disconnect/reconnect (counter starts from 0 again)
	assert.NilError(t, server.DisconnectClient())
	waitForReconnection(t, conn, 1)
	waitForEventSubscription(t, server)
	assert.Equal(t, 1, conn.GetReconnectAttempts())
}
