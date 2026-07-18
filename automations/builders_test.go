package halautomations_test

import (
	"context"
	"testing"

	"github.com/dansimau/hal"
	halautomations "github.com/dansimau/hal/automations"
	"gotest.tools/v3/assert"
)

func TestPrintDebug(t *testing.T) {
	t.Parallel()

	light := hal.NewLight("light.a")
	pd := halautomations.NewPrintDebug("debug", light)

	assert.Equal(t, pd.Name(), "debug")
	assert.Equal(t, len(pd.Entities()), 1)
	assert.Equal(t, pd.Entities()[0].GetID(), "light.a")

	// Action just logs the current state; call it to cover the loop.
	pd.Action(context.Background(), light)
}

func TestSensorLightsBuilders(t *testing.T) {
	t.Parallel()

	sensor := hal.NewBinarySensor("binary_sensor.a")
	onLight := hal.NewLight("light.on")
	offLight := hal.NewLight("light.off")

	a := halautomations.NewSensorsTriggerLights().
		WithName("test").
		WithSensors(sensor).
		TurnsOnLights(onLight).
		TurnsOffLights(offLight).
		WithCondition(func() bool { return true }).
		WithConditionScene(func() bool { return false }, map[string]any{"brightness": 10}).
		SetScene(map[string]any{"brightness": 200})

	assert.Equal(t, a.Name(), "test")

	ids := make(map[string]bool)
	for _, e := range a.Entities() {
		ids[e.GetID()] = true
	}

	assert.Assert(t, ids["binary_sensor.a"])
	assert.Assert(t, ids["light.on"])
}
