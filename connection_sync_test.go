package hal

import (
	"testing"

	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

// TestSyncStatesAppliesInitialState verifies that on connect the framework
// fetches all states from Home Assistant and applies them to registered
// entities (the previously-untested half of syncStates), while skipping states
// for entities that are not registered.
func TestSyncStatesAppliesInitialState(t *testing.T) {
	t.Parallel()

	server, err := hassws.NewServer(map[string]string{"test-token": testUserID})
	assert.NilError(t, err)

	conn := NewConnection(Config{
		HomeAssistant: HomeAssistantConfig{
			Host:   server.ListenAddress(),
			Token:  "test-token",
			UserID: testUserID,
		},
		DatabasePath: ":memory:",
	})

	light := NewLight("light.kitchen")
	conn.RegisterEntities(light)

	// The server reports state for a registered entity and an unregistered one.
	// The unregistered entity exercises the `continue` branch in syncStates.
	server.SetStates([]homeassistant.State{
		{
			EntityID:   "light.kitchen",
			State:      "on",
			Attributes: map[string]any{"brightness": float64(200)},
		},
		{
			EntityID: "sensor.not_registered",
			State:    "42",
		},
	})

	go func() {
		if err := conn.Start(); err != nil {
			t.Errorf("Start() failed: %v", err)
		}
	}()

	defer func() {
		conn.Close()
		server.Close()
	}()

	// The registered light's state should be populated by the initial sync.
	waitFor(t, "light state synced from initial GetStates", func() bool {
		return light.GetState().State == "on"
	}, func() {
		t.Logf("light state: %+v", light.GetState())
	})

	assert.Equal(t, light.GetState().State, "on")
	assert.Equal(t, light.GetBrightness(), float64(200))

	// The unregistered entity must not have been created.
	_, ok := conn.entities["sensor.not_registered"]
	assert.Assert(t, !ok)
}
