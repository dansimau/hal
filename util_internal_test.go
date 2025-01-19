package hal

import (
	"testing"
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
