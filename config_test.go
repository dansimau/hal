package hal

import (
	"os"
	"path/filepath"
	"testing"

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
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		assert.NilError(t, err)

		// Change to temp directory
		err = os.Chdir(tmpDir)
		assert.NilError(t, err)

		config, err := LoadConfig()
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

		_, err = LoadConfig()
		assert.ErrorContains(t, err, "no such file or directory")
	})

	t.Run("returns error for invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)

		invalidContent := `invalid: yaml: content:`
		configPath := filepath.Join(tmpDir, "hal.yaml")
		err := os.WriteFile(configPath, []byte(invalidContent), 0644)
		assert.NilError(t, err)

		err = os.Chdir(tmpDir)
		assert.NilError(t, err)

		_, err = LoadConfig()
		assert.ErrorContains(t, err, "yaml")
	})
}

func TestSearchParentsForFile(t *testing.T) {
	t.Run("finds file in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := "test.txt"
		testPath := filepath.Join(tmpDir, testFile)

		err := os.WriteFile(testPath, []byte("test"), 0644)
		assert.NilError(t, err)

		foundPath, err := searchParentsForFile(testFile, tmpDir)
		assert.NilError(t, err)
		assert.Equal(t, foundPath, testPath)
	})

	t.Run("finds file in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir")
		err := os.Mkdir(subDir, 0755)
		assert.NilError(t, err)

		testFile := "test.txt"
		testPath := filepath.Join(tmpDir, testFile)

		err = os.WriteFile(testPath, []byte("test"), 0644)
		assert.NilError(t, err)

		foundPath, err := searchParentsForFile(testFile, subDir)
		assert.NilError(t, err)
		assert.Equal(t, foundPath, testPath)
	})

	t.Run("returns empty string when file not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		foundPath, err := searchParentsForFile("nonexistent.txt", tmpDir)
		assert.NilError(t, err)
		assert.Equal(t, foundPath, "")
	})
}

func TestSearchParentsForFileFromCwd(t *testing.T) {
	t.Run("searches from current working directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)

		testFile := "test.txt"
		testPath := filepath.Join(tmpDir, testFile)

		err := os.WriteFile(testPath, []byte("test"), 0644)
		assert.NilError(t, err)

		err = os.Chdir(tmpDir)
		assert.NilError(t, err)

		foundPath, err := searchParentsForFileFromCwd(testFile)
		assert.NilError(t, err)
		
		// Resolve symlinks to handle macOS /private/var vs /var differences
		resolvedFound, err := filepath.EvalSymlinks(foundPath)
		assert.NilError(t, err)
		resolvedExpected, err := filepath.EvalSymlinks(testPath)
		assert.NilError(t, err)
		
		assert.Equal(t, resolvedFound, resolvedExpected)
	})
}

func TestGetParents(t *testing.T) {
	t.Run("returns correct parent paths", func(t *testing.T) {
		if filepath.Separator == '\\' {
			// Windows paths
			paths := getParents("C:\\Users\\test\\project")
			expected := []string{
				"C:\\Users\\test\\project",
				"C:\\Users\\test",
				"C:\\Users",
				"C:\\",
				"/",
			}
			assert.DeepEqual(t, paths, expected)
		} else {
			// Unix paths
			paths := getParents("/home/user/project")
			expected := []string{
				"/home/user/project",
				"/home/user",
				"/home",
				"/",
			}
			assert.DeepEqual(t, paths, expected)
		}
	})

	t.Run("handles root directory", func(t *testing.T) {
		paths := getParents("/")
		expected := []string{"/"}
		assert.DeepEqual(t, paths, expected)
	})
}

func TestFileExists(t *testing.T) {
	t.Run("returns true for existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "test.txt")

		err := os.WriteFile(testPath, []byte("test"), 0644)
		assert.NilError(t, err)

		exists := fileExists(testPath)
		assert.Equal(t, exists, true)
	})

	t.Run("returns false for non-existent file", func(t *testing.T) {
		exists := fileExists("/nonexistent/path/file.txt")
		assert.Equal(t, exists, false)
	})

	t.Run("returns true for existing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		exists := fileExists(tmpDir)
		assert.Equal(t, exists, true)
	})
}