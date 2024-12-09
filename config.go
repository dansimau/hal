package hal

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const configFilename = "hal.yaml"

type Config struct {
	HomeAssistant HomeAssistantConfig `yaml:"homeAssistant"`
	Location      LocationConfig      `yaml:"location"`
}

type HomeAssistantConfig struct {
	Host  string `yaml:"host"`
	Token string `yaml:"token"`
}

type LocationConfig struct {
	Latitude  float64 `yaml:"lat"`
	Longitude float64 `yaml:"lng"`
}

func LoadConfig() (*Config, error) {
	configPath, err := searchParentsForFileFromCwd(configFilename)
	if err != nil {
		return nil, err
	}

	yamlBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := Config{}
	if err := yaml.Unmarshal(yamlBytes, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func searchParentsForFile(filename, searchPath string) (path string, err error) {
	for _, path := range getParents(searchPath) {
		f := filepath.Join(path, filename)
		if fileExists(f) {
			return f, nil
		}
	}

	return "", nil
}

func searchParentsForFileFromCwd(filename string) (path string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return searchParentsForFile(filename, wd)
}

func getParents(basePath string) (paths []string) {
	root := basePath

	for root != "/" {
		paths = append(paths, root)
		root = filepath.Dir(root)
	}

	paths = append(paths, "/")

	return paths
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return !os.IsNotExist(err)
}
