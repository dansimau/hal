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

// eventChannelBufferSize bounds the per-subscription response channel. The read
// loop is the single sender, so a buffered channel preserves message order
// (FIFO) while absorbing bursts. Subscription frames are drained off this
// channel promptly by runOrderedHandler (which decouples handler execution from
// draining), so this only needs to cover scheduling jitter, not a slow handler.
const eventChannelBufferSize = 256

// oneShotResponseBufferSize buffers the single expected response for one-shot
// requests (CallService, GetStates). A buffer of one lets the read loop deliver
// that response without blocking even if the caller has already timed out, and
// the listener is removed as soon as the caller is done, so unlike a
// subscription it does not retain a large buffer for the connection's lifetime.
const oneShotResponseBufferSize = 1

const (
	// defaultPingInterval is how often a heartbeat ping is sent to Home
	// Assistant to keep the connection active and prove it is still alive.
	defaultPingInterval = 30 * time.Second

	// writeTimeout bounds how long a single websocket write may block. Without
	// it, a half-open or blackholed connection could wedge a heartbeat write
	// (or any other write) indefinitely while holding writeMutex, preventing
	// reconnection and shutdown.
	writeTimeout = 10 * time.Second
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
	// connection stale and reconnecting. When zero it defaults to twice
	// PingInterval so pong responses keep the deadline alive. Must be larger
	// than PingInterval, or a quiet connection would reconnect continuously.
	ReadTimeout time.Duration
}

func NewClient(config ClientConfig) *Client {
	if config.PingInterval == 0 {
		config.PingInterval = defaultPingInterval
	}

	// Derive the read timeout from the ping interval so a custom PingInterval
	// larger than the default read timeout does not cause continuous
	// reconnects on quiet connections.
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 2 * config.PingInterval
	}

	// Guard against a misconfiguration where the read deadline would expire
	// before a heartbeat pong could arrive.
	if config.ReadTimeout <= config.PingInterval {
		logger.Warn("ReadTimeout must be larger than PingInterval; adjusting", "",
			"pingInterval", config.PingInterval, "readTimeout", config.ReadTimeout, "adjustedTo", 2*config.PingInterval)

		config.ReadTimeout = 2 * config.PingInterval
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

	return c.writeMessage(c.conn, websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
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

	return c.writeMessage(conn, websocket.TextMessage, msgBytes)
}

// Listen for messages from the websocket and dispatch to listener channels.
func (c *Client) listen(conn *websocket.Conn, done chan struct{}) {
	logger.Info("Connection established", "")

	defer func() {
		// Signal the heartbeat goroutine for this connection to stop.
		close(done)

		// Close the underlying socket so a stale/half-open connection is not
		// leaked server-side while we reconnect. Idempotent if already closed
		// (e.g. by shutdown() or the heartbeat).
		_ = conn.Close()

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

		// Deliver sequentially from this single read loop so events reach the
		// handler in the order Home Assistant sent them. Listener channels are
		// buffered and subscription frames are drained promptly by
		// runOrderedHandler, so this rarely blocks; the recover() tolerates a
		// send to a channel closed during shutdown/reconnect.
		func(responseListenerCh chan []byte) {
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

// writeMessage serializes writes to the given connection (gorilla forbids
// concurrent writers) and bounds each write with a deadline. Without the
// deadline, a half-open or blackholed connection could wedge a write
// indefinitely while holding writeMutex, blocking reconnection and shutdown.
func (c *Client) writeMessage(conn *websocket.Conn, messageType int, data []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}

	return conn.WriteMessage(messageType, data)
}

// Send a message to the websocket.
func (c *Client) send(msg any) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	logger.DebugJSON("Writing message", "", string(msgBytes))

	return c.writeMessage(c.conn, websocket.TextMessage, msgBytes)
}

// Add a listener channel for a response to a specific sent message. bufferSize
// bounds the channel: one-shot requests use a small buffer and remove the
// listener once done, while long-lived subscriptions use a larger buffer to
// absorb bursts.
func (c *Client) addMessageResponseListener(msgID, bufferSize int) (ch chan []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	ch = make(chan []byte, bufferSize)
	c.responses[msgID] = ch

	return ch
}

func (c *Client) removeMessageResponseListener(msgID int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.responses, msgID)
}

// registerAndSend assigns a unique ID to the message, registers a response
// listener with the given buffer size, and sends it. It returns the message ID
// and the listener channel. On send failure the listener is removed so it does
// not linger in c.responses.
func (c *Client) registerAndSend(msgBytes []byte, bufferSize int) (msgID int, ch chan []byte, err error) {
	var msg jsonMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		return 0, nil, err
	}

	msgID = c.nextMsgID()
	msg["id"] = msgID

	ch = c.addMessageResponseListener(msgID, bufferSize)

	if err := c.send(msg); err != nil {
		c.removeMessageResponseListener(msgID)

		return 0, nil, err
	}

	return msgID, ch, nil
}

