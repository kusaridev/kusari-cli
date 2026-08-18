// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func claudeCode(t *testing.T) ClientConfig {
	t.Helper()
	client, err := GetClient("claude-code")
	require.NoError(t, err)
	return client
}

// sandboxHome points home-directory lookups at a temp dir so no test can touch
// the developer's real settings.json.
func sandboxHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func readSettingsFile(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &out), "settings must remain valid JSON")
	return out
}

// The hook is bash; Claude Code on Windows runs hooks through PowerShell unless
// Git Bash is present, so installing it there would fail silently on every
// commit.
func TestCommitHookSupported(t *testing.T) {
	assert.Equal(t, runtime.GOOS != "windows", CommitHookSupported(claudeCode(t)))

	cursor, err := GetClient("cursor")
	require.NoError(t, err)
	assert.False(t, CommitHookSupported(cursor), "only Claude Code has this hook mechanism")
}

func TestInstallCommitHook_WritesScriptAndSettings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("commit hook is not supported on Windows")
	}
	sandboxHome(t)

	script, settingsPath, binary, err := InstallCommitHook(claudeCode(t))
	require.NoError(t, err)
	assert.NotEmpty(t, binary, "the hook must report which binary it will invoke")

	// The script must be executable, or the hook silently never runs.
	info, err := os.Stat(script)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0100, "hook script must be executable, got %v", info.Mode())

	// The binary placeholder must be resolved: a hook does not inherit PATH.
	body, err := os.ReadFile(script)
	require.NoError(t, err)
	assert.NotContains(t, string(body), hookBinaryPlaceholder,
		"the binary placeholder must be substituted at install time")

	// And the flags that matter must be present, since the CLI defaults differ
	// from what the MCP path passes.
	assert.Contains(t, string(body), "--full-output",
		"without --full-output the CLI truncates findings and the model sees a partial list")
	assert.Contains(t, string(body), "--fail-on-findings",
		"the verdict must arrive as an exit code rather than be parsed out of the report")
	assert.Contains(t, string(body), "--output-format markdown",
		"markdown is what the model reads best; sarif was only needed while the verdict had to be machine readable")

	settings := readSettingsFile(t, settingsPath)
	pre := settings["hooks"].(map[string]interface{})["PreToolUse"].([]interface{})
	require.Len(t, pre, 1)
	entry := pre[0].(map[string]interface{})
	assert.Equal(t, "Bash", entry["matcher"])
	inner := entry["hooks"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "command", inner["type"])
	assert.Equal(t, script, inner["command"])
	// A scan is an upload plus remote analysis; a short timeout would kill it.
	assert.EqualValues(t, hookTimeoutSeconds, inner["timeout"])
}

// The merge must be additive. Clobbering a user's existing hooks would silently
// disable their tooling.
func TestInstallCommitHook_PreservesExistingSettings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("commit hook is not supported on Windows")
	}
	home := sandboxHome(t)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0755))
	existing := `{
  "theme": "dark",
  "permissions": { "allow": ["Bash(git *)"] },
  "hooks": {
    "PreToolUse": [
      { "matcher": "Write", "hooks": [{ "type": "command", "command": "my-linter" }] }
    ],
    "PostToolUse": [
      { "matcher": "Edit", "hooks": [{ "type": "command", "command": "my-formatter" }] }
    ]
  }
}`
	require.NoError(t, os.WriteFile(settingsPath, []byte(existing), 0644))

	_, _, _, err := InstallCommitHook(claudeCode(t))
	require.NoError(t, err)

	settings := readSettingsFile(t, settingsPath)
	assert.Equal(t, "dark", settings["theme"], "unrelated settings must survive")
	assert.NotNil(t, settings["permissions"], "permissions must survive")

	hooks := settings["hooks"].(map[string]interface{})
	assert.NotNil(t, hooks["PostToolUse"], "other hook events must survive")

	pre := hooks["PreToolUse"].([]interface{})
	require.Len(t, pre, 2, "our entry should be appended, not replace the user's")

	var sawLinter, sawOurs bool
	for _, e := range pre {
		cmd := e.(map[string]interface{})["hooks"].([]interface{})[0].(map[string]interface{})["command"].(string)
		if cmd == "my-linter" {
			sawLinter = true
		}
		if strings.Contains(cmd, hookMarker) {
			sawOurs = true
		}
	}
	assert.True(t, sawLinter, "the user's existing PreToolUse hook must be preserved")
	assert.True(t, sawOurs, "our hook must be registered")
}

