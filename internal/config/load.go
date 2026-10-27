package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed config.default.yaml
var defaultConfigYAML []byte

// Load reads configuration from the given path, falling back to embedded defaults.
func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "config.yaml not found, using embedded default config")
			return loadEmbedded()
		}
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// loadEmbedded parses and validates the embedded default configuration.
func loadEmbedded() (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(defaultConfigYAML, cfg); err != nil {
		return nil, fmt.Errorf("parse embedded default config: %w", err)
	}
	return cfg, nil
}

// LoadFromBytes parses config from a byte slice (for testing).
func LoadFromBytes(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

// FindConfigPath walks parent directories looking for config.yaml.
func FindConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(dir, "config.yaml"),
		"config.yaml",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}