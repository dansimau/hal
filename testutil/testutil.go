package testutil

import (
	"log/slog"
	"testing"
	"time"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/hassws"
	"gotest.tools/v3/assert"
)

const TestUserID = "d8e8fca2dc0f896fd7cb4cb0031ba249"

const (
	// inMemoryDatabasePath is the SQLite DSN for a throwaway in-memory database.
	inMemoryDatabasePath = ":memory:"

	// connectionSettleDelay is how long to wait after starting the connection
	// in the background for it to establish before returning to the test.
	connectionSettleDelay = 100 * time.Millisecond

	// fastReconnectInterval is the short reconnect interval used by tests that
	// exercise reconnection without waiting for the production default.
	fastReconnectInterval = 100 * time.Millisecond
)

func init() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
}

func NewClientServer(t *testing.T) (*hal.Connection, *hassws.Server, func()) {
	t.Helper()

	return NewClientServerWithConfig(t, hal.Config{
		DatabasePath: inMemoryDatabasePath,
	})
}

func NewClientServerWithConfig(t *testing.T, cfg hal.Config) (*hal.Connection, *hassws.Server, func()) {
	t.Helper()

	// Create test server with valid users map
	validUsers := map[string]string{
		"test-token": TestUserID,
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
		cfg.HomeAssistant.UserID = TestUserID
	}

	if cfg.DatabasePath == "" {
		cfg.DatabasePath = inMemoryDatabasePath
	}

	// Create client and connection
	conn := hal.NewConnection(cfg)

	// Create test entity and register it
	entity := hal.NewEntity("test.entity")
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

func NewFastReconnectClientServer(t *testing.T) (*hal.Connection, *hassws.Server, func()) {
	t.Helper()

	return NewClientServerWithConfig(t, hal.Config{
		DatabasePath:      inMemoryDatabasePath,
		ReconnectInterval: fastReconnectInterval,
	})
}
