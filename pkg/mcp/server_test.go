// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer_ReturnsServer(t *testing.T) {
	cfg := NewConfig()

	server, err := NewServer(cfg)

	require.NoError(t, err)
	assert.NotNil(t, server)
}

func TestNewServer_WithNilConfig(t *testing.T) {
	server, err := NewServer(nil)

	// Should use default config
	require.NoError(t, err)
	assert.NotNil(t, server)
}

func TestServer_HasScanQueue(t *testing.T) {
	cfg := NewConfig()
	server, err := NewServer(cfg)
	require.NoError(t, err)

	// Server should have a scan queue
	assert.NotNil(t, server.scanQueue)
}

func TestServer_RegistersTools(t *testing.T) {
	cfg := NewConfig()
	server, err := NewServer(cfg)
	require.NoError(t, err)

	tools := server.GetRegisteredTools()

	// Should have at least scan_local_changes tool
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "scan_local_changes")
}

func TestServer_Config(t *testing.T) {
	cfg := &Config{
		ConsoleURL:  "https://custom.example.com/",
		PlatformURL: "https://custom-api.example.com/",
		Verbose:     true,
	}

	server, err := NewServer(cfg)
	require.NoError(t, err)

	assert.Equal(t, "https://custom.example.com/", server.config.ConsoleURL)
	assert.Equal(t, "https://custom-api.example.com/", server.config.PlatformURL)
	assert.True(t, server.config.Verbose)
}

func TestToolDefinition_ScanLocalChanges(t *testing.T) {
	cfg := NewConfig()
	server, err := NewServer(cfg)
	require.NoError(t, err)

	tools := server.GetRegisteredTools()

	var scanTool *ToolDefinition
	for _, tool := range tools {
		if tool.Name == "scan_local_changes" {
			scanTool = &tool
			break
		}
	}

	require.NotNil(t, scanTool)
	assert.Equal(t, "scan_local_changes", scanTool.Name)
	assert.NotEmpty(t, scanTool.Description)
	assert.NotNil(t, scanTool.InputSchema)
}
