// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPlatform_ReturnsCurrentOS(t *testing.T) {
	platform := GetPlatform()

	// Should match runtime.GOOS
	assert.Equal(t, Platform(runtime.GOOS), platform)
}

func TestPlatform_Constants(t *testing.T) {
	assert.Equal(t, Platform("darwin"), PlatformDarwin)
	assert.Equal(t, Platform("linux"), PlatformLinux)
	assert.Equal(t, Platform("windows"), PlatformWindows)
}

func TestExpandConfigPath_Darwin(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde expansion",
			input:    "~/Library/Application Support/Claude/claude_desktop_config.json",
			expected: filepath.Join(homeDir, "Library/Application Support/Claude/claude_desktop_config.json"),
		},
		{
			name:     "no tilde",
			input:    "/absolute/path/config.json",
			expected: "/absolute/path/config.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandConfigPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandConfigPath_WithHomeEnvVar(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	// Test $HOME expansion
	result := ExpandConfigPath("$HOME/.config/test.json")
	expected := filepath.Join(homeDir, ".config/test.json")
	assert.Equal(t, expected, result)
}

func TestGetConfigPath_ForClient(t *testing.T) {
	// Use a file-based client (not CLI-based)
	client, err := GetClient("claude-desktop")
	require.NoError(t, err)

	path, err := GetConfigPath(client)

	require.NoError(t, err)
	assert.NotEmpty(t, path)

	// Path should be expanded (no ~ or $HOME)
	assert.NotContains(t, path, "~")
	assert.NotContains(t, path, "$HOME")
}

func TestGetConfigPath_ClaudeCode(t *testing.T) {
	client, err := GetClient("claude-code")
	require.NoError(t, err)

	path, err := GetConfigPath(client)

	require.NoError(t, err)
	assert.NotEmpty(t, path)
	// Claude Code uses ~/.claude.json
	assert.Contains(t, path, ".claude.json")
}

func TestGetConfigPath_UnsupportedPlatform(t *testing.T) {
	// Create a client with no path for current platform
	client := ClientConfig{
		Name:        "Test",
		ID:          "test",
		ConfigPaths: map[string]string{}, // Empty paths
	}

	_, err := GetConfigPath(client)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestIsPlatformSupported_Darwin(t *testing.T) {
	assert.True(t, IsPlatformSupported(PlatformDarwin))
}

func TestIsPlatformSupported_Linux(t *testing.T) {
	assert.True(t, IsPlatformSupported(PlatformLinux))
}

func TestIsPlatformSupported_Windows(t *testing.T) {
	assert.True(t, IsPlatformSupported(PlatformWindows))
}

func TestIsPlatformSupported_Unknown(t *testing.T) {
	assert.False(t, IsPlatformSupported(Platform("freebsd")))
}

func TestGetConfigPath_ClaudeDesktop_CorrectFormat(t *testing.T) {
	client, err := GetClient("claude-desktop")
	require.NoError(t, err)

	path, err := GetConfigPath(client)
	require.NoError(t, err)

	// Should end with claude_desktop_config.json
	assert.True(t, strings.HasSuffix(path, ".json"),
		"Claude Desktop config path should end with .json, got: %s", path)
}

func TestGetConfigPath_Cursor_CorrectFormat(t *testing.T) {
	client, err := GetClient("cursor")
	require.NoError(t, err)

	path, err := GetConfigPath(client)
	require.NoError(t, err)

	// Cursor uses mcp.json
	assert.True(t, strings.HasSuffix(path, ".json"),
		"Cursor config path should end with .json, got: %s", path)
}

func TestExpandConfigPath_WindowsAppData(t *testing.T) {
	// Test APPDATA expansion for Windows paths
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			result := ExpandConfigPath("%APPDATA%\\Claude\\config.json")
			assert.Contains(t, result, appData)
		}
	}
}