// Reinstalling must not stack duplicate entries, or every commit scans twice.
func TestInstallCommitHook_Idempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("commit hook is not supported on Windows")
	}
	sandboxHome(t)
	client := claudeCode(t)

	_, settingsPath, _, err := InstallCommitHook(client)
	require.NoError(t, err)
	_, _, _, err = InstallCommitHook(client)
	require.NoError(t, err)

	settings := readSettingsFile(t, settingsPath)
	pre := settings["hooks"].(map[string]interface{})["PreToolUse"].([]interface{})
	assert.Len(t, pre, 1, "reinstall should refresh the entry, not duplicate it")
}

func TestUninstallCommitHook_RemovesOnlyOurs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("commit hook is not supported on Windows")
	}
	home := sandboxHome(t)
	client := claudeCode(t)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0755))
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Write", "hooks": [{ "type": "command", "command": "my-linter" }] }
    ]
  }
}`), 0644))

	script, _, _, err := InstallCommitHook(client)
	require.NoError(t, err)

	removed, err := UninstallCommitHook(client)
	require.NoError(t, err)
	assert.True(t, removed)

	_, err = os.Stat(script)
	assert.True(t, os.IsNotExist(err), "hook script should be deleted")

	settings := readSettingsFile(t, settingsPath)
	pre := settings["hooks"].(map[string]interface{})["PreToolUse"].([]interface{})
	require.Len(t, pre, 1, "the user's hook must remain")
	cmd := pre[0].(map[string]interface{})["hooks"].([]interface{})[0].(map[string]interface{})["command"].(string)
	assert.Equal(t, "my-linter", cmd)
}

// With nothing else configured, uninstall should leave no empty scaffolding.
func TestUninstallCommitHook_PrunesEmptyContainers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("commit hook is not supported on Windows")
	}
	sandboxHome(t)
	client := claudeCode(t)

	_, settingsPath, _, err := InstallCommitHook(client)
	require.NoError(t, err)
	_, err = UninstallCommitHook(client)
	require.NoError(t, err)

	settings := readSettingsFile(t, settingsPath)
	assert.NotContains(t, settings, "hooks", "an empty hooks object should not be left behind")
}

func TestUninstallCommitHook_NoopWhenNotInstalled(t *testing.T) {
	sandboxHome(t)
	removed, err := UninstallCommitHook(claudeCode(t))
	require.NoError(t, err)
	assert.False(t, removed)
}

// A settings file we cannot parse must not be overwritten: doing so would
// destroy configuration, and a malformed settings.json disables every setting
// in it.
func TestInstallCommitHook_RefusesToClobberMalformedSettings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("commit hook is not supported on Windows")
	}
	home := sandboxHome(t)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0755))
	garbage := `{"theme": "dark",,,`
	require.NoError(t, os.WriteFile(settingsPath, []byte(garbage), 0644))

	_, _, _, err := InstallCommitHook(claudeCode(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid JSON")

	after, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Equal(t, garbage, string(after), "the original file must be left untouched")
}

// The hook is opt-in: a default install must not write one.
func TestInstall_DoesNotInstallHookByDefault(t *testing.T) {
	sandboxHome(t)

	result, err := InstallWithOptions(claudeCode(t), InstallOptions{})
	require.NoError(t, err)
	require.True(t, result.Success)
	assert.False(t, result.CommitHookRequested)
	assert.False(t, result.CommitHookInstalled)

	scriptPath, err := HookScriptPath()
	require.NoError(t, err)
	_, err = os.Stat(scriptPath)
	assert.True(t, os.IsNotExist(err), "no hook script should exist without --with-commit-hook")
}

// runHookScript renders the hook with KUSARI_BIN pointed at a stub and runs it
// against a payload, returning stdout and the exit status.
//
// This executes the real script rather than inspecting it. Both bugs found while
// building the hook were invisible to static review: a jq expression that mapped
// "findings" onto "no verdict", and a shell-quoting slip that produced invalid
// JSON. Only running it surfaced either.
func runHookScript(t *testing.T, stub string, payload string) (string, int) {
	t.Helper()

	dir := t.TempDir()

	stubPath := filepath.Join(dir, "kusari-stub")
	require.NoError(t, os.WriteFile(stubPath, []byte(stub), 0755))

	source, err := os.ReadFile(filepath.Join("hooks", hookScriptName))
	require.NoError(t, err)
	script := filepath.Join(dir, hookScriptName)
	require.NoError(t, os.WriteFile(script,
		[]byte(strings.ReplaceAll(string(source), hookBinaryPlaceholder, stubPath)), 0755))

	// A git repo with an uncommitted change, so the hook does not early-exit on
	// "nothing to scan".
	repo := filepath.Join(dir, "repo")
	require.NoError(t, os.MkdirAll(repo, 0755))
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "T"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	require.NoError(t, os.WriteFile(filepath.Join(repo, "f.txt"), []byte("base\n"), 0644))
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	require.NoError(t, os.WriteFile(filepath.Join(repo, "f.txt"), []byte("changed\n"), 0644))

	cmd := exec.Command("bash", script)
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(payload)
	var stdout strings.Builder
	cmd.Stdout = &stdout
	err = cmd.Run()

	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	return stdout.String(), code
}

const commitPayload = `{"tool_name":"Bash","tool_input":{"command":"git commit -m \"feat\""}}`

func TestHookScript_Policy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hook script is bash; not installed on Windows")
	}

	findings := "#!/bin/sh\necho '## Findings'\necho '- src/config.js:18 hardcoded key'\nexit 3\n"
	cleanScan := "#!/bin/sh\necho '## No findings'\nexit 0\n"
	noAuth := "#!/bin/sh\necho 'failed to load auth token' >&2\nexit 1\n"
	broken := "#!/bin/sh\necho 'connection refused' >&2\nexit 1\n"

	tests := []struct {
		name     string
		stub     string
		payload  string
		wantCode int
		// wantSkip means the hook must emit an allow decision with a reason.
		wantSkip bool
	}{
		{
			name:     "non-commit command is allowed without scanning",
			stub:     findings,
			payload:  `{"tool_name":"Bash","tool_input":{"command":"ls -la"}}`,
			wantCode: 0,
		},
		{
			name:     "findings block the commit",
			stub:     findings,
			payload:  commitPayload,
			wantCode: 2,
		},
		{
			name:     "clean scan allows the commit",
			stub:     cleanScan,
			payload:  commitPayload,
			wantCode: 0,
		},
		{
			// A developer cannot log in from inside a hook, so blocking here
			// would strand them with no way forward.
			name:     "expired auth allows the commit with a login hint",
			stub:     noAuth,
			payload:  commitPayload,
			wantCode: 0,
			wantSkip: true,
		},
		{
			name:     "a broken scan allows the commit",
			stub:     broken,
			payload:  commitPayload,
			wantCode: 0,
			wantSkip: true,
		},
		{
			// Claude's most common commit shape; a settings-level `if` filter on
			// a command prefix would miss it.
			name:     "a commit chained after another command still blocks",
			stub:     findings,
			payload:  `{"tool_name":"Bash","tool_input":{"command":"git add -A && git commit -m \"x\""}}`,
			wantCode: 2,
		},
		{
			name:     "git -C <path> commit still blocks",
			stub:     findings,
			payload:  `{"tool_name":"Bash","tool_input":{"command":"git -C /r commit -m \"x\""}}`,
			wantCode: 2,
		},
		{
			name:     "a non-commit git command does not trigger a scan",
			stub:     cleanScan,
			payload:  `{"tool_name":"Bash","tool_input":{"command":"git log --oneline"}}`,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, code := runHookScript(t, tt.stub, tt.payload)
			assert.Equal(t, tt.wantCode, code)

			if !tt.wantSkip {
				return
			}

			// Malformed JSON here is silent: the harness cannot report a
			// decision it could not parse.
			var decoded struct {
				SystemMessage      string `json:"systemMessage"`
				HookSpecificOutput struct {
					PermissionDecision string `json:"permissionDecision"`
				} `json:"hookSpecificOutput"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &decoded),
				"skip output must be valid JSON, got: %s", stdout)
			assert.Equal(t, "allow", decoded.HookSpecificOutput.PermissionDecision,
				"a skip must never deny the commit")
			assert.NotEmpty(t, decoded.SystemMessage, "a skip must say why")
		})
	}
}

// A hook pinned to a throwaway build is a real hazard: the script fails safe and
// lets commits through, so a rebuilt or deleted binary disables scanning with
// nothing to announce it.
func TestTransientBinary(t *testing.T) {
	assert.False(t, TransientBinary(""), "no recorded path is not a transient build")
	assert.False(t, TransientBinary("kusari"), "the bare fallback name resolves via PATH at run time")
	assert.True(t, TransientBinary("/some/checkout/kusari/kusari"),
		"a path that is not what PATH resolves to should be flagged")
}
