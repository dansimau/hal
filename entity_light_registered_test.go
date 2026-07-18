package hal_test

import (
	"context"
	"testing"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/homeassistant"
	"github.com/dansimau/hal/testutil"
	"github.com/davecgh/go-spew/spew"
)

// TestLight_TurnOn_Registered exercises the happy path where a registered light
// calls the (mock) Home Assistant service and receives the echoed state change.
func TestLight_TurnOn_Registered(t *testing.T) {
	t.Parallel()

	conn, _, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	light := hal.NewLight("light.test")
	conn.RegisterEntities(light)

	err := light.TurnOn(map[string]any{"brightness": float64(200)})
	if err != nil {
		t.Fatalf("TurnOn returned error: %v", err)
	}

	testutil.WaitFor(t, "light turned on", func() bool {
		return light.GetState().State == "on"
	}, func() {
		spew.Dump(light.GetID(), light.GetState())
	})
}

func TestLight_TurnOff_Registered(t *testing.T) {
	t.Parallel()

	conn, _, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	light := hal.NewLight("light.test")
	conn.RegisterEntities(light)

	err := light.TurnOff()
	if err != nil {
		t.Fatalf("TurnOff returned error: %v", err)
	}

	testutil.WaitFor(t, "light turned off", func() bool {
		return light.GetState().State == "off"
	}, func() {
		spew.Dump(light.GetID(), light.GetState())
	})
}

func TestLight_TurnOnContext_Registered(t *testing.T) {
	t.Parallel()

	conn, _, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	light := hal.NewLight("light.test")
	conn.RegisterEntities(light)

	err := light.TurnOnContext(context.Background(), map[string]any{"brightness": float64(128)})
	if err != nil {
		t.Fatalf("TurnOnContext returned error: %v", err)
	}

	testutil.WaitFor(t, "light turned on", func() bool {
		return light.GetState().State == "on"
	}, func() {
		spew.Dump(light.GetID(), light.GetState())
	})
}

func TestLight_TurnOffContext_Registered(t *testing.T) {
	t.Parallel()

	conn, _, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	light := hal.NewLight("light.test")
	conn.RegisterEntities(light)

	err := light.TurnOffContext(context.Background())
	if err != nil {
		t.Fatalf("TurnOffContext returned error: %v", err)
	}

	testutil.WaitFor(t, "light turned off", func() bool {
		return light.GetState().State == "off"
	}, func() {
		spew.Dump(light.GetID(), light.GetState())
	})
}

// TestLightGroup_ServicesRegistered exercises LightGroup fan-out over registered
// lights against the mock server.
func TestLightGroup_ServicesRegistered(t *testing.T) {
	t.Parallel()

	conn, _, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	light1 := hal.NewLight("light.one")
	light2 := hal.NewLight("light.two")
	conn.RegisterEntities(light1, light2)

	group := hal.LightGroup{light1, light2}

	if err := group.TurnOnContext(context.Background(), map[string]any{"brightness": float64(255)}); err != nil {
		t.Fatalf("group TurnOnContext returned error: %v", err)
	}

	testutil.WaitFor(t, "both lights on", func() bool {
		return light1.GetState().State == "on" && light2.GetState().State == "on"
	}, func() {
		spew.Dump(light1.GetState(), light2.GetState())
	})

	if !group.IsOn() {
		t.Error("expected group IsOn to be true")
	}

	if err := group.TurnOffContext(context.Background()); err != nil {
		t.Fatalf("group TurnOffContext returned error: %v", err)
	}

	testutil.WaitFor(t, "both lights off", func() bool {
		return light1.GetState().State == "off" && light2.GetState().State == "off"
	}, func() {
		spew.Dump(light1.GetState(), light2.GetState())
	})

	if !group.IsOff() {
		t.Error("expected group IsOff to be true")
	}
}

// TestLightGroup_IsOff verifies IsOff logic without a connection.
func TestLightGroup_IsOff(t *testing.T) {
	t.Parallel()

	t.Run("returns true when all lights off", func(t *testing.T) {
		t.Parallel()

		light1 := hal.NewLight("light.one")
		light2 := hal.NewLight("light.two")
		light1.SetState(homeassistant.State{State: "off"})
		light2.SetState(homeassistant.State{State: "off"})

		group := hal.LightGroup{light1, light2}
		if !group.IsOff() {
			t.Error("expected IsOff to be true")
		}
	})

	t.Run("returns false when any light on", func(t *testing.T) {
		t.Parallel()

		light1 := hal.NewLight("light.one")
		light2 := hal.NewLight("light.two")
		light1.SetState(homeassistant.State{State: "off"})
		light2.SetState(homeassistant.State{State: "on"})

		group := hal.LightGroup{light1, light2}
		if group.IsOff() {
			t.Error("expected IsOff to be false")
		}
	})
}
