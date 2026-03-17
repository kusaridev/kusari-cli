// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Platform represents an operating system platform.
type Platform string

const (
	PlatformDarwin  Platform = "darwin"
	PlatformLinux   Platform = "linux"
	PlatformWindows Platform = "windows"
)

// GetPlatform returns the current operating system platform.
func GetPlatform() Platform {
	return Platform(runtime.GOOS)
}

// IsPlatformSupported returns true if the platform is supported.
func IsPlatformSupported(p Platform) bool {
	switch p {
	case PlatformDarwin, PlatformLinux, PlatformWindows:
		return true
	default:
		return false
	}
}

// ExpandConfigPath expands special variables in a config path.
// Supports: ~, $HOME, %APPDATA%, %USERPROFILE%
func ExpandConfigPath(path string) string {
	homeDir, _ := os.UserHomeDir()

	// Expand ~ at the start
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(homeDir, path[2:])
	}

	// Expand $HOME
	if strings.Contains(path, "$HOME") {
		path = strings.ReplaceAll(path, "$HOME", homeDir)
	}

	// Windows-specific expansions
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			path = strings.ReplaceAll(path, "%APPDATA%", appData)
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			path = strings.ReplaceAll(path, "%USERPROFILE%", userProfile)
		}
	}

	return path
}

// GetConfigPath returns the expanded config file path for a client on the current platform.
// Returns empty string for CLI-based clients that don't use config files.
func GetConfigPath(client ClientConfig) (string, error) {
	// CLI-based clients don't have a config file path
	if client.InstallMethod == InstallMethodCLI {
		return "", nil
	}

	platform := GetPlatform()
	path, ok := client.ConfigPaths[string(platform)]
	if !ok {
		return "", fmt.Errorf("client %s not supported on platform %s", client.ID, platform)
	}
	return ExpandConfigPath(path), nil
}
