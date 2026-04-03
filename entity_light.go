package hal

import (
	"context"
	"errors"
	"strings"

	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/logger"
)

type LightInterface interface {
	EntityInterface

	GetBrightness() float64
	IsOn() bool
	TurnOn(attributes ...map[string]any) error
	TurnOnContext(ctx context.Context, attributes ...map[string]any) error
	TurnOff() error
	TurnOffContext(ctx context.Context) error
}

type Light struct {
	*Entity
}

func NewLight(id string) *Light {
	return &Light{Entity: NewEntity(id)}
}

func (l *Light) GetBrightness() float64 {
	if v, ok := l.Entity.GetState().Attributes["brightness"].(float64); ok {
		return v
	}

	return 0
}

func (l *Light) IsOn() bool {
	return l.Entity.GetState().State == "on"
}

func (l *Light) TurnOn(attributes ...map[string]any) error {
	entityID := l.GetID()
	if l.connection == nil {
		logger.Error("Light not registered", entityID)

		return ErrEntityNotRegistered
	}

	logger.Info("Turning on light", entityID)

	data := map[string]any{
		"entity_id": []string{l.GetID()},
	}

	for _, attribute := range attributes {
		for k, v := range attribute {
			data[k] = v
		}
	}

	_, err := l.connection.CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "light",
		Service: "turn_on",
		Data:    data,
	})
	if err != nil {
		entityID := l.GetID()
		logger.Error("Error turning on light", entityID, "error", err)

		return err
	}

	return nil
}

func (l *Light) TurnOnContext(ctx context.Context, attributes ...map[string]any) error {
	if l.connection == nil {
		logger.ErrorContext(ctx, "Light not registered")

		return ErrEntityNotRegistered
	}

	logger.InfoContext(ctx, "Turning on light", "attributes", attributes)

	data := map[string]any{
		"entity_id": []string{l.GetID()},
	}

	for _, attribute := range attributes {
		for k, v := range attribute {
			data[k] = v
		}
	}

	_, err := l.connection.CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "light",
		Service: "turn_on",
		Data:    data,
	})
	if err != nil {
		logger.ErrorContext(ctx, "Error turning on light", "error", err)

		return err
	}

	return nil
}

func (l *Light) TurnOff() error {
	entityID := l.GetID()
	if l.connection == nil {
		logger.Error("Light not registered", entityID)

		return ErrEntityNotRegistered
	}

	logger.Info("Turning off light", entityID)

	data := map[string]any{
		"entity_id": []string{l.GetID()},
	}

	_, err := l.connection.CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "light",
		Service: "turn_off",
		Data:    data,
	})
	if err != nil {
		entityID := l.GetID()
		logger.Error("Error turning off light", entityID, "error", err)

		return err
	}

	return nil
}

func (l *Light) TurnOffContext(ctx context.Context) error {
	if l.connection == nil {
		logger.ErrorContext(ctx, "Light not registered")

		return ErrEntityNotRegistered
	}

	logger.InfoContext(ctx, "Turning off light")

	data := map[string]any{
		"entity_id": []string{l.GetID()},
	}

	_, err := l.connection.CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "light",
		Service: "turn_off",
		Data:    data,
	})
	if err != nil {
		logger.ErrorContext(ctx, "Error turning off light", "error", err)

		return err
	}

	return nil
}

type LightGroup []LightInterface

func (lg LightGroup) BindConnection(connection *Connection) {
	for _, l := range lg {
		l.BindConnection(connection)
	}
}

func (lg LightGroup) GetID() string {
	if len(lg) == 0 {
		return "(empty light group)"
	}

	ids := make([]string, len(lg))
	for i, l := range lg {
		ids[i] = l.GetID()
	}

	return strings.Join(ids, ", ")
}

func (lg LightGroup) GetBrightness() float64 {
	if len(lg) == 0 {
		return 0
	}

	return lg[0].GetBrightness()
}

func (lg LightGroup) GetState() homeassistant.State {
	if len(lg) == 0 {
		return homeassistant.State{}
	}

	return lg[0].GetState()
}

func (lg LightGroup) SetState(state homeassistant.State) {
	for _, l := range lg {
		l.SetState(state)
	}
}

func (lg LightGroup) IsOn() bool {
	for _, l := range lg {
		if !l.IsOn() {
			return false
		}
	}

	return true
}

func (lg LightGroup) IsOff() bool {
	for _, l := range lg {
		if l.IsOn() {
			return false
		}
	}

	return true
}

func (lg LightGroup) TurnOn(attributes ...map[string]any) error {
	var errs []error

	for _, l := range lg {
		if err := l.TurnOn(attributes...); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 1 {
		return errs[0]
	} else if len(errs) > 1 {
		return errors.Join(errs...)
	}

	return nil
}

func (lg LightGroup) TurnOnContext(ctx context.Context, attributes ...map[string]any) error {
	var errs []error

	for _, l := range lg {
		if err := l.TurnOnContext(ctx, attributes...); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 1 {
		return errs[0]
	} else if len(errs) > 1 {
		return errors.Join(errs...)
	}

	return nil
}

func (lg LightGroup) TurnOff() error {
	var errs []error

	for _, l := range lg {
		if err := l.TurnOff(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 1 {
		return errs[0]
	} else if len(errs) > 1 {
		return errors.Join(errs...)
	}

	return nil
}

func (lg LightGroup) TurnOffContext(ctx context.Context) error {
	var errs []error

	for _, l := range lg {
		if err := l.TurnOffContext(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 1 {
		return errs[0]
	} else if len(errs) > 1 {
		return errors.Join(errs...)
	}

	return nil
}
