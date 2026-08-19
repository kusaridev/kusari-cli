// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallationResult contains the result of an install/uninstall operation.
type InstallationResult struct {
	// Success indicates whether the operation succeeded.
	Success bool
	// ClientName is the client that was configured.
	ClientName string
	// ConfigPath is the path to the config file modified (empty for CLI-based).
	ConfigPath string
	// Message contains success or error message.
	Message string
	// NeedsRestart indicates whether the client needs restart.
	NeedsRestart bool
	// SkillsPath is the directory skills were installed into, empty if none were.
	SkillsPath string
	// Skills lists the skill names installed or removed.
	Skills []string
	// SkillsSupported reports whether the client has a skills mechanism at all;
	// false means the caller should say so rather than imply skills were set up.
	SkillsSupported bool
	// SkillsError is set when MCP configuration succeeded but skills did not.
	// Skills are an enhancement, so this does not fail the install.
	SkillsError error
	// CommitHookRequested reports whether the caller asked for the commit hook.
	CommitHookRequested bool
	// CommitHookInstalled reports whether it was actually installed or removed.
	CommitHookInstalled bool
	// CommitHookScript is the path of the installed hook script.
	CommitHookScript string
	// CommitHookSettings is the settings file the hook was registered in.
	CommitHookSettings string
	// CommitHookBinary is the kusari executable the hook will invoke. Reported
	// because the hook records whichever binary installed it, which may be a
	// throwaway build rather than the installed one.
	CommitHookBinary string
	// CommitHookError is set when the hook could not be installed or removed.
	CommitHookError error
}

// InstallOptions carries opt-in choices for an install.
type InstallOptions struct {
	// WithCommitHook installs a PreToolUse hook that scans before Claude commits.
	// Off by default: unlike a skill file, a hook is a shell command that runs on
	// the user's machine, so it has to be asked for explicitly.
	WithCommitHook bool
}

// GetServerConfig returns the MCP server configuration to add to client configs.
func GetServerConfig() map[string]interface{} {
	return map[string]interface{}{
		"command": "kusari",
		"args":    []string{"ai", "serve", "--verbose"},
	}
}

// GetServerArgs returns the server command arguments.
func GetServerArgs() []string {
	return []string{"ai", "serve"}
}

// Install installs the Kusari MCP server for the given client.
func Install(client ClientConfig) (*InstallationResult, error) {
	return InstallWithOptions(client, InstallOptions{})
}

// InstallWithOptions configures the MCP server plus any opted-in extras.
func InstallWithOptions(client ClientConfig, opts InstallOptions) (*InstallationResult, error) {
	var result *InstallationResult
	var err error
	if client.InstallMethod == InstallMethodCLI {
		result, err = installViaCLI(client)
	} else {
		result, err = installViaFile(client)
	}
	if err != nil || result == nil || !result.Success {
		return result, err
	}

	// Skills are an enhancement layered on top of the MCP server. A failure here
	// must not fail the install: the server is already configured and useful.
	result.SkillsSupported = SupportsSkills(client)
	if result.SkillsSupported {
		if path, pathErr := GetSkillsPath(client); pathErr == nil {
			result.SkillsPath = path
		}
		installed, skillErr := InstallSkills(client)
		result.Skills = installed
		result.SkillsError = skillErr
	}

	// Same reasoning as skills: a hook failure must not fail the install, since
	// the MCP server is already configured and useful without it.
	result.CommitHookRequested = opts.WithCommitHook
	if opts.WithCommitHook {
		if !CommitHookSupported(client) {
			result.CommitHookError = fmt.Errorf("the commit hook is not supported for %s on %s", client.Name, runtime.GOOS)
		} else {
			script, settings, binary, hookErr := InstallCommitHook(client)
			result.CommitHookScript = script
			result.CommitHookSettings = settings
			result.CommitHookBinary = binary
			result.CommitHookError = hookErr
			result.CommitHookInstalled = hookErr == nil
		}
	}
	return result, nil
}

