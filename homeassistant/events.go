package homeassistant

type Event struct {
	EventData EventData `json:"data"`
}

type EventData struct {
	EntityID string `json:"entity_id"`
	OldState *State `json:"old_state"`
	NewState *State `json:"new_state"`
}
