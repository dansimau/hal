package halautomations_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/dansimau/hal"
	halautomations "github.com/dansimau/hal/automations"
	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/testutil"
	"github.com/davecgh/go-spew/spew"
)

func TestSensorLightsTurnOnAfterDimming(t *testing.T) {
	t.Parallel()

	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	// Create test light
	testLight := hal.NewLight("test.light")
	conn.RegisterEntities(testLight)

	// Create test sensor
	testSensor := hal.NewBinarySensor("test.sensor")
	conn.RegisterEntities(testSensor)

	// Create automation
	automation := halautomations.NewSensorsTriggerLights().
		WithName("test automation").
		WithSensors(testSensor).
		WithLights(testLight).
		WithBrightness(100).
		TurnsOffAfter(time.Second * 3).
		DimLightsBeforeTurnOff(time.Second)

	conn.RegisterAutomations(automation)

	// Trigger motion sensor
	slog.Info("Test: Triggering motion sensor")
	server.SendStateChangeEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: testSensor.GetID(),
			NewState: &homeassistant.State{
				EntityID: testSensor.GetID(),
				State:    "on",
			},
		},
	})

	slog.Info("Test: Asserting light was turned on")
	testutil.WaitFor(t, "verify light was turned on", func() bool {
		return testLight.GetState().State == "on"
	}, func() {
		spew.Dump(testLight.GetID(), testLight.GetState())
	})

	// Clear motion sensor
	slog.Info("Test: Clearing motion sensor")
	server.SendStateChangeEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: testSensor.GetID(),
			NewState: &homeassistant.State{
				EntityID: testSensor.GetID(),
				State:    "off",
			},
		},
	})

	// TODO: Replace this with mocked time
	slog.Info("Test: Sleeping")
	time.Sleep(time.Second)

	slog.Info("Test: Asserting light was dimmed")
	testutil.WaitFor(t, "verify light was dimmed", func() bool {
		return testLight.GetState().Attributes["brightness"] == 50.0
	}, func() {
		spew.Dump(testLight.GetID(), testLight.GetState(), testLight.GetState().Attributes["brightness"])
	})

	// Trigger motion sensor again
	slog.Info("Test: Triggering motion sensor again")
	server.SendStateChangeEvent(homeassistant.Event{
		EventData: homeassistant.EventData{
			EntityID: testSensor.GetID(),
			NewState: &homeassistant.State{
				EntityID: testSensor.GetID(),
				State:    "on",
			},
		},
	})

	// Verify light is bright again
	slog.Info("Test: Asserting light is bright again")
	testutil.WaitFor(t, "verify light is bright again", func() bool {
		return testLight.GetState().Attributes["brightness"] == 100.0
	}, func() {
		spew.Dump(testLight.GetID(), testLight.GetState())
	})
}
