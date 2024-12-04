package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
)

// ConfigStore manages reading and watching for changes to a config file
type ConfigStore struct {
	path string
}

func NewConfigStore(path string) *ConfigStore {
	return &ConfigStore{path: path}
}

// Read loads and validates the config file
func (c ConfigStore) Read() (Config, error) {
	var cfg Config
	data, err := ioutil.ReadFile(c.path)
	if err != nil {
		return cfg, fmt.Errorf("error reading config file: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("error loading config: %w", err)
	}

	if err := validateConfig(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func validateConfig(cfg Config) error {
	ports := map[int]struct{}{}

	for _, app := range cfg.Apps {
		for _, port := range app.Ports {
			if _, ok := ports[port]; ok {
				return errors.New("invalid configuration - duplicate ports")
			}
			ports[port] = struct{}{}
		}
	}
	return nil
}

type Config struct {
	Apps []App
}

type App struct {
	Name    string
	Ports   []int
	Targets []string
}
