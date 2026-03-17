// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstall_CreatesConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "claude_desktop_config.json")

	client := ClientConfig{
		Name:         "Claude Code",
		ID:           "claude",
		ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
		ServerKey:    "kusari-inspector",
		ConfigFormat: ConfigFormatStandard,
	}

	result, err := Install(client)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Claude Code", result.ClientName)
	assert.Equal(t, configPath, result.ConfigPath)
	assert.True(t, result.NeedsRestart)

	// Verify file was created
	assert.FileExists(t, configPath)

	// Verify content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	require.NoError(t, err)

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	require.True(t, ok)
	_, ok = mcpServers["kusari-inspector"]
	assert.True(t, ok)
}

func TestInstall_UpdatesExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "claude_desktop_config.json")

	// Create existing config with another MCP server
	existingConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"other-server": map[string]interface{}{
				"command": "other-command",
			},
		},
	}
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	client := ClientConfig{
		Name:         "Claude Code",
		ID:           "claude",
		ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
		ServerKey:    "kusari-inspector",
		ConfigFormat: ConfigFormatStandard,
	}

	result, err := Install(client)

	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify both servers exist
	data, err = os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	require.NoError(t, err)

	mcpServers := config["mcpServers"].(map[string]interface{})
	assert.Contains(t, mcpServers, "kusari-inspector")
	assert.Contains(t, mcpServers, "other-server")
}

func TestInstall_CreatesParentDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nested", "dir", "config.json")

	client := ClientConfig{
		Name:         "Claude Code",
		ID:           "claude",
		ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
		ServerKey:    "kusari-inspector",
		ConfigFormat: ConfigFormatStandard,
	}

	result, err := Install(client)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.FileExists(t, configPath)
}

func TestInstall_UpdatesExistingKusariConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "claude_desktop_config.json")

	// Create existing config with kusari-inspector already present
	existingConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"kusari-inspector": map[string]interface{}{
				"command": "old-command",
			},
		},
	}
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	client := ClientConfig{
		Name:         "Claude Code",
		ID:           "claude",
		ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
		ServerKey:    "kusari-inspector",
		ConfigFormat: ConfigFormatStandard,
	}

	result, err := Install(client)

	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify config was updated (not duplicated)
	data, err = os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	require.NoError(t, err)

	mcpServers := config["mcpServers"].(map[string]interface{})
	kusari := mcpServers["kusari-inspector"].(map[string]interface{})
	// Should have new command, not old
	assert.NotEqual(t, "old-command", kusari["command"])
}

func TestInstallationResult_Fields(t *testing.T) {
	result := InstallationResult{
		Success:      true,
		ClientName:   "Claude Code",
		ConfigPath:   "/path/to/config.json",
		Message:      "Successfully installed",
		NeedsRestart: true,
	}

	assert.True(t, result.Success)
	assert.Equal(t, "Claude Code", result.ClientName)
	assert.Equal(t, "/path/to/config.json", result.ConfigPath)
	assert.Equal(t, "Successfully installed", result.Message)
	assert.True(t, result.NeedsRestart)
}

func TestUninstall_RemovesKusariConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config with kusari-inspector and another server
	existingConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"kusari-inspector": map[string]interface{}{
				"command": "kusari",
			},
			"other-server": map[string]interface{}{
				"command": "other",
			},
		},
	}
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	client := ClientConfig{
		Name:         "Claude Code",
		ID:           "claude",
		ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
		ServerKey:    "kusari-inspector",
		ConfigFormat: ConfigFormatStandard,
	}

	result, err := Uninstall(client)

	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify kusari-inspector removed but other-server preserved
	data, err = os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	require.NoError(t, err)

	mcpServers := config["mcpServers"].(map[string]interface{})
	assert.NotContains(t, mcpServers, "kusari-inspector")
	assert.Contains(t, mcpServers, "other-server")
}

func TestUninstall_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config without kusari-inspector
	existingConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"other-server": map[string]interface{}{
				"command": "other",
			},
		},
	}
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	client := ClientConfig{
		Name:         "Claude Code",
		ID:           "claude",
		ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
		ServerKey:    "kusari-inspector",
		ConfigFormat: ConfigFormatStandard,
	}

	result, err := Uninstall(client)

	// Should succeed (idempotent)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestGetServerConfig_ReturnsCorrectStructure(t *testing.T) {
	config := GetServerConfig()

	assert.Equal(t, "kusari", config["command"])
	args, ok := config["args"].([]string)
	require.True(t, ok)
	assert.Contains(t, args, "ai")
	assert.Contains(t, args, "serve")
}

// ListClients tests (T037)

