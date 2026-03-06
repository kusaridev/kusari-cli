// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"fmt"
	"strings"
)

// ConfigFormat specifies the configuration file format for a client.
type ConfigFormat int

const (
	// ConfigFormatStandard uses the mcpServers object format.
	ConfigFormatStandard ConfigFormat = iota
	// ConfigFormatContinue uses the experimental.modelContextProtocolServers array format.
	ConfigFormatContinue
)

// InstallMethod specifies how to install the MCP server for a client.
type InstallMethod int

const (
	// InstallMethodFile installs by writing to a config file.
	InstallMethodFile InstallMethod = iota
	// InstallMethodCLI installs using the client's CLI command.
	InstallMethodCLI
)

// ClientConfig holds configuration for a supported MCP client.
type ClientConfig struct {
	// Name is the human-readable client name (e.g., "Claude Code")
	Name string
	// ID is the CLI identifier (e.g., "claude-code")
	ID string
	// ConfigPaths maps platform to config file path (for file-based install)
	ConfigPaths map[string]string
	// ServerKey is the key used in the mcpServers object
	ServerKey string
	// ConfigFormat specifies the config file format
	ConfigFormat ConfigFormat
	// InstallMethod specifies how to install (file or CLI)
	InstallMethod InstallMethod
	// CLICommand is the command to use for CLI-based install (e.g., "claude")
	CLICommand string
}

// supportedClients contains all supported MCP clients.
// Config paths verified from official documentation:
// - Claude Code: https://code.claude.com/docs/en/settings
// - Claude Desktop: https://modelcontextprotocol.io/docs/develop/connect-local-servers
// - Cursor: https://cursor.com/docs/context/mcp
// - Windsurf: https://docs.windsurf.com/windsurf/cascade/mcp
// - Cline: https://docs.cline.bot/mcp/configuring-mcp-servers
// - Continue: https://docs.continue.dev/customize/deep-dives/mcp
var supportedClients = []ClientConfig{
	{
		Name: "Claude Code",
		ID:   "claude-code",
		ConfigPaths: map[string]string{
			// Claude Code uses ~/.claude.json for MCP servers (NOT ~/.claude/settings.json)
			"darwin":  "~/.claude.json",
			"linux":   "~/.claude.json",
			"windows": "%USERPROFILE%\\.claude.json",
		},
		ServerKey:     "kusari-inspector",
		ConfigFormat:  ConfigFormatStandard,
		InstallMethod: InstallMethodFile,
	},
	{
		Name: "Claude Desktop",
		ID:   "claude-desktop",
		ConfigPaths: map[string]string{
			"darwin":  "~/Library/Application Support/Claude/claude_desktop_config.json",
			"linux":   "~/.config/Claude/claude_desktop_config.json",
			"windows": "%APPDATA%\\Claude\\claude_desktop_config.json",
		},
		ServerKey:     "kusari-inspector",
		ConfigFormat:  ConfigFormatStandard,
		InstallMethod: InstallMethodFile,
	},
	{
		Name: "Cursor",
		ID:   "cursor",
		ConfigPaths: map[string]string{
			"darwin":  "~/.cursor/mcp.json",
			"linux":   "~/.cursor/mcp.json",
			"windows": "%USERPROFILE%\\.cursor\\mcp.json",
		},
		ServerKey:     "kusari-inspector",
		ConfigFormat:  ConfigFormatStandard,
		InstallMethod: InstallMethodFile,
	},
	{
		Name: "Windsurf",
		ID:   "windsurf",
		ConfigPaths: map[string]string{
			"darwin":  "~/.codeium/windsurf/mcp_config.json",
			"linux":   "~/.codeium/windsurf/mcp_config.json",
			"windows": "%USERPROFILE%\\.codeium\\windsurf\\mcp_config.json",
		},
		ServerKey:     "kusari-inspector",
		ConfigFormat:  ConfigFormatStandard,
		InstallMethod: InstallMethodFile,
	},
	{
		Name: "Cline",
		ID:   "cline",
		ConfigPaths: map[string]string{
			"darwin":  "~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json",
			"linux":   "~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json",
			"windows": "%APPDATA%\\Code\\User\\globalStorage\\saoudrizwan.claude-dev\\settings\\cline_mcp_settings.json",
		},
		ServerKey:     "kusari-inspector",
		ConfigFormat:  ConfigFormatStandard,
		InstallMethod: InstallMethodFile,
	},
	{
		Name: "Continue",
		ID:   "continue",
		ConfigPaths: map[string]string{
			// Continue uses config.json with experimental.modelContextProtocolServers array format
			"darwin":  "~/.continue/config.json",
			"linux":   "~/.continue/config.json",
			"windows": "%USERPROFILE%\\.continue\\config.json",
		},
		ServerKey:     "kusari-inspector",
		ConfigFormat:  ConfigFormatContinue,
		InstallMethod: InstallMethodFile,
	},
}

// GetAllClients returns all supported MCP clients.
func GetAllClients() []ClientConfig {
	return supportedClients
}

// GetClient returns the client configuration for the given ID.
// Client ID matching is case-insensitive.
// Also supports legacy "claude" alias for "claude-code".
func GetClient(id string) (ClientConfig, error) {
	id = strings.ToLower(id)

	// Support legacy "claude" alias for backward compatibility
	if id == "claude" {
		id = "claude-code"
	}

	for _, client := range supportedClients {
		if strings.ToLower(client.ID) == id {
			return client, nil
		}
	}
	return ClientConfig{}, fmt.Errorf("unknown client: %s", id)
}
