package hal

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetStringOrStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected []string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "string input",
			input:    "test",
			expected: []string{"test"},
		},
		{
			name:     "string slice input",
			input:    []string{"test"},
			expected: []string{"test"},
		},
		{
			name:     "empty string input",
			input:    "",
			expected: []string{""},
		},
		{
			name:     "slice of strings",
			input:    []any{"test1", "test2", "test3"},
			expected: []string{"test1", "test2", "test3"},
		},
		{
			name:     "empty slice",
			input:    []any{},
			expected: []string{},
		},
		{
			name:     "slice with non-string values",
			input:    []any{1, "test", true},
			expected: []string{"test"},
		},
		{
			name:     "unsupported type",
			input:    123,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getStringOrStringSlice(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("got len %d, want len %d", len(result), len(tt.expected))
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("got %s, want %s at index %d", result[i], tt.expected[i], i)
				}
			}
		})
	}
}

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
