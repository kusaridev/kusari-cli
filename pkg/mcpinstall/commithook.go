// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed hooks/kusari-precommit.sh
var hookFS embed.FS

const (
	// hookScriptName is the file written into the user's Kusari directory.
	hookScriptName = "kusari-precommit.sh"
	// hookMarker identifies our hook entry inside settings.json so it can be
	// found again for upgrade or removal without disturbing anyone else's hooks.
	hookMarker = "kusari-precommit"
	// hookBinaryPlaceholder is substituted with the absolute path to the running
	// kusari binary. A hook does not inherit the interactive shell's PATH.
	hookBinaryPlaceholder = "__KUSARI_BIN__"
	// hookTimeoutSeconds has to exceed a full scan: upload plus remote analysis.
	hookTimeoutSeconds = 300
)

// CommitHookSupported reports whether the commit hook can be installed for this
// client on this platform.
//
// The hook is a bash script. On Windows, Claude Code runs hooks through
// PowerShell unless Git Bash is present, so shipping the bash version there
// would install something that silently fails on every commit. Better to say it
// is unsupported than to pretend.
func CommitHookSupported(client ClientConfig) bool {
	return client.ID == "claude-code" && runtime.GOOS != "windows"
}

// HookScriptPath returns where the hook script is written. It lives under the
// Kusari directory rather than the agent's, alongside the token and scan cache.
func HookScriptPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, ".kusari", "hooks", hookScriptName), nil
}

// SettingsPath returns the Claude Code settings file the hook is merged into.
// This is user-level settings, matching where skills are installed.
func SettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// writeHookScript materializes the embedded script with the binary path baked
// in, returning both the script path and the binary it will call.
func writeHookScript() (scriptPath string, binary string, err error) {
	scriptPath, err = HookScriptPath()
	if err != nil {
		return "", "", err
	}

	data, err := hookFS.ReadFile("hooks/" + hookScriptName)
	if err != nil {
		return "", "", fmt.Errorf("failed to read embedded hook script: %w", err)
	}

	// Resolve the running binary so the hook does not depend on PATH. Falling
	// back to the bare name keeps the install working when the executable path
	// cannot be determined; the script reports it if the binary is missing.
	binary = "kusari"
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			binary = resolved
		} else {
			binary = exe
		}
	}
	script := strings.ReplaceAll(string(data), hookBinaryPlaceholder, binary)

	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		return "", "", fmt.Errorf("failed to create %s: %w", filepath.Dir(scriptPath), err)
	}
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return "", "", fmt.Errorf("failed to write %s: %w", scriptPath, err)
	}
	return scriptPath, binary, nil
}

// TransientBinary reports whether the path baked into the hook looks like a
// throwaway build rather than an installed binary: it is not what `kusari`
// resolves to on PATH.
//
// This matters because the hook records whichever executable installed it. A
// developer running ./kusari out of a checkout pins the hook to a build
// artifact, and a later rebuild or clean silently disables it -- the script
// fails safe and lets commits through, so nothing announces the breakage.
func TransientBinary(binary string) bool {
	if binary == "" || binary == "kusari" {
		return false
	}
	onPath, err := exec.LookPath("kusari")
	if err != nil {
		// Nothing installed to compare against; the recorded path is all there is.
		return true
	}
	if resolved, err := filepath.EvalSymlinks(onPath); err == nil {
		onPath = resolved
	}
	return onPath != binary
}

// InstallCommitHook writes the hook script and registers it in the user's
// Claude Code settings, returning the script path and the settings path.
func InstallCommitHook(client ClientConfig) (scriptPath string, settingsPath string, binary string, err error) {
	if !CommitHookSupported(client) {
		return "", "", "", fmt.Errorf("the commit hook is not supported for %s on %s", client.Name, runtime.GOOS)
	}

	scriptPath, binary, err = writeHookScript()
	if err != nil {
		return "", "", "", err
	}

	settingsPath, err = SettingsPath()
	if err != nil {
		return "", "", "", err
	}

	settings, err := readSettings(settingsPath)
	if err != nil {
		return "", "", "", err
	}

	if err := mergeCommitHook(settings, scriptPath); err != nil {
		return "", "", "", err
	}

	if err := writeSettings(settingsPath, settings); err != nil {
		return "", "", "", err
	}
	return scriptPath, settingsPath, binary, nil
}

