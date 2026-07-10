package hal

import (
	"testing"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

// TestHeartbeatKeepsConnectionAlive verifies that heartbeat pings keep the
// connection alive during a quiet period with no state change events, i.e. the
// read timeout does not trip as long as pong responses keep arriving.
func TestHeartbeatKeepsConnectionAlive(t *testing.T) {
	conn, server, cleanup := newClientServerWithConfig(t, Config{
		DatabasePath:      ":memory:",
		ReconnectInterval: 100 * time.Millisecond,
		PingInterval:      50 * time.Millisecond,
		ReadTimeout:       300 * time.Millisecond,
	})
	defer cleanup()

	testEntity := NewEntity("test.entity")
	conn.RegisterEntities(testEntity)

	// Wait well beyond the read timeout with no events. Because the server
	// responds to heartbeat pings, the connection should stay alive and not
	// reconnect.
	time.Sleep(600 * time.Millisecond)

	assert.Equal(t, 0, conn.GetReconnectAttempts(), "connection should not reconnect while pings are answered")

	// Connection should still be functional.
	server.SendEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{
				EntityID: "test.entity",
				State:    "on",
			},
		},
	})

	waitFor(t, "event received after quiet period", func() bool {
		return testEntity.GetState().State == "on"
	}, func() {
		t.Logf("Entity state: %v", testEntity.GetState())
	})
}

// TestStaleConnectionTriggersReconnect verifies that a "stuck" connection (one
// that remains open but stops delivering data) is detected via the read timeout
// and triggers a reconnection.
func TestStaleConnectionTriggersReconnect(t *testing.T) {
	conn, server, cleanup := newClientServerWithConfig(t, Config{
		DatabasePath:      ":memory:",
		ReconnectInterval: 100 * time.Millisecond,
		PingInterval:      50 * time.Millisecond,
		ReadTimeout:       200 * time.Millisecond,
	})
	defer cleanup()

	testEntity := NewEntity("test.entity")
	conn.RegisterEntities(testEntity)

	// Simulate a stuck connection: the socket stays open but the server stops
	// replying to pings, so no data reaches the client.
	server.SetRespondToPings(false)

	// The client should detect the staleness via the read timeout and reconnect.
	waitForReconnection(t, conn, 1)

	// Restore normal behaviour so the reconnected connection stays healthy.
	server.SetRespondToPings(true)

	// Wait for the subscription to be re-established, then verify events flow.
	waitForEventSubscription(t, server, 1)

	server.SendEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: "test.entity",
			NewState: &homeassistant.State{
				EntityID: "test.entity",
				State:    "recovered",
			},
		},
	})

	waitFor(t, "event received after stale reconnect", func() bool {
		return testEntity.GetState().State == "recovered"
	}, func() {
		t.Logf("Entity state: %v", testEntity.GetState())
	})
}
