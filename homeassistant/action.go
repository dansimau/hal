package homeassistant

const (
	ActionLightTurnOn  = "light.turn_on"
	ActionLightTurnOff = "light.turn_off"
)

type Action struct {
	Action string `json:"action"`
	Target Target `json:"target"`
}

type Target struct {
	EntityID []string `json:"entity_id"`
}