// UninstallCommitHook removes our hook entry and deletes the script. Reports
// whether anything was actually removed.
func UninstallCommitHook(client ClientConfig) (bool, error) {
	if client.ID != "claude-code" {
		return false, nil
	}

	removed := false

	settingsPath, err := SettingsPath()
	if err != nil {
		return false, err
	}
	if _, statErr := os.Stat(settingsPath); statErr == nil {
		settings, err := readSettings(settingsPath)
		if err != nil {
			return false, err
		}
		if pruneCommitHook(settings) {
			if err := writeSettings(settingsPath, settings); err != nil {
				return false, err
			}
			removed = true
		}
	}

	scriptPath, err := HookScriptPath()
	if err != nil {
		return removed, err
	}
	if err := os.Remove(scriptPath); err == nil {
		removed = true
	} else if !os.IsNotExist(err) {
		return removed, fmt.Errorf("failed to remove %s: %w", scriptPath, err)
	}

	return removed, nil
}

// readSettings loads settings.json into a generic map. A missing file is an
// empty settings object; a malformed one is an error, because overwriting it
// would destroy configuration we cannot read.
func readSettings(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]interface{}{}, nil
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON, refusing to overwrite it: %w", path, err)
	}
	return settings, nil
}

// writeSettings persists settings.json, creating parent directories as needed.
// The write goes to a temporary file first: a partial write to settings.json
// silently disables every setting in it.
func writeSettings(path string, settings map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode settings: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".kusari-tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	return nil
}

// commitHookEntry is the PreToolUse entry registered in settings.json.
func commitHookEntry(scriptPath string) map[string]interface{} {
	return map[string]interface{}{
		"matcher": "Bash",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":          "command",
				"command":       scriptPath,
				"timeout":       hookTimeoutSeconds,
				"statusMessage": "Scanning changes with Kusari Inspector...",
			},
		},
	}
}

// mergeCommitHook adds or refreshes our entry under hooks.PreToolUse, leaving
// every other hook the user has configured untouched.
func mergeCommitHook(settings map[string]interface{}, scriptPath string) error {
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		if settings["hooks"] != nil {
			return fmt.Errorf("settings.hooks is not an object; refusing to modify it")
		}
		hooks = map[string]interface{}{}
		settings["hooks"] = hooks
	}

	var entries []interface{}
	if existing, ok := hooks["PreToolUse"]; ok && existing != nil {
		entries, ok = existing.([]interface{})
		if !ok {
			return fmt.Errorf("settings.hooks.PreToolUse is not an array; refusing to modify it")
		}
	}

	// Replace an entry we installed previously rather than stacking duplicates,
	// so reinstalling or upgrading does not scan twice per commit.
	replaced := false
	for i, entry := range entries {
		if isOurHookEntry(entry) {
			entries[i] = commitHookEntry(scriptPath)
			replaced = true
			break
		}
	}
	if !replaced {
		entries = append(entries, commitHookEntry(scriptPath))
	}

	hooks["PreToolUse"] = entries
	return nil
}

// pruneCommitHook removes our entry, reporting whether it found one.
func pruneCommitHook(settings map[string]interface{}) bool {
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		return false
	}
	entries, ok := hooks["PreToolUse"].([]interface{})
	if !ok {
		return false
	}

	kept := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		if isOurHookEntry(entry) {
			continue
		}
		kept = append(kept, entry)
	}
	if len(kept) == len(entries) {
		return false
	}

	// Prune empty containers so an uninstall leaves no residue behind.
	if len(kept) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = kept
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
	return true
}

// isOurHookEntry identifies entries this installer created, by looking for the
// hook script name in any command it runs.
func isOurHookEntry(entry interface{}) bool {
	m, ok := entry.(map[string]interface{})
	if !ok {
		return false
	}
	inner, ok := m["hooks"].([]interface{})
	if !ok {
		return false
	}
	for _, h := range inner {
		hm, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, hookMarker) {
			return true
		}
	}
	return false
}
