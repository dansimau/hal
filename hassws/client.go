package hassws

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/logger"
	"github.com/gorilla/websocket"
)

const readTimeoutSeconds = 3

const (
	// defaultPingInterval is how often a heartbeat ping is sent to Home
	// Assistant to keep the connection active and prove it is still alive.
	defaultPingInterval = 30 * time.Second

	// defaultReadTimeout is how long we wait to receive any data (including
	// pong responses) before considering the connection stale and closing it
	// so it can be re-established. Must be larger than defaultPingInterval.
	defaultReadTimeout = 60 * time.Second
)

// Connection states
type connectionState int

const (
	stateDisconnected connectionState = iota
	stateConnecting
	stateConnected
)

var (
	ErrAuthInvalid        = errors.New("invalid access token")
	ErrNotConnected       = errors.New("websocket not connected")
	ErrReadTimeout        = errors.New("read timeout")
	ErrUnexpectedResponse = errors.New("unexpected response")
)

type Client struct {
	cfg  ClientConfig
	conn *websocket.Conn

	// writeMutex serializes writes to the websocket. Gorilla does not support
	// concurrent writers, and the heartbeat goroutine writes concurrently with
	// service calls and subscriptions.
	writeMutex sync.Mutex

	msgID atomic.Int64

	// Each request has a unique ID and any response will have the same ID. To
	// provide a synchronous API, we store a channel for each request and stream
	// the response there.
	responses map[int]chan []byte
	mutex     sync.RWMutex

	// Connection state tracking
	state          atomic.Value // connectionState
	onDisconnected func()
}

type ClientConfig struct {
	Host  string
	Token string

	// PingInterval is how often to send a heartbeat ping to Home Assistant.
	// Defaults to defaultPingInterval if zero.
	PingInterval time.Duration

	// ReadTimeout is how long to wait for any data before considering the
	// connection stale and reconnecting. Defaults to defaultReadTimeout if
	// zero. Should be larger than PingInterval.
	ReadTimeout time.Duration
}

func NewClient(config ClientConfig) *Client {
	if config.PingInterval == 0 {
		config.PingInterval = defaultPingInterval
	}

	if config.ReadTimeout == 0 {
		config.ReadTimeout = defaultReadTimeout
	}

	c := &Client{
		cfg:       config,
		responses: make(map[int]chan []byte),
	}
	c.setState(stateDisconnected)
	return c
}

// getState returns the current connection state
func (c *Client) getState() connectionState {
	v := c.state.Load()
	if v == nil {
		return stateDisconnected
	}
	return v.(connectionState)
}

// setState updates the connection state
func (c *Client) setState(s connectionState) {
	c.state.Store(s)
}

// SetOnDisconnected sets the callback to be called when the connection is lost
func (c *Client) SetOnDisconnected(fn func()) {
	c.onDisconnected = fn
}

func (c *Client) authenticate() error {
	// Read auth_required message from server
	var authRequired AuthChallenge
	if err := c.read(&authRequired); err != nil {
		return err
	}

	logger.Debug("Authenticating", "")

	// Send auth message with access token
	if err := c.send(AuthRequest{
		Type:        "auth",
		AccessToken: c.cfg.Token,
	}); err != nil {
		return err
	}

	// Read auth_ok or auth_invalid response
	var authResponse AuthResponse
	if err := c.read(&authResponse); err != nil {
		return err
	}

	logger.Debug("Received auth response", "", "msg", authResponse)

	if authResponse.Type == "auth_invalid" {
		return fmt.Errorf("%w: %s", ErrAuthInvalid, authResponse.Message)
	}

	if authResponse.Type != "auth_ok" {
		return fmt.Errorf("%w: %s", ErrUnexpectedResponse, authResponse.Message)
	}

	logger.Debug("Authenticated", "")

	return nil
}

func (c *Client) Close() error {
	c.setState(stateDisconnected)

	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	return c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
}

func (c *Client) shutdown() error {
	c.setState(stateDisconnected)
	return c.conn.Close()
}

func (c *Client) Connect() error {
	c.setState(stateConnecting)

	logger.Info("Connecting", "", "host", c.cfg.Host)

	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/api/websocket", c.cfg.Host), nil)
	if err != nil {
		c.setState(stateDisconnected)
		return err
	}

	logger.Debug("Connection established", "")

	c.conn = conn

	if err := c.authenticate(); err != nil {
		c.setState(stateDisconnected)
		return err
	}

	c.setState(stateConnected)

	// done is closed when the listen loop for this connection exits. It is used
	// to stop the heartbeat goroutine so it does not outlive the connection.
	done := make(chan struct{})

	// Start listening for messages and sending heartbeat pings. Both capture
	// the current connection so they are unaffected by later reconnections
	// reassigning c.conn.
	go c.listen(conn, done)
	go c.heartbeat(conn, done)

	return nil
}

