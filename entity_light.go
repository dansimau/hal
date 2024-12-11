package hal

import (
	"errors"
	"log/slog"

	"github.com/dansimau/hal/hassws"
)

type LightInterface interface {
	IsOn() bool
	TurnOn() error
	TurnOff() error
}

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
		slog.Error("Light not registered", "entity", l.GetID())
		return ErrEntityNotRegistered
	}

	slog.Debug("Turning on light", "entity", l.GetID())

	_, err := l.connection.HomeAssistant().CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "light",
		Service: "turn_on",
		Data: map[string]any{
			"entity_id": []string{l.GetID()},
		},
	})
	if err != nil {
		slog.Error("Error turning on light", "entity", l.GetID(), "error", err)
	}

	return err
}

func (l *Light) TurnOff() error {
	if l.connection == nil {
		slog.Error("Light not registered", "entity", l.GetID())
		return ErrEntityNotRegistered
	}

	slog.Info("Turning off light", "entity", l.GetID())

	_, err := l.connection.HomeAssistant().CallService(hassws.CallServiceRequest{
		Type:    hassws.MessageTypeCallService,
		Domain:  "light",
		Service: "turn_off",
		Data: map[string]any{
			"entity_id": []string{l.GetID()},
		},
	})
	if err != nil {
		slog.Error("Error turning off light", "entity", l.GetID(), "error", err)
	}

	return err
}

type LightGroup []LightInterface

func (lg LightGroup) IsOn() bool {
	for _, l := range lg {
		if !l.IsOn() {
			return false
		}
	}

	return true
}

func (lg LightGroup) TurnOn() error {
	var errs []error

	for _, l := range lg {
		if err := l.TurnOn(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 1 {
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

	if len(errs) > 1 {
		return errors.Join(errs...)
	}

	return nil
}
