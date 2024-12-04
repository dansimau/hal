package hassws

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const readTimeoutSeconds = 3

var (
	ErrAuthInvalid        = errors.New("invalid access token")
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
}

type ClientConfig struct {
	Host  string
	Token string
}

func NewWebsocketAPI(config ClientConfig) *Client {
	return &Client{
		cfg:       config,
		responses: make(map[int]chan []byte),
	}
}

func (c *Client) authenticate() error {
	// Read auth_required message from server
	var authRequired AuthChallenge
	if err := c.read(&authRequired); err != nil {
		return err
	}

	log.Println("Authenticating")

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

	slog.Debug("Received auth response", "msg", authResponse)

	if authResponse.Type == "auth_invalid" {
		return fmt.Errorf("%w: %s", ErrAuthInvalid, authResponse.Message)
	}

	if authResponse.Type != "auth_ok" {
		return fmt.Errorf("%w: %s", ErrUnexpectedResponse, authResponse.Message)
	}

	log.Println("Authenticated")

	return nil
}

func (c *Client) Close() error {
	return c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
}

func (c *Client) shutdown() error {
	return c.conn.Close()
}

func (c *Client) Connect() error {
	log.Println("Connecting to", c.cfg.Host)

	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/api/websocket", c.cfg.Host), nil)
	if err != nil {
		return err
	}

	log.Println("Connected")

	c.conn = conn

	if err := c.authenticate(); err != nil {
		return err
	}

	// Start listening for messages
	go c.listen()

	return nil
}

// Listen for messages from the websocket and dispatch to listener channels.
func (c *Client) listen() {
	for {
		_, msgBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Println("Received close message, bye")

				if err := c.shutdown(); err != nil {
					slog.Error("Error during shutdown", "error", err)
				}

				return
			}

			panic(fmt.Errorf("error reading from websocket: %w", err))
		}

		slog.Debug("Received message", "msg", string(msgBytes))

		// Get message ID
		var msg CommandMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			slog.Error("Error unmarshalling message", "error", err)

			continue
		}

		c.mutex.RLock()
		responseListenerCh, ok := c.responses[msg.ID]
		c.mutex.RUnlock()

		if !ok {
			slog.Warn("No listeners for message", "id", msg.ID)

			continue
		}

		go func(responseListenerCh chan []byte) {
			defer func() {
				if r := recover(); r != nil {
					c.removeMessageResponseListener(msg.ID)
					slog.Debug("Removed listener due to panic sending to channel", "id", msg.ID)
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

	slog.Debug("Received message", "msg", string(msgBytes))

	return json.Unmarshal(msgBytes, target)
}

// Send a message to the websocket.
func (c *Client) send(msg any) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	slog.Debug("Writing message", "msg", string(msgBytes))

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
				slog.Error("Error unmarshalling event message", "error", err)

				continue
			}

			handler(msg)
		}
	}(responseChan)

	return nil
}

func (c *Client) CallService(msg CallServiceRequest) (CallServiceResponse, error) {
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

	return resp, nil
}
