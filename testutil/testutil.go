package testutil

import (
	"log/slog"
	"testing"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/hassws"
	"gotest.tools/v3/assert"
)

func init() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
}

func NewClientServer(t *testing.T) (*hal.Connection, *hassws.Server, func()) {
	t.Helper()

	// Create test server
	server, err := hassws.NewServer()
	assert.NilError(t, err)

	// Create client and connection
	conn := hal.NewConnection(hal.Config{
		HomeAssistant: hal.HomeAssistantConfig{
			Host:  server.ListenAddress(),
			Token: "test-token",
		},
	})

	// Create test entity and register it
	entity := hal.NewEntity("test.entity")
	conn.RegisterEntities(entity)

	// Start connection
	err = conn.Start()
	assert.NilError(t, err)

	return conn, server, func() {
		conn.Close()
		server.Close()
	}
}
