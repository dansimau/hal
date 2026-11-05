package hal

import (
	"testing"
	"time"

	"github.com/dansimau/hal/hassws"
	"gotest.tools/v3/assert"
)

const (
	testUserID         = "d8e8fca2dc0f896fd7cb4cb0031ba249"
	testToken          = "test-token"
	testEntityID       = "test.entity"
	memoryDatabasePath = ":memory:"
	pollInterval       = 100 * time.Millisecond
	waitTimeout        = 3 * time.Second

	// fastReconnectInterval is the short reconnect interval used so tests
	// observing reconnection do not wait for the production default.
	fastReconnectInterval = 100 * time.Millisecond

	// connectionSettleDelay is how long newClientServerWithConfig waits for the
	// background connection to establish before returning.
	connectionSettleDelay = 100 * time.Millisecond
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
	t.Helper()

	return newClientServerWithConfig(t, Config{
		DatabasePath:      memoryDatabasePath,
		ReconnectInterval: fastReconnectInterval,
	})
}

// newClientServerWithConfig creates a test connection with custom configuration.
func newClientServerWithConfig(t *testing.T, cfg Config) (*Connection, *hassws.Server, func()) {
	t.Helper()

	// Create test server with valid users map
	validUsers := map[string]string{
		testToken: testUserID,
	}
	server, err := hassws.NewServer(validUsers)
	assert.NilError(t, err)

	// Apply server address and test defaults to config
	if cfg.HomeAssistant.Host == "" {
		cfg.HomeAssistant.Host = server.ListenAddress()
	}

	if cfg.HomeAssistant.Token == "" {
		cfg.HomeAssistant.Token = testToken
	}

	if cfg.HomeAssistant.UserID == "" {
		cfg.HomeAssistant.UserID = testUserID
	}

	if cfg.DatabasePath == "" {
		cfg.DatabasePath = memoryDatabasePath
	}

	// Create client and connection
	conn := NewConnection(cfg)

	// Create test entity and register it
	entity := NewEntity(testEntityID)
	conn.RegisterEntities(entity)

	// Start connection in background (Start is now blocking)
	go func() {
		err := conn.Start()
		if err != nil {
			t.Errorf("Start() failed: %v", err)
		}
	}()

	// Give connection time to establish
	time.Sleep(connectionSettleDelay)

	return conn, server, func() {
		conn.Close()
		server.Close()
	}
}
