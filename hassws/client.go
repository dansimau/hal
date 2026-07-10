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
	defaultPingInterval      = 30 * time.Second
	defaultPongTimeout       = 60 * time.Second
	defaultInactivityTimeout = 10 * time.Minute
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

	msgID atomic.Int64

	// Each request has a unique ID and any response will have the same ID. To
	// provide a synchronous API, we store a channel for each request and stream
	// the response there.
	responses map[int]chan []byte
	mutex     sync.RWMutex
	writeMu   sync.Mutex

	// Connection state tracking
	state          atomic.Value // connectionState
	onDisconnected func()

	pingInterval      time.Duration
	pongTimeout       time.Duration
	inactivityTimeout time.Duration
	lastDataReceived  atomic.Int64
}

type ClientConfig struct {
	Host  string
	Token string

	// PingInterval controls how often websocket ping frames are sent. Set to 0
	// to use the default, or a negative value to disable heartbeat pings.
	PingInterval time.Duration

	// PongTimeout controls how long the client waits for any websocket frame or
	// pong response before treating the connection as dead. Set to 0 to use the
	// default, or a negative value to disable read deadlines.
	PongTimeout time.Duration

	// InactivityTimeout controls how long the client allows the Home Assistant
	// event stream to be quiet before reconnecting. Set to 0 to use the default,
	// or a negative value to disable application-level inactivity detection.
	InactivityTimeout time.Duration
}

func NewClient(config ClientConfig) *Client {
	pingInterval := config.PingInterval
	if pingInterval == 0 {
		pingInterval = defaultPingInterval
	}

	pongTimeout := config.PongTimeout
	if pongTimeout == 0 {
		pongTimeout = defaultPongTimeout
	}

	inactivityTimeout := config.InactivityTimeout
	if inactivityTimeout == 0 {
		inactivityTimeout = defaultInactivityTimeout
	}

	c := &Client{
		cfg:               config,
		responses:         make(map[int]chan []byte),
		pingInterval:      pingInterval,
		pongTimeout:       pongTimeout,
		inactivityTimeout: inactivityTimeout,
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
	if c.conn == nil {
		return nil
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
}

func (c *Client) shutdown() error {
	c.setState(stateDisconnected)
	if c.conn == nil {
		return nil
	}

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

	c.lastDataReceived.Store(time.Now().UnixNano())

	// Start listening for messages and monitoring connection health.
	go c.listen()
	go c.heartbeat()

	return nil
}

// Listen for messages from the websocket and dispatch to listener channels.
func (c *Client) listen() {
	logger.Info("Connection established", "")

	defer func() {
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

	c.configureReadDeadline()

	for {
		_, msgBytes, err := c.conn.ReadMessage()
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

		c.markDataReceived()

		logger.DebugJSON("Received message", "", string(msgBytes))

		// Get message ID
		var msg CommandMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			logger.Error("Error unmarshalling message", "", "error", err)

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

func (c *Client) configureReadDeadline() {
	if c.pongTimeout < 0 {
		return
	}

	deadline := time.Now().Add(c.pongTimeout)
	_ = c.conn.SetReadDeadline(deadline)
	c.conn.SetPongHandler(func(string) error {
		logger.Debug("Received websocket pong", "")
		return c.conn.SetReadDeadline(time.Now().Add(c.pongTimeout))
	})
}

func (c *Client) markDataReceived() {
	c.lastDataReceived.Store(time.Now().UnixNano())
	if c.pongTimeout >= 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.pongTimeout))
	}
}

func (c *Client) heartbeat() {
	if c.pingInterval < 0 && c.inactivityTimeout < 0 {
		return
	}

	interval := c.pingInterval
	if interval < 0 || (c.inactivityTimeout > 0 && c.inactivityTimeout < interval) {
		interval = c.inactivityTimeout
	}
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if c.getState() != stateConnected {
			return
		}

		if c.inactivityTimeout > 0 {
			last := time.Unix(0, c.lastDataReceived.Load())
			if time.Since(last) > c.inactivityTimeout {
				logger.Warn("No Home Assistant websocket data received; reconnecting", "", "timeout", c.inactivityTimeout)
				if err := c.shutdown(); err != nil {
					logger.Error("Error closing inactive websocket", "", "error", err)
				}
				return
			}
		}

		if c.pingInterval < 0 {
			continue
		}

		c.writeMu.Lock()
		err := c.conn.WriteControl(websocket.PingMessage, []byte("hal heartbeat"), time.Now().Add(5*time.Second))
		c.writeMu.Unlock()
		if err != nil {
			logger.Error("Error sending websocket ping", "", "error", err)
			if err := c.shutdown(); err != nil {
				logger.Error("Error closing websocket after ping failure", "", "error", err)
			}
			return
		}
	}
}

// Send a message to the websocket.
func (c *Client) send(msg any) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	logger.DebugJSON("Writing message", "", string(msgBytes))

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

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
