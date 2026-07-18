package hassws

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

const testUserID = "d8e8fca2dc0f896fd7cb4cb0031ba249"

// newTestClient creates a mock server and a client configured to reach it. The
// client is NOT connected yet, so callers can set up callbacks (e.g.
// SetOnDisconnected) before connecting. The server is closed on test cleanup.
func newTestClient(t *testing.T, cfg ClientConfig) (*Client, *Server) {
	t.Helper()

	server, err := NewServer(map[string]string{"test-token": testUserID})
	assert.NilError(t, err)

	t.Cleanup(func() { _ = server.Close() })

	if cfg.Host == "" {
		cfg.Host = server.ListenAddress()
	}

	if cfg.Token == "" {
		cfg.Token = "test-token"
	}

	return NewClient(cfg), server
}

// connect connects the client and registers a graceful close on cleanup.
func connect(t *testing.T, c *Client) {
	t.Helper()

	assert.NilError(t, c.Connect())

	t.Cleanup(func() { _ = c.Close() })
}

// waitForCond polls fn until it returns true or the timeout elapses.
func waitForCond(t *testing.T, name string, fn func() bool) {
	t.Helper()

	deadline := time.After(3 * time.Second)

	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for: %s", name)
		default:
			if fn() {
				return
			}

			time.Sleep(20 * time.Millisecond)
		}
	}
}

// --- Pure unit tests (no connection) ---

func TestNextMsgID(t *testing.T) {
	t.Parallel()

	c := NewClient(ClientConfig{})

	assert.Equal(t, c.nextMsgID(), 1)
	assert.Equal(t, c.nextMsgID(), 2)
	assert.Equal(t, c.nextMsgID(), 3)
}

func TestGetStateDefaultAndSet(t *testing.T) {
	t.Parallel()

	// A zero-value client has never had its state set; getState must not panic
	// and should report disconnected.
	c := &Client{}
	assert.Equal(t, c.getState(), stateDisconnected)

	c.setState(stateConnected)
	assert.Equal(t, c.getState(), stateConnected)

	c.setState(stateConnecting)
	assert.Equal(t, c.getState(), stateConnecting)
}

func TestCallServiceNotConnected(t *testing.T) {
	t.Parallel()

	c := NewClient(ClientConfig{})

	_, err := c.CallService(CallServiceRequest{
		Type:    MessageTypeCallService,
		Domain:  "light",
		Service: "turn_on",
		Data:    map[string]any{"entity_id": []string{"light.test"}},
	})
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestGetStatesNotConnected(t *testing.T) {
	t.Parallel()

	c := NewClient(ClientConfig{})

	_, err := c.GetStates()
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestSendMarshalError(t *testing.T) {
	t.Parallel()

	c := NewClient(ClientConfig{})

	// channels cannot be marshalled to JSON, so send returns the marshal error
	// before ever touching the connection.
	err := c.send(make(chan int))
	assert.Assert(t, err != nil)
}

func TestSendNotConnected(t *testing.T) {
	t.Parallel()

	c := NewClient(ClientConfig{})

	// A marshalable message on a client that never connected must return
	// ErrNotConnected rather than panicking on a nil conn.
	err := c.send(CommandMessage{ID: 1, Type: MessageTypePing})
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestSendMessageStreamResponsesInvalidJSON(t *testing.T) {
	t.Parallel()

	c := NewClient(ClientConfig{})

	_, err := c.sendMessageStreamResponses([]byte("not json"))
	assert.Assert(t, err != nil)
}

// TestSendMessageStreamResponsesRemovesListenerOnSendFailure is a regression
// test: when the underlying send fails, the response listener registered for the
// message must be removed instead of leaking in the responses map.
func TestSendMessageStreamResponsesRemovesListenerOnSendFailure(t *testing.T) {
	t.Parallel()

	c := NewClient(ClientConfig{})

	// Not connected, so send fails with ErrNotConnected after the listener was
	// registered.
	_, err := c.sendMessageStreamResponses([]byte(`{"type":"ping"}`))
	assert.ErrorIs(t, err, ErrNotConnected)

	c.mutex.RLock()
	remaining := len(c.responses)
	c.mutex.RUnlock()

	assert.Equal(t, remaining, 0)
}

func TestReadMessageFromChannel(t *testing.T) {
	t.Parallel()

	t.Run("returns a buffered message", func(t *testing.T) {
		t.Parallel()

		c := NewClient(ClientConfig{})
		ch := make(chan []byte, 1)
		ch <- []byte("hello")

		res, err := c.readMessageFromChannel(ch)
		assert.NilError(t, err)
		assert.Equal(t, string(res), "hello")
	})

	t.Run("times out on an empty channel", func(t *testing.T) {
		t.Parallel()

		c := NewClient(ClientConfig{})

		_, err := c.readMessageFromChannel(make(chan []byte))
		assert.ErrorIs(t, err, ErrReadTimeout)
	})
}

func TestCloseWithoutConnect(t *testing.T) {
	t.Parallel()

	c := NewClient(ClientConfig{})

	// Closing a client that never connected is a no-op, not a nil-pointer panic.
	assert.NilError(t, c.Close())
	assert.Equal(t, c.getState(), stateDisconnected)
}

func TestReadWithoutConnect(t *testing.T) {
	t.Parallel()

	c := NewClient(ClientConfig{})

	err := c.read(&AuthChallenge{})
	assert.ErrorIs(t, err, ErrNotConnected)
}

// --- Integration tests (drive the mock server) ---

func TestConnectAndAuthenticate(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, ClientConfig{})
	connect(t, client)

	assert.Equal(t, client.getState(), stateConnected)
}

func TestConnectDialError(t *testing.T) {
	t.Parallel()

	// Nothing listening on this address, so the dial fails.
	client := NewClient(ClientConfig{Host: "127.0.0.1:1", Token: "test-token"})

	err := client.Connect()
	assert.Assert(t, err != nil)
	assert.Equal(t, client.getState(), stateDisconnected)
}

func TestConnectAuthInvalid(t *testing.T) {
	t.Parallel()

	server, err := NewServer(map[string]string{"real-token": testUserID})
	assert.NilError(t, err)

	t.Cleanup(func() { _ = server.Close() })

	client := NewClient(ClientConfig{Host: server.ListenAddress(), Token: "wrong-token"})

	err = client.Connect()
	assert.ErrorIs(t, err, ErrAuthInvalid)
	assert.Equal(t, client.getState(), stateDisconnected)
}

func TestCallService(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, ClientConfig{})
	connect(t, client)

	resp, err := client.CallService(CallServiceRequest{
		Type:    MessageTypeCallService,
		Domain:  "light",
		Service: "turn_on",
		Data:    map[string]any{"entity_id": []string{"light.test"}},
	})
	assert.NilError(t, err)
	assert.Equal(t, resp.Success, true)
}

func TestGetStates(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, ClientConfig{})
	connect(t, client)

	// The mock server always returns an empty state list.
	states, err := client.GetStates()
	assert.NilError(t, err)
	assert.Equal(t, len(states), 0)
}