// heartbeat periodically sends a ping to Home Assistant to keep the connection
// active and generate traffic during quiet periods. The pong response resets
// the read deadline in listen(). If a ping fails to send, the connection is
// closed so listen() returns and reconnection is triggered.
func (c *Client) heartbeat(conn *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(c.cfg.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := c.ping(conn); err != nil {
				logger.Error("Heartbeat ping failed, closing connection", "", "error", err)

				// Force the connection closed so the listen loop returns and
				// the reconnection logic kicks in.
				_ = conn.Close()

				return
			}
		}
	}
}

// ping sends a Home Assistant ping message on the given connection.
func (c *Client) ping(conn *websocket.Conn) error {
	msg := CommandMessage{
		ID:   c.nextMsgID(),
		Type: MessageTypePing,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	logger.DebugJSON("Writing message", "", string(msgBytes))

	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	return conn.WriteMessage(websocket.TextMessage, msgBytes)
}

// Listen for messages from the websocket and dispatch to listener channels.
func (c *Client) listen(conn *websocket.Conn, done chan struct{}) {
	logger.Info("Connection established", "")

	defer func() {
		// Signal the heartbeat goroutine for this connection to stop.
		close(done)

		// Close all pending response channels to unblock waiting callers
		c.mutex.Lock()
		for msgID, ch := range c.responses {
			// Safely close channel (recover from panic if already closed)
			func() {
				defer func() {
					_ = recover() // Ignore panic if channel already closed
				}()
				close(ch)
			}()
			delete(c.responses, msgID)
		}
		c.mutex.Unlock()

		// Set state to disconnected
		c.setState(stateDisconnected)

		// Signal disconnection to the Connection layer for reconnection
		if c.onDisconnected != nil {
			c.onDisconnected()
		}
	}()

	for {
		// Reset the read deadline before each read. If no data (including
		// heartbeat pong responses) arrives within ReadTimeout, ReadMessage
		// returns an error, the loop exits and reconnection is triggered. This
		// detects "stuck" connections that remain open but stop delivering data.
		if err := conn.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout)); err != nil {
			logger.Error("Error setting read deadline", "", "error", err)

			return
		}

		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				logger.Info("Received close message", "")

				if err := c.shutdown(); err != nil {
					logger.Error("Error during shutdown", "", "error", err)
				}

				return
			}

			// Log error and return gracefully instead of panicking
			logger.Error("Error reading from websocket", "", "error", err)
			return
		}

		logger.DebugJSON("Received message", "", string(msgBytes))

		// Get message ID
		var msg CommandMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			logger.Error("Error unmarshalling message", "", "error", err)

			continue
		}

		// Pong responses to our heartbeat pings have no registered listener;
		// receiving one has already reset the read deadline above, so just
		// move on without logging a spurious "no listeners" warning.
		if msg.Type == MessageTypePong {
			continue
		}

		c.mutex.RLock()
		responseListenerCh, ok := c.responses[msg.ID]
		c.mutex.RUnlock()

		if !ok {
			logger.Warn("No listeners for message", "", "id", msg.ID)

			continue
		}

		go func(responseListenerCh chan []byte) {
			defer func() {
				if r := recover(); r != nil {
					c.removeMessageResponseListener(msg.ID)
					logger.Debug("Removed listener due to panic sending to channel", "", "id", msg.ID)
				}
			}()

			responseListenerCh <- msgBytes
		}(responseListenerCh)
	}
}

// Generate a unique ID for each message.
func (c *Client) nextMsgID() int {
	return int(c.msgID.Add(1))
}

// Read a message from the websocket and unmarshal it into the target.
func (c *Client) read(target any) error {
	_, msgBytes, err := c.conn.ReadMessage()
	if err != nil {
		return err
	}

	logger.DebugJSON("Received message", "", string(msgBytes))

	return json.Unmarshal(msgBytes, target)
}

// Send a message to the websocket.
func (c *Client) send(msg any) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	logger.DebugJSON("Writing message", "", string(msgBytes))

	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	return c.conn.WriteMessage(websocket.TextMessage, msgBytes)
}

// Add a listener channel for a response to a specific sent message.
func (c *Client) addMessageResponseListener(msgID int) (ch chan []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	ch = make(chan []byte)
	c.responses[msgID] = ch

	return ch
}

func (c *Client) removeMessageResponseListener(msgID int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.responses, msgID)
}

// Send a message to the websocket and return a channel to listen for responses.
func (c *Client) sendMessageStreamResponses(msgBytes []byte) (ch chan []byte, err error) {
	var msg jsonMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		return nil, err
	}

	msgID := c.nextMsgID()
	msg["id"] = msgID

	ch = c.addMessageResponseListener(msgID)

	if err := c.send(msg); err != nil {
		return nil, err
	}

	return ch, nil
}

