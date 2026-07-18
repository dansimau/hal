package hal_test

import (
	"testing"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/testutil"
	"github.com/davecgh/go-spew/spew"
)

func TestInputBoolean_TurnOn_Registered(t *testing.T) {
	t.Parallel()

	conn, _, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	sw := hal.NewInputBoolean("input_boolean.test")
	conn.RegisterEntities(sw)

	if err := sw.TurnOn(map[string]any{"custom": "value"}); err != nil {
		t.Fatalf("TurnOn returned error: %v", err)
	}

	testutil.WaitFor(t, "switch turned on", func() bool {
		return sw.IsOn()
	}, func() {
		spew.Dump(sw.GetID(), sw.GetState())
	})
}

func TestInputBoolean_TurnOff_Registered(t *testing.T) {
	t.Parallel()

	conn, _, cleanup := testutil.NewClientServer(t)
	defer cleanup()

	sw := hal.NewInputBoolean("input_boolean.test")
	conn.RegisterEntities(sw)

	if err := sw.TurnOff(); err != nil {
		t.Fatalf("TurnOff returned error: %v", err)
	}

	testutil.WaitFor(t, "switch turned off", func() bool {
		return sw.IsOff()
	}, func() {
		spew.Dump(sw.GetID(), sw.GetState())
	})
}