// installViaCLI installs using the client's CLI command (e.g., claude mcp add).
func installViaCLI(client ClientConfig) (*InstallationResult, error) {
	// Check if CLI is available
	cliPath, err := exec.LookPath(client.CLICommand)
	if err != nil {
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			Message:    fmt.Sprintf("%s CLI not found. Please install %s first.", client.CLICommand, client.Name),
		}, fmt.Errorf("%s CLI not found: %w", client.CLICommand, err)
	}

	// Build command: <cli> mcp add <server-key> -- kusari ai serve
	args := []string{"mcp", "add", client.ServerKey, "--", "kusari", "ai", "serve"}

	cmd := exec.Command(cliPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			Message:    fmt.Sprintf("Failed to run %s: %s", client.CLICommand, string(output)),
		}, fmt.Errorf("CLI install failed: %w", err)
	}

	return &InstallationResult{
		Success:      true,
		ClientName:   client.Name,
		Message:      fmt.Sprintf("Successfully configured %s", client.Name),
		NeedsRestart: true,
	}, nil
}

// installViaFile installs by writing to a config file.
func installViaFile(client ClientConfig) (*InstallationResult, error) {
	configPath, err := GetConfigPath(client)
	if err != nil {
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			Message:    fmt.Sprintf("Failed to get config path: %v", err),
		}, err
	}

	// Create parent directories if needed
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			ConfigPath: configPath,
			Message:    fmt.Sprintf("Failed to create directory: %v", err),
		}, err
	}

	// Read existing config or create new one
	config, err := readConfigFile(configPath)
	if err != nil {
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			ConfigPath: configPath,
			Message:    fmt.Sprintf("Failed to read config: %v", err),
		}, err
	}

	// Add or update the kusari-inspector server
	if client.ConfigFormat == ConfigFormatContinue {
		addServerContinueFormat(config, client.ServerKey)
	} else {
		addServerStandardFormat(config, client.ServerKey)
	}

	// Write config back
	if err := writeConfigFile(configPath, config); err != nil {
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			ConfigPath: configPath,
			Message:    fmt.Sprintf("Failed to write config: %v", err),
		}, err
	}

	return &InstallationResult{
		Success:      true,
		ClientName:   client.Name,
		ConfigPath:   configPath,
		Message:      fmt.Sprintf("Successfully configured %s", client.Name),
		NeedsRestart: true,
	}, nil
}

// Uninstall removes the Kusari MCP server from the given client.
func Uninstall(client ClientConfig) (*InstallationResult, error) {
	var result *InstallationResult
	var err error
	if client.InstallMethod == InstallMethodCLI {
		result, err = uninstallViaCLI(client)
	} else {
		result, err = uninstallViaFile(client)
	}
	if err != nil || result == nil {
		return result, err
	}

	// Remove skills even if the MCP removal reported a problem: leaving orphaned
	// skill directories behind is worse than a partial cleanup.
	result.SkillsSupported = SupportsSkills(client)
	if result.SkillsSupported {
		if path, pathErr := GetSkillsPath(client); pathErr == nil {
			result.SkillsPath = path
		}
		removed, skillErr := UninstallSkills(client)
		result.Skills = removed
		result.SkillsError = skillErr
	}

	// Always attempt hook removal, whether or not it was ever installed:
	// leaving a hook behind that shells to a removed integration is worse than
	// a no-op cleanup.
	hookRemoved, hookErr := UninstallCommitHook(client)
	result.CommitHookInstalled = hookRemoved
	result.CommitHookError = hookErr
	return result, nil
}

