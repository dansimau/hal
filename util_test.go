package hal

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetShortFunctionName(t *testing.T) {
	t.Run("extracts function name from function reference", func(t *testing.T) {
		testFunc := func() {}
		name := getShortFunctionName(testFunc)

		// The name should contain "TestGetShortFunctionName.func1" or similar
		// depending on Go version and compilation
		assert.Assert(t, len(name) > 0)
		assert.Assert(t, name != "")
	})

	t.Run("handles named function", func(t *testing.T) {
		name := getShortFunctionName(getShortFunctionName)
		assert.Equal(t, name, "getShortFunctionName")
	})

	t.Run("handles method", func(t *testing.T) {
		light := NewLight("test")
		name := getShortFunctionName(light.TurnOn)
		// Method names may have "-fm" suffix in Go runtime
		assert.Assert(t, name == "TurnOn" || name == "TurnOn-fm")
	})
}

func TestGetStringOrStringSliceAdditional(t *testing.T) {
	t.Run("handles boolean input", func(t *testing.T) {
		result := getStringOrStringSlice(true)
		assert.DeepEqual(t, result, []string{})
	})

	t.Run("handles struct input", func(t *testing.T) {
		type testStruct struct {
			field string
		}
		result := getStringOrStringSlice(testStruct{field: "test"})
		assert.DeepEqual(t, result, []string{})
	})
}
