package hal

import "testing"

func TestFindEntities(t *testing.T) {
	t.Parallel()

	type nested struct {
		Extra *Light
	}

	type home struct {
		Light  *Light
		Sensor *BinarySensor
		secret *Light // unexported: must be skipped
		Nested nested
		Group  []*Light
		ByName map[string]*Light
		NilPtr *Light // nil: must be skipped
	}

	h := home{
		Light:  NewLight("light.a"),
		Sensor: NewBinarySensor("binary_sensor.b"),
		secret: NewLight("light.secret"),
		Nested: nested{Extra: NewLight("light.c")},
		Group:  []*Light{NewLight("light.d")},
		ByName: map[string]*Light{"x": NewLight("light.e")},
	}

	// Reference the unexported field so it is not flagged as unused.
	_ = h.secret

	found := findEntities(&h)

	ids := make(map[string]bool)
	for _, e := range found {
		ids[e.GetID()] = true
	}

	for _, want := range []string{"light.a", "binary_sensor.b", "light.c", "light.d", "light.e"} {
		if !ids[want] {
			t.Errorf("expected to find %q, got %v", want, ids)
		}
	}

	if ids["light.secret"] {
		t.Error("unexported field should not be discovered")
	}

	if len(found) != 5 {
		t.Errorf("expected 5 entities, got %d (%v)", len(found), ids)
	}
}

func TestFindEntitiesNonStruct(t *testing.T) {
	t.Parallel()

	// A non-struct value yields no entities and must not panic.
	if got := findEntities(42); len(got) != 0 {
		t.Errorf("expected no entities for non-struct, got %d", len(got))
	}
}

func TestConnectionFindEntities(t *testing.T) {
	t.Parallel()

	conn := NewConnection(Config{DatabasePath: ":memory:"})

	type home struct {
		Light *Light
	}

	conn.FindEntities(&home{Light: NewLight("light.z")})

	if _, ok := conn.entities["light.z"]; !ok {
		t.Error("expected light.z to be registered")
	}
}