// uninstallViaCLI uninstalls using the client's CLI command.
func uninstallViaCLI(client ClientConfig) (*InstallationResult, error) {
	// Check if CLI is available
	cliPath, err := exec.LookPath(client.CLICommand)
	if err != nil {
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			Message:    fmt.Sprintf("%s CLI not found.", client.CLICommand),
		}, fmt.Errorf("%s CLI not found: %w", client.CLICommand, err)
	}

	// Build command: <cli> mcp remove <server-key>
	args := []string{"mcp", "remove", client.ServerKey}

	cmd := exec.Command(cliPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If the server wasn't installed, that's okay
		if strings.Contains(string(output), "not found") || strings.Contains(string(output), "does not exist") {
			return &InstallationResult{
				Success:      true,
				ClientName:   client.Name,
				Message:      "Not installed",
				NeedsRestart: false,
			}, nil
		}
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			Message:    fmt.Sprintf("Failed to run %s: %s", client.CLICommand, string(output)),
		}, fmt.Errorf("CLI uninstall failed: %w", err)
	}

	return &InstallationResult{
		Success:      true,
		ClientName:   client.Name,
		Message:      fmt.Sprintf("Removed Kusari Inspector from %s", client.Name),
		NeedsRestart: true,
	}, nil
}

// uninstallViaFile uninstalls by modifying the config file.
func uninstallViaFile(client ClientConfig) (*InstallationResult, error) {
	configPath, err := GetConfigPath(client)
	if err != nil {
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			Message:    fmt.Sprintf("Failed to get config path: %v", err),
		}, err
	}

	// Read existing config
	config, err := readConfigFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &InstallationResult{
				Success:      true,
				ClientName:   client.Name,
				ConfigPath:   configPath,
				Message:      "Not installed",
				NeedsRestart: false,
			}, nil
		}
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			ConfigPath: configPath,
			Message:    fmt.Sprintf("Failed to read config: %v", err),
		}, err
	}

	// Remove the kusari-inspector server
	if client.ConfigFormat == ConfigFormatContinue {
		removeServerContinueFormat(config, client.ServerKey)
	} else {
		removeServerStandardFormat(config, client.ServerKey)
	}

	// Write config back
	if err := writeConfigFile(configPath, config); err != nil {
		return &InstallationResult{
			Success:    false,
			ClientName: client.Name,
			ConfigPath: configPath,
			Message:    fmt.Sprintf("Failed to write config: %v", err),
		}, err
	}

	return &InstallationResult{
		Success:      true,
		ClientName:   client.Name,
		ConfigPath:   configPath,
		Message:      fmt.Sprintf("Removed Kusari Inspector from %s", client.Name),
		NeedsRestart: true,
	}, nil
}

// readConfigFile reads a JSON config file, returning empty map if file doesn't exist.
func readConfigFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}

	return config, nil
}

// writeConfigFile writes a JSON config file with proper formatting.
func writeConfigFile(path string, config map[string]interface{}) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// addServerStandardFormat adds the server to the mcpServers object (Claude, Cursor, etc.)
func addServerStandardFormat(config map[string]interface{}, serverKey string) {
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}
	mcpServers[serverKey] = GetServerConfig()
}

// removeServerStandardFormat removes the server from the mcpServers object.
func removeServerStandardFormat(config map[string]interface{}, serverKey string) {
	if mcpServers, ok := config["mcpServers"].(map[string]interface{}); ok {
		delete(mcpServers, serverKey)
	}
}

// addServerContinueFormat adds the server to Continue's experimental.modelContextProtocolServers array.
func addServerContinueFormat(config map[string]interface{}, serverKey string) {
	experimental, ok := config["experimental"].(map[string]interface{})
	if !ok {
		experimental = make(map[string]interface{})
		config["experimental"] = experimental
	}

	servers, ok := experimental["modelContextProtocolServers"].([]interface{})
	if !ok {
		servers = make([]interface{}, 0)
	}

	// Check if already exists
	for _, s := range servers {
		if srv, ok := s.(map[string]interface{}); ok {
			if srv["name"] == serverKey {
				// Update existing
				srv["command"] = "kusari"
				srv["args"] = []string{"ai", "serve"}
				return
			}
		}
	}

	// Add new server
	serverConfig := GetServerConfig()
	serverConfig["name"] = serverKey
	servers = append(servers, serverConfig)
	experimental["modelContextProtocolServers"] = servers
}

