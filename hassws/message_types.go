package hassws

import (
	"encoding/json"

	"github.com/dansimau/hal/homeassistant"
)

const (
	MessageTypeAuthChallenge   MessageType = "auth_challenge"
	MessageTypeAuthRequest     MessageType = "auth_request"
	MessageTypeAuthResponse    MessageType = "auth_response"
	MessageTypeCallService     MessageType = "call_service"
	MessageTypeEvent           MessageType = "event"
	MessageTypeGetStates       MessageType = "get_states"
	MessageTypeResult          MessageType = "result"
	MessageTypeStateChanged    MessageType = "state_changed"
	MessageTypeSubscribeEvents MessageType = "subscribe_events"
)

type MessageType string

type CommandMessage struct {
	ID   int         `json:"id"`
	Type MessageType `json:"type"`
}

type CommandResponse struct {
	ID      int             `json:"id"`
	Type    MessageType     `json:"type"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

type AuthChallenge struct {
	Type      string `json:"type"`
	HAVersion string `json:"ha_version,omitempty"`
}

type AuthRequest struct {
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
}

type AuthResponse struct {
	Type      string `json:"type"`
	Message   string `json:"message,omitempty"`
	HAVersion string `json:"ha_version,omitempty"`
}

type EventMessage struct {
	ID        int                 `json:"id"`
	Type      MessageType         `json:"type"` // "event"
	Event     homeassistant.Event `json:"event"`
	EventType MessageType         `json:"event_type"`
	TimeFired string              `json:"time_fired"`
	Origin    string              `json:"origin"`
	Context   EventMessageContext `json:"context"`
}

type EventMessageContext struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	UserID   string `json:"user_id"`
}

type subscribeEventsRequest struct {
	ID        int         `json:"id"`
	Type      MessageType `json:"type"`
	EventType string      `json:"event_type,omitempty"`
}

type subscribeEventsResponse struct {
	ID      int         `json:"id"`
	Type    MessageType `json:"type"`
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
}

type CallServiceRequest struct {
	Type    MessageType       `json:"type"`
	Domain  string            `json:"domain"`
	Service string            `json:"service"`
	Data    map[string]any    `json:"service_data,omitempty"`
	Target  map[string]string `json:"target,omitempty"`
}

type CallServiceResponse struct {
	ID      int         `json:"id"`
	Type    MessageType `json:"type"`
	Success bool        `json:"success"`
	Result  struct {
		Context struct {
			ID string `json:"id"`
		} `json:"context"`
	} `json:"result"`
	Error map[string]any `json:"error,omitempty"`
}

type jsonMessage map[string]any
