package homeassistant

import (
	"time"
)

const (
	EventTypeStateChanged = "state_changed"
)

type State struct {
	EntityID string `json:"entity_id"`

	State      string         `json:"state"`
	Attributes map[string]any `json:"attributes"`

	LastChanged  time.Time `json:"last_changed"`
	LastReported time.Time `json:"last_reported"`
	LastUpdated  time.Time `json:"last_updated"`
}
