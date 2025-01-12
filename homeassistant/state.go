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

func (s *State) Update(newState State) {
	if newState.State != "" {
		s.State = newState.State
	}

	for k, v := range newState.Attributes {
		if s.Attributes == nil {
			s.Attributes = make(map[string]any)
		}

		s.Attributes[k] = v
	}

	if newState.LastChanged != (time.Time{}) {
		s.LastChanged = newState.LastChanged
	}

	if newState.LastReported != (time.Time{}) {
		s.LastReported = newState.LastReported
	}

	if newState.LastUpdated != (time.Time{}) {
		s.LastUpdated = newState.LastUpdated
	}
}
