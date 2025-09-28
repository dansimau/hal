package hal

import (
	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/logger"
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
		logger.Error("InputBoolean not registered", entityID)

		return ErrEntityNotRegistered
	}

	logger.Debug("Turning on virtual switch", entityID)

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
		logger.Error("Error turning on virtual switch", entityID, "error", err)
	}

	return err
}

func (s *InputBoolean) TurnOff() error {
	entityID := s.GetID()
	if s.connection == nil {
		logger.Error("InputBoolean not registered", entityID)

		return ErrEntityNotRegistered
	}

	logger.Info("Turning off virtual switch", entityID)

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
		logger.Error("Error turning off virtual switch", entityID, "error", err)
	}

	return err
}
