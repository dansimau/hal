package hal_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/testutil"
	"gotest.tools/v3/assert"
)

//nolint:paralleltest // NewConnection sets the global logger database
func TestMetricsInstrumentation(t *testing.T) {
	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	// Create test entities
	sensor := hal.NewBinarySensor("test.motion_sensor")
	light := hal.NewLight("test.light")
	conn.RegisterEntities(sensor, light)

	// Create test automation. The service call error is recorded before the
	// flag is set so it is safe to read once the flag is observed.
	var (
		automationExecuted atomic.Bool
		turnOnErr          error
	)

	automation := hal.NewAutomation().
		WithName("motion_light_automation").
		WithEntities(sensor).
		WithAction(func(_ context.Context, _ hal.EntityInterface) {
			turnOnErr = light.TurnOn() //nolint:contextcheck // TurnOn has no context-aware path to the client

			automationExecuted.Store(true)
		})

	conn.RegisterAutomations(automation)

	// Trigger a state change that will run automation
	server.SendEvent(homeassistant.Event{
		EventType: homeassistant.EventTypeStateChanged,
		EventData: homeassistant.EventData{
			EntityID: "test.motion_sensor",
			NewState: &homeassistant.State{State: stateOn},
		},
	})

	// Wait for processing
	testutil.WaitFor(t, "automation triggered", automationExecuted.Load, func() {})
	assert.NilError(t, turnOnErr)

	// Test passes if no errors/panics occurred during metrics collection
	// The detailed metric verification is done in the metrics package tests
}

//nolint:paralleltest // NewConnection sets the global logger database
func TestMetricsAutomationTriggered(t *testing.T) {
	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	// Create test entities and automation
	sensor := hal.NewBinarySensor(testSensorID)
	conn.RegisterEntities(sensor)

	var automationExecuted atomic.Bool

	automation := hal.NewAutomation().
		WithName("test_automation").
		WithEntities(sensor).
		WithAction(func(_ context.Context, _ hal.EntityInterface) {
			automationExecuted.Store(true)
		})

	conn.RegisterAutomations(automation)

	// Trigger automation
	server.SendEvent(homeassistant.Event{
		EventType: homeassistant.EventTypeStateChanged,
		EventData: homeassistant.EventData{
			EntityID: testSensorID,
			NewState: &homeassistant.State{State: stateOn},
		},
	})

	// Wait for the automation to run (which means metrics were collected).
	testutil.WaitFor(t, "automation triggered", automationExecuted.Load, func() {})
}

//nolint:paralleltest // NewConnection sets the global logger database
func TestMetricsIntegrationBasicScenarios(t *testing.T) {
	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	// Create test entities
	sensor := hal.NewBinarySensor(testSensorID)
	light1 := hal.NewLight("test.light1")
	light2 := hal.NewLight("test.light2")
	conn.RegisterEntities(sensor, light1, light2)

	// Create multiple automations for the same trigger. Each records its service
	// call error before setting its flag so the error is safe to read once the
	// flag is observed.
	var (
		automation1Executed, automation2Executed atomic.Bool
		light1Err, light2Err                     error
	)

	automation1 := hal.NewAutomation().
		WithName("automation_1").
		WithEntities(sensor).
		WithAction(func(_ context.Context, _ hal.EntityInterface) {
			light1Err = light1.TurnOn() //nolint:contextcheck // TurnOn has no context-aware path to the client

			automation1Executed.Store(true)
		})

	automation2 := hal.NewAutomation().
		WithName("automation_2").
		WithEntities(sensor).
		WithAction(func(_ context.Context, _ hal.EntityInterface) {
			light2Err = light2.TurnOn() //nolint:contextcheck // TurnOn has no context-aware path to the client

			automation2Executed.Store(true)
		})

	conn.RegisterAutomations(automation1, automation2)

	// Test normal state change (should trigger automations and record metrics)
	server.SendEvent(homeassistant.Event{
		EventType: homeassistant.EventTypeStateChanged,
		EventData: homeassistant.EventData{
			EntityID: testSensorID,
			NewState: &homeassistant.State{State: stateOn},
		},
	})

	// Wait for both automations to run.
	testutil.WaitFor(t, "both automations triggered", func() bool {
		return automation1Executed.Load() && automation2Executed.Load()
	}, func() {})

	assert.NilError(t, light1Err)
	assert.NilError(t, light2Err)

	// Reset for next test
	automation1Executed.Store(false)
	automation2Executed.Store(false)

	// Test state change from HAL's own user (should NOT trigger automations)
	server.SendEvent(homeassistant.Event{
		EventType: homeassistant.EventTypeStateChanged,
		EventData: homeassistant.EventData{
			EntityID: testSensorID,
			NewState: &homeassistant.State{State: stateOff},
		},
		Context: homeassistant.EventMessageContext{
			UserID: testutil.TestUserID,
		},
	})
	time.Sleep(50 * time.Millisecond)

	// Verify automations were NOT triggered (loop protection)
	assert.Assert(t, !automation1Executed.Load(), "Expected automation 1 NOT to be triggered for own actions")
	assert.Assert(t, !automation2Executed.Load(), "Expected automation 2 NOT to be triggered for own actions")

	// If we get here without panics/errors, metrics collection is working properly
}
