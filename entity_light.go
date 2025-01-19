package hal

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/homeassistant"
)

type LightInterface interface {
	EntityInterface

	GetBrightness() float64
	IsOn() bool
	TurnOn(attributes ...map[string]any) error
	TurnOff() error
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
	if l.connection == nil {
		slog.Error("Light not registered", "entity", l.GetID())

		return ErrEntityNotRegistered
	}

	slog.Debug("Turning on light", "entity", l.GetID())

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
		slog.Error("Error turning on light", "entity", l.GetID(), "error", err)

		return err
	}

	return nil
}

func (l *Light) TurnOff() error {
	if l.connection == nil {
		slog.Error("Light not registered", "entity", l.GetID())

		return ErrEntityNotRegistered
	}

	slog.Info("Turning off light", "entity", l.GetID())

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
		slog.Error("Error turning off light", "entity", l.GetID(), "error", err)

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

func (lg LightGroup) TurnOn(attributes ...map[string]any) error {
	var errs []error

	for _, l := range lg {
		if err := l.TurnOn(attributes...); err != nil {
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
