package hal_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dansimau/hal"
	"gotest.tools/v3/assert"
)

func TestLoadConfig(t *testing.T) {
	t.Run("loads valid config", func(t *testing.T) {
		// Create a temporary directory and config file
		tmpDir := t.TempDir()
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)

		configContent := `homeAssistant:
  host: "localhost:8123"
  token: "test-token"
  userId: "test-user"
location:
  lat: 37.7749
  lng: -122.4194`

		configPath := filepath.Join(tmpDir, "hal.yaml")
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		assert.NilError(t, err)

		// Change to temp directory
		err = os.Chdir(tmpDir)
		assert.NilError(t, err)

		config, err := hal.LoadConfig()
		assert.NilError(t, err)
		assert.Equal(t, config.HomeAssistant.Host, "localhost:8123")
		assert.Equal(t, config.HomeAssistant.Token, "test-token")
		assert.Equal(t, config.HomeAssistant.UserID, "test-user")
		assert.Equal(t, config.Location.Latitude, 37.7749)
		assert.Equal(t, config.Location.Longitude, -122.4194)
	})

	t.Run("returns error when config file not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)

		err := os.Chdir(tmpDir)
		assert.NilError(t, err)

		_, err = hal.LoadConfig()
		assert.ErrorContains(t, err, "no such file or directory")
	})

	t.Run("returns error for invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)

		invalidContent := `invalid: yaml: content:`
		configPath := filepath.Join(tmpDir, "hal.yaml")
		err := os.WriteFile(configPath, []byte(invalidContent), 0o644)
		assert.NilError(t, err)

		err = os.Chdir(tmpDir)
		assert.NilError(t, err)

		_, err = hal.LoadConfig()
		assert.ErrorContains(t, err, "yaml")
	})
}
