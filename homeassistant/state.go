package homeassistant

import (
	"encoding/json"
	"time"
)

const (
	EventTypeStateChanged = "state_changed"
)

type State struct {
	EntityID string `json:"entity_id"`

	State      string `json:"state"`
	Attributes struct {
		DeviceClass  string `json:"device_class"`
		FriendlyName string `json:"friendly_name"`

		json.RawMessage
	} `json:"attributes"`

	LastChanged  time.Time `json:"last_changed"`
	LastReported time.Time `json:"last_reported"`
	LastUpdated  time.Time `json:"last_updated"`
}
