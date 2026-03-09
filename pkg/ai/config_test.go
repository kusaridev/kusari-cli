// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package ai

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := NewConfig()

	assert.Equal(t, "https://console.us.kusari.cloud/", cfg.ConsoleURL)
	assert.Equal(t, "https://platform.api.us.kusari.cloud/", cfg.PlatformURL)
	assert.False(t, cfg.Verbose)
}

func TestConfig_FromEnvironment(t *testing.T) {
	// Set environment variables
	t.Setenv("KUSARI_CONSOLE_URL", "https://custom-console.example.com/")
	t.Setenv("KUSARI_PLATFORM_URL", "https://custom-platform.example.com/")
	t.Setenv("KUSARI_VERBOSE", "true")

	cfg, err := LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, "https://custom-console.example.com/", cfg.ConsoleURL)
	assert.Equal(t, "https://custom-platform.example.com/", cfg.PlatformURL)
	assert.True(t, cfg.Verbose)
}

func TestConfig_FromEnvironment_PartialOverride(t *testing.T) {
	// Only set one environment variable
	t.Setenv("KUSARI_CONSOLE_URL", "https://custom-console.example.com/")

	cfg, err := LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, "https://custom-console.example.com/", cfg.ConsoleURL)
	// Should use default for platform URL
	assert.Equal(t, "https://platform.api.us.kusari.cloud/", cfg.PlatformURL)
}

func TestConfig_FromConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp-config.yaml")

	configContent := `console_url: https://file-console.example.com/
platform_url: https://file-platform.example.com/
verbose: true
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfigFromFile(configPath)

	require.NoError(t, err)
	assert.Equal(t, "https://file-console.example.com/", cfg.ConsoleURL)
	assert.Equal(t, "https://file-platform.example.com/", cfg.PlatformURL)
	assert.True(t, cfg.Verbose)
}

func TestConfig_EnvironmentOverridesFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp-config.yaml")

	configContent := `console_url: https://file-console.example.com/
platform_url: https://file-platform.example.com/
verbose: false
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Environment should override file
	t.Setenv("KUSARI_CONSOLE_URL", "https://env-console.example.com/")

	cfg, err := LoadConfigWithFile(configPath)

	require.NoError(t, err)
	// Env var should win
	assert.Equal(t, "https://env-console.example.com/", cfg.ConsoleURL)
	// File value should be used
	assert.Equal(t, "https://file-platform.example.com/", cfg.PlatformURL)
}

func TestConfig_MissingConfigFile_UsesDefaults(t *testing.T) {
	cfg, err := LoadConfigFromFile("/nonexistent/path/config.yaml")

	// Should not error, just use defaults
	require.NoError(t, err)
	assert.Equal(t, "https://console.us.kusari.cloud/", cfg.ConsoleURL)
}

func TestConfig_VerboseFromEnv_Various(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true lowercase", "true", true},
		{"TRUE uppercase", "TRUE", true},
		{"1", "1", true},
		{"false lowercase", "false", false},
		{"FALSE uppercase", "FALSE", false},
		{"0", "0", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("KUSARI_VERBOSE", tt.envValue)
			}
			cfg, err := LoadConfig()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.Verbose)
		})
	}
}
