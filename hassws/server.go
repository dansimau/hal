package hassws

import (
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dansimau/hal/homeassistant"
	"github.com/gorilla/websocket"
)

const readHeaderTimeoutSeconds = 10

// Server is a Home Assistant websocket API server, for testing.
type Server struct {
	listener  net.Listener
	http      *http.Server
	websocket *websocket.Conn

	messagesReceived [][]byte
	messagesSent     [][]byte

	// Subscribers is a list of message IDs that initiated a subscription.
	subscribers []int

	lock sync.RWMutex
}

func NewServer() (*Server, error) {
	server := &Server{
		http: &http.Server{
			ReadHeaderTimeout: readHeaderTimeoutSeconds * time.Second,
		},
	}

	server.http.Handler = http.HandlerFunc(server.handler)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server.listener = listener

	go func() {
		if err := server.http.Serve(server.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	}()

	return server, nil
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.websocket = conn
	defer conn.Close()

	if err := s.handleAuthentication(conn); err != nil {
		slog.Error("Authentication failed", "error", err)

		return
	}

	s.listen()
}

func (s *Server) listen() {
	for {
		_, messageBytes, err := s.websocket.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Println("[Server] Received close message, bye")

				if err := s.shutdown(); err != nil {
					slog.Error("Error during shutdown", "error", err)
				}

				break
			}

			slog.Error("Failed during read", "error", err)

			break
		}

		log.Printf("[Server] Received message: %s", string(messageBytes))

		s.lock.Lock()
		s.messagesReceived = append(s.messagesReceived, messageBytes)
		s.lock.Unlock()

		// Parse as CommandMessage
		var cmd CommandMessage
		if err := json.Unmarshal(messageBytes, &cmd); err != nil {
			panic(err)
		}

		switch cmd.Type {
		case MessageTypeSubscribeEvents:
			s.lock.Lock()
			s.subscribers = append(s.subscribers, cmd.ID)
			s.lock.Unlock()

			s.SendMessage(subscribeEventsResponse{
				ID:      cmd.ID,
				Type:    MessageTypeResult,
				Success: true,
			})
		default:
			panic("[Server]Unknown message type: " + cmd.Type)
		}
	}
}

func (s *Server) handleAuthentication(conn *websocket.Conn) error {
	// Send auth_required message
	authChallenge := AuthChallenge{
		Type:      "auth_required",
		HAVersion: "2024.1.0",
	}
	if err := conn.WriteJSON(authChallenge); err != nil {
		return err
	}

	// Wait for auth message
	var authReq AuthRequest
	if err := conn.ReadJSON(&authReq); err != nil {
		return err
	}

	// Accept any token
	authResp := AuthResponse{
		Type:      "auth_ok",
		HAVersion: "2024.1.0",
	}

	return conn.WriteJSON(authResp)
}

func (s *Server) Close() error {
	return s.websocket.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"),
	)
}

func (s *Server) shutdown() error {
	var errs []error

	if s.websocket != nil {
		errs = append(errs, s.websocket.Close())
	}

	if s.http != nil {
		errs = append(errs, s.http.Close())
	}

	if s.listener != nil {
		errs = append(errs, s.listener.Close())
	}

	return errors.Join(errs...)
}

func (s *Server) ListenAddress() string {
	return s.listener.Addr().String()
}

func (s *Server) SendMessage(message any) {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.websocket.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		panic(err)
	}

	s.messagesSent = append(s.messagesSent, msgBytes)

	log.Printf("[Server] Sent message: %s", string(msgBytes))
}

func (s *Server) MessagesReceived() [][]byte {
	return s.messagesReceived
}

func (s *Server) MessagesSent() [][]byte {
	return s.messagesSent
}

// SendStateChangeEvent sends a state change event to the server.
func (s *Server) SendStateChangeEvent(event homeassistant.Event) {
	for _, id := range s.subscribers {
		s.SendMessage(EventMessage{
			ID:        id,
			Type:      MessageTypeEvent,
			EventType: MessageTypeStateChanged,
			Event:     event,
		})
	}
}