// Send a message to the websocket and return a channel to listen for a stream of
// responses (e.g. a subscription). The listener lives for the connection's
// lifetime, so it uses a larger buffer to absorb bursts.
func (c *Client) sendMessageStreamResponses(msgBytes []byte) (ch chan []byte, err error) {
	_, ch, err = c.registerAndSend(msgBytes, eventChannelBufferSize)

	return ch, err
}

// Send a message to the websocket and wait for a single response.
func (c *Client) sendMessageWaitResponse(msgBytes []byte) (response []byte, err error) {
	msgID, responseChan, err := c.registerAndSend(msgBytes, oneShotResponseBufferSize)
	if err != nil {
		return nil, err
	}

	// One-shot request: remove the listener once we are done so neither it nor
	// its buffer lingers in c.responses for the lifetime of the connection.
	defer c.removeMessageResponseListener(msgID)

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

// frameQueue is an unbounded, order-preserving FIFO of raw websocket frames
// with a single producer and a single consumer. It lets a subscription's frames
// be moved off the shared response channel immediately, without waiting for a
// (potentially slow) handler to finish, so the read loop is never blocked.
type frameQueue struct {
	mutex  sync.Mutex
	cond   *sync.Cond
	buf    [][]byte
	closed bool
}

func newFrameQueue() *frameQueue {
	q := &frameQueue{}
	q.cond = sync.NewCond(&q.mutex)

	return q
}

// push appends a frame. It never blocks the caller (the read loop).
func (q *frameQueue) push(b []byte) {
	q.mutex.Lock()
	q.buf = append(q.buf, b)
	q.mutex.Unlock()

	q.cond.Signal()
}

// close marks the queue closed. pop returns the remaining frames and then
// reports the queue as drained.
func (q *frameQueue) close() {
	q.mutex.Lock()
	q.closed = true
	q.mutex.Unlock()

	q.cond.Broadcast()
}

// pop blocks until a frame is available and returns it, or returns ok=false once
// the queue is closed and fully drained.
func (q *frameQueue) pop() (b []byte, ok bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for len(q.buf) == 0 && !q.closed {
		q.cond.Wait()
	}

	if len(q.buf) == 0 {
		return nil, false
	}

	b = q.buf[0]
	q.buf = q.buf[1:]

	return b, true
}

// runOrderedHandler consumes raw frames from ch and invokes handle for each, in
// receipt order. It decouples draining ch from running handle: frames are moved
// off ch into an unbounded queue as fast as they arrive, while handle runs one
// at a time from that queue. This ensures a handler that blocks (e.g. one making
// a synchronous CallService whose result is delivered by the same read loop)
// can never fill ch and stall the shared read loop.
func runOrderedHandler(ch chan []byte, handle func([]byte)) {
	queue := newFrameQueue()

	go func() {
		for b := range ch {
			queue.push(b)
		}

		queue.close()
	}()

	for {
		b, ok := queue.pop()
		if !ok {
			return
		}

		handle(b)
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

	// Dispatch events to the handler in order. runOrderedHandler drains
	// responseChan promptly so a slow handler cannot back up the shared read loop.
	go runOrderedHandler(responseChan, func(b []byte) {
		var msg EventMessage
		if err := json.Unmarshal(b, &msg); err != nil {
			logger.Error("Error unmarshalling event message", "", "error", err)

			return
		}

		handler(msg)
	})

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

	// Dispatch raw frames to the handler in order. runOrderedHandler drains
	// responseChan promptly so a slow handler cannot back up the shared read loop.
	go runOrderedHandler(responseChan, handler)

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
