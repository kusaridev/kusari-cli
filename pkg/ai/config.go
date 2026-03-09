// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package ai

import (
	"os"
	"strconv"
	"strings"

	"github.com/kusaridev/kusari-cli/pkg/constants"
	"gopkg.in/yaml.v3"
)

// Config holds the MCP server configuration.
type Config struct {
	ConsoleURL  string `yaml:"console_url"`
	PlatformURL string `yaml:"platform_url"`
	Verbose     bool   `yaml:"verbose"`
}

// NewConfig returns a Config with default values.
func NewConfig() *Config {
	return &Config{
		ConsoleURL:  constants.DefaultConsoleURL,
		PlatformURL: constants.DefaultPlatformURL,
		Verbose:     false,
	}
}

// LoadConfig loads configuration from environment variables with defaults.
func LoadConfig() (*Config, error) {
	cfg := NewConfig()

	if val := os.Getenv("KUSARI_CONSOLE_URL"); val != "" {
		cfg.ConsoleURL = val
	}
	if val := os.Getenv("KUSARI_PLATFORM_URL"); val != "" {
		cfg.PlatformURL = val
	}
	if val := os.Getenv("KUSARI_VERBOSE"); val != "" {
		cfg.Verbose = parseBool(val)
	}

	return cfg, nil
}

// LoadConfigFromFile loads configuration from a YAML file.
// If the file doesn't exist, returns default configuration without error.
func LoadConfigFromFile(path string) (*Config, error) {
	cfg := NewConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadConfigWithFile loads configuration from a file, then overlays environment variables.
// Environment variables take precedence over file values.
func LoadConfigWithFile(path string) (*Config, error) {
	cfg, err := LoadConfigFromFile(path)
	if err != nil {
		return nil, err
	}

	// Environment variables override file values
	if val := os.Getenv("KUSARI_CONSOLE_URL"); val != "" {
		cfg.ConsoleURL = val
	}
	if val := os.Getenv("KUSARI_PLATFORM_URL"); val != "" {
		cfg.PlatformURL = val
	}
	if val := os.Getenv("KUSARI_VERBOSE"); val != "" {
		cfg.Verbose = parseBool(val)
	}

	return cfg, nil
}

// parseBool parses a string to boolean, accepting various common formats.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	return false
}
