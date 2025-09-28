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
	entityID := s.GetID()
	if s.connection == nil {
		// Use slog directly when connection is nil
		slog.Error("InputBoolean not registered", "entity", entityID)

		return ErrEntityNotRegistered
	}

	s.connection.loggingService.Debug("Turning on virtual switch", &entityID, "entity", entityID)

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
		entityID := s.GetID()
		s.connection.loggingService.Error("Error turning on virtual switch", &entityID, "entity", entityID, "error", err)
	}

	return err
}

func (s *InputBoolean) TurnOff() error {
	entityID := s.GetID()
	if s.connection == nil {
		// Use slog directly when connection is nil
		slog.Error("InputBoolean not registered", "entity", entityID)

		return ErrEntityNotRegistered
	}

	s.connection.loggingService.Info("Turning off virtual switch", &entityID, "entity", entityID)

	_, err := s.connection.CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "input_boolean",
		Service: "turn_off",
		Data: map[string]any{
			"entity_id": []string{s.GetID()},
		},
	})
	if err != nil {
		entityID := s.GetID()
		s.connection.loggingService.Error("Error turning off virtual switch", &entityID, "entity", entityID, "error", err)
	}

	return err
}
