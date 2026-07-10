package hassws

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestNewClientReadTimeoutDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		pingInterval     time.Duration
		readTimeout      time.Duration
		wantPingInterval time.Duration
		wantReadTimeout  time.Duration
	}{
		{
			name:             "defaults applied when unset",
			wantPingInterval: defaultPingInterval,
			wantReadTimeout:  2 * defaultPingInterval,
		},
		{
			name:             "read timeout derived from custom ping interval",
			pingInterval:     90 * time.Second,
			wantPingInterval: 90 * time.Second,
			wantReadTimeout:  180 * time.Second,
		},
		{
			name:             "explicit read timeout above ping interval is kept",
			pingInterval:     30 * time.Second,
			readTimeout:      45 * time.Second,
			wantPingInterval: 30 * time.Second,
			wantReadTimeout:  45 * time.Second,
		},
		{
			name:             "read timeout not larger than ping interval is bumped",
			pingInterval:     30 * time.Second,
			readTimeout:      20 * time.Second,
			wantPingInterval: 30 * time.Second,
			wantReadTimeout:  60 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := NewClient(ClientConfig{
				PingInterval: tc.pingInterval,
				ReadTimeout:  tc.readTimeout,
			})

			assert.Equal(t, tc.wantPingInterval, c.cfg.PingInterval)
			assert.Equal(t, tc.wantReadTimeout, c.cfg.ReadTimeout)
		})
	}
}