// Send a message to the websocket and wait for a response.
func (c *Client) sendMessageWaitResponse(msgBytes []byte) (response []byte, err error) {
	responseChan, err := c.sendMessageStreamResponses(msgBytes)
	if err != nil {
		return nil, err
	}

	// Close channel after receiving first response
	defer func() {
		close(responseChan)
	}()

	return c.readMesssageFromChannel(responseChan)
}

// Read a message from a listener channel.
func (c *Client) readMesssageFromChannel(ch chan []byte) (response []byte, err error) {
	select {
	case res := <-ch:
		return res, nil
	case <-time.After(readTimeoutSeconds * time.Second):
		return nil, ErrReadTimeout
	}
}

// Subscribe to home assistant events.
func (c *Client) SubscribeEvents(eventType string, handler func(EventMessage)) error {
	msg := subscribeEventsRequest{
		ID:        c.nextMsgID(),
		Type:      MessageTypeSubscribeEvents,
		EventType: eventType,
	}

	reqBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	responseChan, err := c.sendMessageStreamResponses(reqBytes)
	if err != nil {
		return err
	}

	// First message contains the initial response about the subscription
	resBytes, err := c.readMesssageFromChannel(responseChan)
	if err != nil {
		close(responseChan)

		return err
	}

	var res subscribeEventsResponse
	if err := json.Unmarshal(resBytes, &res); err != nil {
		close(responseChan)

		return err
	}

	if !res.Success {
		close(responseChan)

		return fmt.Errorf("%w: %s", ErrUnexpectedResponse, resBytes)
	}

	// Create Goroutine to event messages and dispatch to handler
	go func(ch chan []byte) {
		for b := range ch {
			var msg EventMessage
			if err := json.Unmarshal(b, &msg); err != nil {
				logger.Error("Error unmarshalling event message", "", "error", err)

				continue
			}

			handler(msg)
		}
	}(responseChan)

	logger.Info("Listening for state changes", "")

	return nil
}

// SubscribeEventsRaw subscribes to home assistant events and passes the raw
// JSON frame bytes to the handler instead of a parsed EventMessage. Useful
// when callers need the unmodified payload (e.g. to display unknown fields).
func (c *Client) SubscribeEventsRaw(eventType string, handler func([]byte)) error {
	msg := subscribeEventsRequest{
		ID:        c.nextMsgID(),
		Type:      MessageTypeSubscribeEvents,
		EventType: eventType,
	}

	reqBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	responseChan, err := c.sendMessageStreamResponses(reqBytes)
	if err != nil {
		return err
	}

	resBytes, err := c.readMesssageFromChannel(responseChan)
	if err != nil {
		close(responseChan)

		return err
	}

	var res subscribeEventsResponse
	if err := json.Unmarshal(resBytes, &res); err != nil {
		close(responseChan)

		return err
	}

	if !res.Success {
		close(responseChan)

		return fmt.Errorf("%w: %s", ErrUnexpectedResponse, resBytes)
	}

	go func(ch chan []byte) {
		for b := range ch {
			handler(b)
		}
	}(responseChan)

	return nil
}

func (c *Client) CallService(msg CallServiceRequest) (CallServiceResponse, error) {
	if c.getState() != stateConnected {
		return CallServiceResponse{}, ErrNotConnected
	}

	reqBytes, err := json.Marshal(msg)
	if err != nil {
		return CallServiceResponse{}, err
	}

	resBytes, err := c.sendMessageWaitResponse(reqBytes)
	if err != nil {
		return CallServiceResponse{}, err
	}

	var resp CallServiceResponse
	if err := json.Unmarshal(resBytes, &resp); err != nil {
		return CallServiceResponse{}, err
	}

	if resp.Type == MessageTypeResult && !resp.Success {
		logger.Error("Call service failed", "", "err", resp.Error)
	}

	return resp, nil
}

func (c *Client) GetStates() ([]homeassistant.State, error) {
	if c.getState() != stateConnected {
		return nil, ErrNotConnected
	}

	msg := CommandMessage{
		ID:   c.nextMsgID(),
		Type: MessageTypeGetStates,
	}

	reqBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	resBytes, err := c.sendMessageWaitResponse(reqBytes)
	if err != nil {
		return nil, err
	}

	var resp CommandResponse
	if err := json.Unmarshal(resBytes, &resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("%w: %s", ErrUnexpectedResponse, resp.Error)
	}

	var states []homeassistant.State
	if err := json.Unmarshal(resp.Result, &states); err != nil {
		return nil, err
	}

	return states, nil
}
