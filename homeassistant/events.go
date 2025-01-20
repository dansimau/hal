package homeassistant

type Event struct {
	EventData EventData           `json:"data"`
	EventType string              `json:"event_type"`
	TimeFired string              `json:"time_fired"`
	Origin    string              `json:"origin"`
	Context   EventMessageContext `json:"context"`
}

type EventMessageContext struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	UserID   string `json:"user_id"`
}

type EventData struct {
	EntityID string `json:"entity_id"`
	OldState *State `json:"old_state"`
	NewState *State `json:"new_state"`
}
