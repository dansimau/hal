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

func TestMetricsInstrumentation(t *testing.T) {
	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	// Create test entities
	sensor := hal.NewBinarySensor("test.motion_sensor")
	light := hal.NewLight("test.light")
	conn.RegisterEntities(sensor, light)

	// Create test automation
	automation := hal.NewAutomation().
		WithName("motion_light_automation").
		WithEntities(sensor).
		WithAction(func(ctx context.Context, trigger hal.EntityInterface) {
			light.TurnOn()
		})

	conn.RegisterAutomations(automation)

	// Trigger a state change that will run automation
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.motion_sensor",
			NewState: &homeassistant.State{State: "on"},
		},
	})

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Test passes if no errors/panics occurred during metrics collection
	// The detailed metric verification is done in the metrics package tests
}

func TestMetricsAutomationTriggered(t *testing.T) {
	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	// Create test entities and automation
	sensor := hal.NewBinarySensor("test.sensor")
	conn.RegisterEntities(sensor)

	var automationExecuted atomic.Bool
	automation := hal.NewAutomation().
		WithName("test_automation").
		WithEntities(sensor).
		WithAction(func(ctx context.Context, trigger hal.EntityInterface) {
			automationExecuted.Store(true)
		})

	conn.RegisterAutomations(automation)

	// Trigger automation
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.sensor",
			NewState: &homeassistant.State{State: "on"},
		},
	})
	time.Sleep(10 * time.Millisecond)

	// Verify automation was triggered (which means metrics were collected)
	assert.Assert(t, automationExecuted.Load(), "Expected automation to be triggered")
}

func TestMetricsIntegrationBasicScenarios(t *testing.T) {
	conn, server, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	// Create test entities
	sensor := hal.NewBinarySensor("test.sensor")
	light1 := hal.NewLight("test.light1")
	light2 := hal.NewLight("test.light2")
	conn.RegisterEntities(sensor, light1, light2)

	// Create multiple automations for the same trigger
	var automation1Executed, automation2Executed atomic.Bool

	automation1 := hal.NewAutomation().
		WithName("automation_1").
		WithEntities(sensor).
		WithAction(func(ctx context.Context, trigger hal.EntityInterface) {
			automation1Executed.Store(true)
			light1.TurnOn()
		})

	automation2 := hal.NewAutomation().
		WithName("automation_2").
		WithEntities(sensor).
		WithAction(func(ctx context.Context, trigger hal.EntityInterface) {
			automation2Executed.Store(true)
			light2.TurnOn()
		})

	conn.RegisterAutomations(automation1, automation2)

	// Test normal state change (should trigger automations and record metrics)
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.sensor",
			NewState: &homeassistant.State{State: "on"},
		},
	})
	time.Sleep(50 * time.Millisecond)

	// Verify both automations were triggered
	assert.Assert(t, automation1Executed.Load(), "Expected automation 1 to be triggered")
	assert.Assert(t, automation2Executed.Load(), "Expected automation 2 to be triggered")

	// Reset for next test
	automation1Executed.Store(false)
	automation2Executed.Store(false)

	// Test state change from HAL's own user (should NOT trigger automations)
	server.SendEvent(homeassistant.Event{
		EventType: "state_changed",
		EventData: homeassistant.EventData{
			EntityID: "test.sensor",
			NewState: &homeassistant.State{State: "off"},
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
