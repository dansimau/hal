package commands

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "days", input: "2d", want: 48 * time.Hour},
		{name: "single day", input: "1d", want: 24 * time.Hour},
		{name: "hours", input: "3h", want: 3 * time.Hour},
		{name: "minutes", input: "5m", want: 5 * time.Minute},
		{name: "seconds", input: "30s", want: 30 * time.Second},
		{name: "invalid days", input: "xd", wantErr: true},
		{name: "invalid format", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseDuration(tt.input)
			if tt.wantErr {
				assert.Assert(t, err != nil)

				return
			}

			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestParseTime(t *testing.T) {
	t.Parallel()

	t.Run("parses date only", func(t *testing.T) {
		t.Parallel()

		got, err := parseTime("2024-01-15")
		assert.NilError(t, err)
		assert.Equal(t, got, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))
	})

	t.Run("parses date and time", func(t *testing.T) {
		t.Parallel()

		got, err := parseTime("2024-01-15 13:45")
		assert.NilError(t, err)
		assert.Equal(t, got, time.Date(2024, 1, 15, 13, 45, 0, 0, time.UTC))
	})

	t.Run("parses date and time with seconds", func(t *testing.T) {
		t.Parallel()

		got, err := parseTime("2024-01-15 13:45:30")
		assert.NilError(t, err)
		assert.Equal(t, got, time.Date(2024, 1, 15, 13, 45, 30, 0, time.UTC))
	})

	t.Run("returns error for invalid input", func(t *testing.T) {
		t.Parallel()

		_, err := parseTime("not-a-date")
		assert.Assert(t, err != nil)
	})
}
