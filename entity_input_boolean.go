package hal

import (
	"log/slog"

	"github.com/dansimau/hal/hassws"
)

// InputBoolean is a virtual switch that can be turned on or off.
type InputBoolean struct {
	*Entity
}

func NewInputBoolean(id string) *InputBoolean {
	return &InputBoolean{Entity: NewEntity(id)}
}

func (s *InputBoolean) IsOff() bool {
	return s.GetState().State == "off"
}

func (s *InputBoolean) IsOn() bool {
	return s.GetState().State == "on"
}

func (s *InputBoolean) TurnOn(attributes ...map[string]any) error {
	if s.connection == nil {
		slog.Error("InputBoolean not registered", "entity", s.GetID())

		return ErrEntityNotRegistered
	}

	slog.Debug("Turning on virtual switch", "entity", s.GetID())

	data := map[string]any{
		"entity_id": []string{s.GetID()},
	}

	for _, attribute := range attributes {
		for k, v := range attribute {
			data[k] = v
		}
	}

	_, err := s.connection.CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "input_boolean",
		Service: "turn_on",
		Data:    data,
	})
	if err != nil {
		slog.Error("Error turning on virtual switch", "entity", s.GetID(), "error", err)
	}

	return err
}

func (s *InputBoolean) TurnOff() error {
	if s.connection == nil {
		slog.Error("InputBoolean not registered", "entity", s.GetID())

		return ErrEntityNotRegistered
	}

	slog.Info("Turning off virtual switch", "entity", s.GetID())

	_, err := s.connection.CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "input_boolean",
		Service: "turn_off",
		Data: map[string]any{
			"entity_id": []string{s.GetID()},
		},
	})
	if err != nil {
		slog.Error("Error turning off virtual switch", "entity", s.GetID(), "error", err)
	}

	return err
}