func TestListClients_ReturnsAllClients(t *testing.T) {
	tmpDir := t.TempDir()

	// Create clients with temp paths
	clients := []ClientConfig{
		{
			Name:         "Client1",
			ID:           "client1",
			ConfigPaths:  map[string]string{"darwin": filepath.Join(tmpDir, "client1.json"), "linux": filepath.Join(tmpDir, "client1.json"), "windows": filepath.Join(tmpDir, "client1.json")},
			ServerKey:    "kusari-inspector",
			ConfigFormat: ConfigFormatStandard,
		},
		{
			Name:         "Client2",
			ID:           "client2",
			ConfigPaths:  map[string]string{"darwin": filepath.Join(tmpDir, "client2.json"), "linux": filepath.Join(tmpDir, "client2.json"), "windows": filepath.Join(tmpDir, "client2.json")},
			ServerKey:    "kusari-inspector",
			ConfigFormat: ConfigFormatStandard,
		},
	}

	results := ListClients(clients)

	assert.Len(t, results, 2)
	assert.Equal(t, "Client1", results[0].ClientName)
	assert.Equal(t, "Client2", results[1].ClientName)
}

func TestListClients_DetectsInstalledStatus(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config with kusari-inspector installed
	existingConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"kusari-inspector": map[string]interface{}{
				"command": "kusari",
			},
		},
	}
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	clients := []ClientConfig{
		{
			Name:         "Installed Client",
			ID:           "installed",
			ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
			ServerKey:    "kusari-inspector",
			ConfigFormat: ConfigFormatStandard,
		},
	}

	results := ListClients(clients)

	require.Len(t, results, 1)
	assert.True(t, results[0].Installed)
}

func TestListClients_DetectsNotInstalledStatus(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config WITHOUT kusari-inspector
	existingConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"other-server": map[string]interface{}{
				"command": "other",
			},
		},
	}
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	clients := []ClientConfig{
		{
			Name:         "Not Installed Client",
			ID:           "notinstalled",
			ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
			ServerKey:    "kusari-inspector",
			ConfigFormat: ConfigFormatStandard,
		},
	}

	results := ListClients(clients)

	require.Len(t, results, 1)
	assert.False(t, results[0].Installed)
}

func TestListClients_HandlesNonExistentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent", "config.json")

	clients := []ClientConfig{
		{
			Name:         "No Config Client",
			ID:           "noconfig",
			ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
			ServerKey:    "kusari-inspector",
			ConfigFormat: ConfigFormatStandard,
		},
	}

	results := ListClients(clients)

	require.Len(t, results, 1)
	assert.False(t, results[0].Installed)
}

// Continue config format tests (T038)

func TestInstall_ContinueFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	client := ClientConfig{
		Name:         "Continue",
		ID:           "continue",
		ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
		ServerKey:    "kusari-inspector",
		ConfigFormat: ConfigFormatContinue,
	}

	result, err := Install(client)

	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify Continue format (experimental.modelContextProtocolServers array)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	require.NoError(t, err)

	experimental, ok := config["experimental"].(map[string]interface{})
	require.True(t, ok, "expected experimental key")

	servers, ok := experimental["modelContextProtocolServers"].([]interface{})
	require.True(t, ok, "expected modelContextProtocolServers array")
	require.Len(t, servers, 1)

	server := servers[0].(map[string]interface{})
	assert.Equal(t, "kusari-inspector", server["name"])
	assert.Equal(t, "kusari", server["command"])
}

func TestUninstall_ContinueFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create Continue-format config with kusari-inspector and another server
	existingConfig := map[string]interface{}{
		"experimental": map[string]interface{}{
			"modelContextProtocolServers": []interface{}{
				map[string]interface{}{
					"name":    "kusari-inspector",
					"command": "kusari",
				},
				map[string]interface{}{
					"name":    "other-server",
					"command": "other",
				},
			},
		},
	}
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	client := ClientConfig{
		Name:         "Continue",
		ID:           "continue",
		ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
		ServerKey:    "kusari-inspector",
		ConfigFormat: ConfigFormatContinue,
	}

	result, err := Uninstall(client)

	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify kusari-inspector removed but other-server preserved
	data, err = os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	require.NoError(t, err)

	experimental := config["experimental"].(map[string]interface{})
	servers := experimental["modelContextProtocolServers"].([]interface{})

	assert.Len(t, servers, 1)
	server := servers[0].(map[string]interface{})
	assert.Equal(t, "other-server", server["name"])
}

func TestListClients_ContinueFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create Continue-format config with kusari-inspector installed
	existingConfig := map[string]interface{}{
		"experimental": map[string]interface{}{
			"modelContextProtocolServers": []interface{}{
				map[string]interface{}{
					"name":    "kusari-inspector",
					"command": "kusari",
				},
			},
		},
	}
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	clients := []ClientConfig{
		{
			Name:         "Continue",
			ID:           "continue",
			ConfigPaths:  map[string]string{"darwin": configPath, "linux": configPath, "windows": configPath},
			ServerKey:    "kusari-inspector",
			ConfigFormat: ConfigFormatContinue,
		},
	}

	results := ListClients(clients)

	require.Len(t, results, 1)
	assert.True(t, results[0].Installed)
}
