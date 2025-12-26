package hal

import (
	"testing"
	"time"

	"github.com/dansimau/hal/hassws"
	"gotest.tools/v3/assert"
)

const (
	testUserID   = "d8e8fca2dc0f896fd7cb4cb0031ba249"
	pollInterval = 100 * time.Millisecond
	waitTimeout  = 3 * time.Second
)

// waitFor waits for the given function to return true.
func waitFor(t *testing.T, name string, callbackFn func() bool, onFailed func()) {
	t.Helper()

	timeout := time.After(waitTimeout)

	for {
		select {
		case <-timeout:
			t.Errorf("assertion failed: %s", name)
			onFailed()
			t.FailNow()
		default:
			if callbackFn() {
				return
			}

			time.Sleep(pollInterval)
		}
	}
}

// newFastReconnectClientServer creates a test connection with fast reconnection interval (100ms).
func newFastReconnectClientServer(t *testing.T) (*Connection, *hassws.Server, func()) {
	return newClientServerWithConfig(t, Config{
		DatabasePath:      ":memory:",
		ReconnectInterval: 100 * time.Millisecond,
	})
}

// newClientServerWithConfig creates a test connection with custom configuration.
func newClientServerWithConfig(t *testing.T, cfg Config) (*Connection, *hassws.Server, func()) {
	t.Helper()

	// Create test server with valid users map
	validUsers := map[string]string{
		"test-token": testUserID,
	}
	server, err := hassws.NewServer(validUsers)
	assert.NilError(t, err)

	// Apply server address and test defaults to config
	if cfg.HomeAssistant.Host == "" {
		cfg.HomeAssistant.Host = server.ListenAddress()
	}
	if cfg.HomeAssistant.Token == "" {
		cfg.HomeAssistant.Token = "test-token"
	}
	if cfg.HomeAssistant.UserID == "" {
		cfg.HomeAssistant.UserID = testUserID
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = ":memory:"
	}

	// Create client and connection
	conn := NewConnection(cfg)

	// Create test entity and register it
	entity := NewEntity("test.entity")
	conn.RegisterEntities(entity)

	// Start connection in background (Start is now blocking)
	go func() {
		if err := conn.Start(); err != nil {
			t.Errorf("Start() failed: %v", err)
		}
	}()

	// Give connection time to establish
	time.Sleep(100 * time.Millisecond)

	return conn, server, func() {
		conn.Close()
		server.Close()
	}
}