// removeServerContinueFormat removes the server from Continue's array.
func removeServerContinueFormat(config map[string]interface{}, serverKey string) {
	experimental, ok := config["experimental"].(map[string]interface{})
	if !ok {
		return
	}

	servers, ok := experimental["modelContextProtocolServers"].([]interface{})
	if !ok {
		return
	}

	filtered := make([]interface{}, 0, len(servers))
	for _, s := range servers {
		if srv, ok := s.(map[string]interface{}); ok {
			if srv["name"] != serverKey {
				filtered = append(filtered, s)
			}
		}
	}
	experimental["modelContextProtocolServers"] = filtered
}

// ClientStatus contains the installation status for a client.
type ClientStatus struct {
	// ClientName is the human-readable client name.
	ClientName string
	// ClientID is the CLI identifier.
	ClientID string
	// Installed indicates whether kusari-inspector is configured.
	Installed bool
	// ConfigPath is the path to the config file (empty for CLI-based).
	ConfigPath string
	// InstallMethod indicates how the client is configured.
	InstallMethod InstallMethod
}

// ListClients returns the installation status for all given clients.
func ListClients(clients []ClientConfig) []ClientStatus {
	results := make([]ClientStatus, 0, len(clients))

	for _, client := range clients {
		status := ClientStatus{
			ClientName:    client.Name,
			ClientID:      client.ID,
			Installed:     false,
			InstallMethod: client.InstallMethod,
		}

		if client.InstallMethod == InstallMethodCLI {
			// For CLI-based clients, check if CLI exists and try to list servers
			status.Installed = checkCLIInstallStatus(client)
		} else {
			// For file-based clients, check config file
			configPath, err := GetConfigPath(client)
			if err != nil {
				results = append(results, status)
				continue
			}
			status.ConfigPath = configPath

			config, err := readConfigFile(configPath)
			if err != nil {
				results = append(results, status)
				continue
			}

			status.Installed = isServerInstalled(config, client)
		}

		results = append(results, status)
	}

	return results
}

// checkCLIInstallStatus checks if the MCP server is installed for a CLI-based client.
func checkCLIInstallStatus(client ClientConfig) bool {
	cliPath, err := exec.LookPath(client.CLICommand)
	if err != nil {
		return false
	}

	// Try to get the server info: <cli> mcp get <server-key>
	cmd := exec.Command(cliPath, "mcp", "get", client.ServerKey)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	// If we got output without error, the server exists
	return len(output) > 0 && !strings.Contains(string(output), "not found")
}

// isServerInstalled checks if kusari-inspector is configured in the given config.
func isServerInstalled(config map[string]interface{}, client ClientConfig) bool {
	if client.ConfigFormat == ConfigFormatContinue {
		return isServerInstalledContinueFormat(config, client.ServerKey)
	}
	return isServerInstalledStandardFormat(config, client.ServerKey)
}

// isServerInstalledStandardFormat checks the mcpServers object format.
func isServerInstalledStandardFormat(config map[string]interface{}, serverKey string) bool {
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		return false
	}
	_, exists := mcpServers[serverKey]
	return exists
}

// isServerInstalledContinueFormat checks Continue's experimental array format.
func isServerInstalledContinueFormat(config map[string]interface{}, serverKey string) bool {
	experimental, ok := config["experimental"].(map[string]interface{})
	if !ok {
		return false
	}

	servers, ok := experimental["modelContextProtocolServers"].([]interface{})
	if !ok {
		return false
	}

	for _, s := range servers {
		if srv, ok := s.(map[string]interface{}); ok {
			if srv["name"] == serverKey {
				return true
			}
		}
	}
	return false
}