func TestSubscribeEvents(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, ClientConfig{})
	connect(t, client)

	var (
		mu       sync.Mutex
		received []EventMessage
	)

	err := client.SubscribeEvents("", func(m EventMessage) {
		mu.Lock()
		received = append(received, m)
		mu.Unlock()
	})
	assert.NilError(t, err)

	waitForCond(t, "subscription registered", func() bool {
		return server.GetSubscriptionCount() >= 1
	})

	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "light.test",
			NewState: &homeassistant.State{State: "on"},
		},
	})

	waitForCond(t, "event delivered to handler", func() bool {
		mu.Lock()
		defer mu.Unlock()

		return len(received) == 1
	})

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, received[0].Event.EventData.EntityID, "light.test")
}

func TestSubscribeEventsRaw(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, ClientConfig{})
	connect(t, client)

	var (
		mu       sync.Mutex
		received [][]byte
	)

	err := client.SubscribeEventsRaw("", func(b []byte) {
		mu.Lock()
		received = append(received, b)
		mu.Unlock()
	})
	assert.NilError(t, err)

	waitForCond(t, "subscription registered", func() bool {
		return server.GetSubscriptionCount() >= 1
	})

	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "light.test",
			NewState: &homeassistant.State{State: "on"},
		},
	})

	waitForCond(t, "raw event delivered to handler", func() bool {
		mu.Lock()
		defer mu.Unlock()

		return len(received) == 1
	})

	mu.Lock()
	defer mu.Unlock()

	assert.Assert(t, bytes.Contains(received[0], []byte("light.test")))
}

// TestHeartbeatKeepsConnectionAlive verifies that the heartbeat pings (and the
// pong responses that reset the read deadline) keep a quiet connection alive
// well past the read timeout.
func TestHeartbeatKeepsConnectionAlive(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, ClientConfig{
		PingInterval: 100 * time.Millisecond,
		ReadTimeout:  600 * time.Millisecond,
	})
	connect(t, client)

	// Wait through several ping intervals. If pings/pongs were not flowing, the
	// read deadline would fire and the client would disconnect.
	time.Sleep(400 * time.Millisecond)

	assert.Equal(t, client.getState(), stateConnected)
}

// TestStaleConnectionTriggersDisconnect verifies that when the server stops
// responding (no pongs), the read deadline expires and the client disconnects.
func TestStaleConnectionTriggersDisconnect(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, ClientConfig{
		PingInterval: 100 * time.Millisecond,
		ReadTimeout:  300 * time.Millisecond,
	})

	server.SetRespondToPings(false)

	var disconnected atomic.Bool

	// Set the callback before connecting so the listen goroutine observes it.
	client.SetOnDisconnected(func() { disconnected.Store(true) })

	assert.NilError(t, client.Connect())

	t.Cleanup(func() { _ = client.Close() })

	waitForCond(t, "disconnect on stale connection", func() bool {
		return disconnected.Load()
	})

	assert.Equal(t, client.getState(), stateDisconnected)
}

// TestForcedDisconnect verifies that an ungraceful socket close is detected and
// surfaces through the onDisconnected callback.
func TestForcedDisconnect(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, ClientConfig{})

	var disconnected atomic.Bool

	client.SetOnDisconnected(func() { disconnected.Store(true) })

	assert.NilError(t, client.Connect())

	t.Cleanup(func() { _ = client.Close() })

	assert.NilError(t, server.DisconnectClient())

	waitForCond(t, "disconnect detected", func() bool {
		return disconnected.Load()
	})

	assert.Equal(t, client.getState(), stateDisconnected)
}

// TestGracefulServerClose verifies that a normal websocket close frame from the
// server drives the client's shutdown path.
func TestGracefulServerClose(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, ClientConfig{})

	var disconnected atomic.Bool

	client.SetOnDisconnected(func() { disconnected.Store(true) })

	assert.NilError(t, client.Connect())

	assert.NilError(t, server.Close())

	waitForCond(t, "disconnect after graceful close", func() bool {
		return disconnected.Load()
	})

	assert.Equal(t, client.getState(), stateDisconnected)
}
