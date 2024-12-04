package hal

import (
	"github.com/dansimau/hal/hassws"
)

type Light struct {
	*Entity
}

func NewLight(id string) *Light {
	return &Light{Entity: NewEntity(id)}
}

func (l *Light) IsOn() bool {
	return l.Entity.GetState().State == "on"
}

func (l *Light) TurnOn() error {
	if l.connection == nil {
		return ErrEntityNotRegistered
	}

	_, err := l.connection.HomeAssistant().CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "light",
		Service: "turn_on",
		Data: map[string]any{
			"entity_id": []string{l.GetID()},
		},
	})

	return err
}

func (l *Light) TurnOff() error {
	if l.connection == nil {
		return ErrEntityNotRegistered
	}

	_, err := l.connection.HomeAssistant().CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "light",
		Service: "turn_off",
		Data: map[string]any{
			"entity_id": []string{l.GetID()},
		},
	})

	return err
}
