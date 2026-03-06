// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllClients_ReturnsSixClients(t *testing.T) {
	clients := GetAllClients()

	assert.Len(t, clients, 6)
}

func TestGetAllClients_ContainsExpectedClients(t *testing.T) {
	clients := GetAllClients()

	expectedIDs := []string{"claude-code", "claude-desktop", "cursor", "windsurf", "cline", "continue"}
	actualIDs := make([]string, len(clients))
	for i, c := range clients {
		actualIDs[i] = c.ID
	}

	for _, expected := range expectedIDs {
		assert.Contains(t, actualIDs, expected)
	}
}

func TestGetClient_ClaudeCode(t *testing.T) {
	client, err := GetClient("claude-code")

	require.NoError(t, err)
	assert.Equal(t, "Claude Code", client.Name)
	assert.Equal(t, "claude-code", client.ID)
	assert.Equal(t, "kusari-inspector", client.ServerKey)
	assert.Equal(t, InstallMethodFile, client.InstallMethod)
	assert.NotEmpty(t, client.ConfigPaths)
}

func TestGetClient_ClaudeLegacyAlias(t *testing.T) {
	// "claude" should be an alias for "claude-code"
	client, err := GetClient("claude")

	require.NoError(t, err)
	assert.Equal(t, "Claude Code", client.Name)
	assert.Equal(t, "claude-code", client.ID)
}

func TestGetClient_ClaudeDesktop(t *testing.T) {
	client, err := GetClient("claude-desktop")

	require.NoError(t, err)
	assert.Equal(t, "Claude Desktop", client.Name)
	assert.Equal(t, "claude-desktop", client.ID)
	assert.Equal(t, "kusari-inspector", client.ServerKey)
	assert.Equal(t, InstallMethodFile, client.InstallMethod)
	assert.NotEmpty(t, client.ConfigPaths)
}

func TestGetClient_Cursor(t *testing.T) {
	client, err := GetClient("cursor")

	require.NoError(t, err)
	assert.Equal(t, "Cursor", client.Name)
	assert.Equal(t, "cursor", client.ID)
	assert.Equal(t, "kusari-inspector", client.ServerKey)
	assert.Equal(t, ConfigFormatStandard, client.ConfigFormat)
}

func TestGetClient_Windsurf(t *testing.T) {
	client, err := GetClient("windsurf")

	require.NoError(t, err)
	assert.Equal(t, "Windsurf", client.Name)
	assert.Equal(t, "windsurf", client.ID)
	assert.Equal(t, ConfigFormatStandard, client.ConfigFormat)
}

func TestGetClient_Cline(t *testing.T) {
	client, err := GetClient("cline")

	require.NoError(t, err)
	assert.Equal(t, "Cline", client.Name)
	assert.Equal(t, "cline", client.ID)
	assert.Equal(t, ConfigFormatStandard, client.ConfigFormat)
}

func TestGetClient_Continue(t *testing.T) {
	client, err := GetClient("continue")

	require.NoError(t, err)
	assert.Equal(t, "Continue", client.Name)
	assert.Equal(t, "continue", client.ID)
	// Continue uses a different config format
	assert.Equal(t, ConfigFormatContinue, client.ConfigFormat)
}

func TestGetClient_UnknownClient(t *testing.T) {
	_, err := GetClient("unknown-client")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown client")
}

func TestGetClient_CaseInsensitive(t *testing.T) {
	// Should work with different cases
	client, err := GetClient("CLAUDE-CODE")

	require.NoError(t, err)
	assert.Equal(t, "Claude Code", client.Name)
}

func TestClientConfig_HasConfigPathsForAllPlatforms(t *testing.T) {
	clients := GetAllClients()
	platforms := []string{"darwin", "linux", "windows"}

	for _, client := range clients {
		for _, platform := range platforms {
			t.Run(client.ID+"_"+platform, func(t *testing.T) {
				path, ok := client.ConfigPaths[platform]
				assert.True(t, ok, "client %s should have config path for %s", client.ID, platform)
				assert.NotEmpty(t, path, "config path for %s on %s should not be empty", client.ID, platform)
			})
		}
	}
}

func TestClientConfig_ServerKeyIsKusariInspector(t *testing.T) {
	clients := GetAllClients()

	for _, client := range clients {
		assert.Equal(t, "kusari-inspector", client.ServerKey,
			"client %s should use 'kusari-inspector' as server key", client.ID)
	}
}

func TestConfigFormat_Constants(t *testing.T) {
	// Ensure config format constants are defined correctly
	assert.Equal(t, ConfigFormat(0), ConfigFormatStandard)
	assert.Equal(t, ConfigFormat(1), ConfigFormatContinue)
}
