package hassws

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestNewClientDefaults(t *testing.T) {
	c := NewClient(ClientConfig{})

	assert.Equal(t, defaultPingInterval, c.cfg.PingInterval)
	assert.Equal(t, defaultPongTimeout, c.cfg.PongTimeout)
}

func TestNewClientAdjustsPongTimeoutBelowPingInterval(t *testing.T) {
	// PongTimeout <= PingInterval would cause a reconnect loop on a
	// quiet-but-healthy connection, so it should be bumped above the ping
	// interval.
	c := NewClient(ClientConfig{
		PingInterval: 30 * time.Second,
		PongTimeout:  10 * time.Second,
	})

	assert.Assert(t, c.cfg.PongTimeout > c.cfg.PingInterval)
	assert.Equal(t, 60*time.Second, c.cfg.PongTimeout)
}

func TestNewClientKeepsValidPongTimeout(t *testing.T) {
	c := NewClient(ClientConfig{
		PingInterval: 5 * time.Second,
		PongTimeout:  20 * time.Second,
	})

	assert.Equal(t, 5*time.Second, c.cfg.PingInterval)
	assert.Equal(t, 20*time.Second, c.cfg.PongTimeout)
}
