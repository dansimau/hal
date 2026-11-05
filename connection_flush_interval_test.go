package hal

import (
	"testing"
	"time"

	"github.com/dansimau/hal/store"
	"gotest.tools/v3/assert"
)

func TestFlushIntervalFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "valid", value: "2s", want: 2 * time.Second},
		{name: "zero", value: "0", want: 0},
		{name: "invalid is ignored", value: "soon", want: store.DefaultFlushInterval},
		{name: "negative is ignored", value: "-1s", want: store.DefaultFlushInterval},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(FlushIntervalEnv, tc.value)

			conn := NewConnection(Config{DatabasePath: ":memory:"})
			defer conn.Close()

			assert.Equal(t, conn.db.FlushInterval(), tc.want)
		})
	}
}

func TestFlushIntervalDefault(t *testing.T) {
	t.Setenv(FlushIntervalEnv, "")

	conn := NewConnection(Config{DatabasePath: ":memory:"})
	defer conn.Close()

	assert.Equal(t, conn.db.FlushInterval(), store.DefaultFlushInterval)
}
