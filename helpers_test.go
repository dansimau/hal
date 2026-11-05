package hal_test

import (
	"testing"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/homeassistant"
	"gotest.tools/v3/assert"
)

// Entity IDs, state values and attribute names shared across the tests in
// this package.
const (
	testEntityID = "test.entity"
	testSensorID = "test.sensor"

	stateOn          = "on"
	stateOff         = "off"
	stateUnavailable = "unavailable"

	attrBrightness = "brightness"
	attrEventType  = "event_type"
)

// stateCheckCase is one state value together with the expected result of a
// state predicate such as IsOn or IsOff.
type stateCheckCase struct {
	name  string
	state string
	want  bool
}

// isOnCases are the standard cases for an IsOn predicate; otherState is the
// value that is neither on nor off.
func isOnCases(otherState string) []stateCheckCase {
	return []stateCheckCase{
		{name: "returns true when state is on", state: stateOn, want: true},
		{name: "returns false when state is off", state: stateOff, want: false},
		{name: "returns false when state is other value", state: otherState, want: false},
	}
}

// isOffCases are the standard cases for an IsOff predicate; otherState is the
// value that is neither on nor off.
func isOffCases(otherState string) []stateCheckCase {
	return []stateCheckCase{
		{name: "returns true when state is off", state: stateOff, want: true},
		{name: "returns false when state is on", state: stateOn, want: false},
		{name: "returns false when state is other value", state: otherState, want: false},
	}
}

// runStateCheckCases runs each case as a subtest: it sets the case's state on
// a freshly constructed entity and asserts that check returns the expected
// value.
func runStateCheckCases[E hal.EntityInterface](
	t *testing.T,
	newEntity func() E,
	check func(E) bool,
	cases []stateCheckCase,
) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			entity := newEntity()
			entity.SetState(homeassistant.State{State: tc.state})

			assert.Equal(t, check(entity), tc.want)
		})
	}
}
