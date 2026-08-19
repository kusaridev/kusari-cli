// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The skills have to be reachable from the compiled binary, not just from a
// checkout: a user who installed via `go install` has no repository to copy
// them out of.
func TestSkills_EmbeddedInBinary(t *testing.T) {
	skills, err := Skills()
	require.NoError(t, err)
	require.NotEmpty(t, skills, "at least one skill must be embedded")

	byName := map[string]Skill{}
	for _, s := range skills {
		byName[s.Name] = s
	}

	scan, ok := byName["kusari-scan"]
	require.True(t, ok, "kusari-scan skill should be embedded, got %v", byName)

	require.Contains(t, scan.Files, "SKILL.md", "a skill is nothing without SKILL.md")
	assert.Contains(t, scan.Files, "README.md")

	// Claude Code only recognizes a skill whose SKILL.md carries frontmatter
	// with a name and description, so a malformed file makes the skill silently
	// invisible rather than failing loudly.
	body := string(scan.Files["SKILL.md"])
	assert.True(t, strings.HasPrefix(body, "---\n"), "SKILL.md must open with YAML frontmatter")
	assert.Contains(t, body, "name: \"kusari-scan\"", "frontmatter must declare the skill name")
	assert.Contains(t, body, "description:", "frontmatter must declare a description")
	assert.Contains(t, body, "scan_local_changes", "the skill must actually invoke the MCP scan tool")
}

// Every embedded file must be non-empty; an empty SKILL.md installs cleanly and
// then does nothing.
func TestSkills_NoEmptyFiles(t *testing.T) {
	skills, err := Skills()
	require.NoError(t, err)

	for _, s := range skills {
		for name, data := range s.Files {
			assert.NotEmpty(t, data, "%s/%s is embedded but empty", s.Name, name)
		}
	}
}

// Skills are a Claude Code construct; the other supported editors have no
// equivalent, and claiming otherwise would litter their config directories.
func TestSupportsSkills_OnlyClaudeCode(t *testing.T) {
	want := map[string]bool{
		"claude-code":    true,
		"claude-desktop": false,
		"cursor":         false,
		"windsurf":       false,
		"cline":          false,
		"continue":       false,
	}

	for _, client := range GetAllClients() {
		expected, known := want[client.ID]
		require.True(t, known, "unhandled client %q: decide whether it supports skills", client.ID)
		assert.Equal(t, expected, SupportsSkills(client),
			"client %q skills support", client.ID)
	}
}

// The skills directory must be user-level, so one install covers every
// repository the user works in rather than needing setup per repo.
func TestGetSkillsPath_IsUserLevel(t *testing.T) {
	client, err := GetClient("claude-code")
	require.NoError(t, err)

	path, err := GetSkillsPath(client)
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(path, home),
		"skills path %q should live under the home directory %q", path, home)
	assert.Equal(t, filepath.Join(home, ".claude", "skills"), path)
	assert.NotContains(t, path, "kusari-cli",
		"skills path must not be tied to whichever repo the install ran from")
}

func TestGetSkillsPath_UnsupportedClient(t *testing.T) {
	client, err := GetClient("cursor")
	require.NoError(t, err)

	_, err = GetSkillsPath(client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support agent skills")
}

// A full round trip against a real directory: install writes the skill where
// Claude Code looks for it, and uninstall takes it back out.
func TestInstallSkills_RoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	client, err := GetClient("claude-code")
	require.NoError(t, err)

	root, err := GetSkillsPath(client)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(root, home), "test must not touch the real home dir")

	installed, err := InstallSkills(client)
	require.NoError(t, err)
	require.Contains(t, installed, "kusari-scan")

	// SKILL.md is what makes it a skill; without it Claude Code ignores the dir.
	skillFile := filepath.Join(root, "kusari-scan", "SKILL.md")
	data, err := os.ReadFile(skillFile)
	require.NoError(t, err, "SKILL.md should exist at %s", skillFile)
	assert.Contains(t, string(data), "kusari-scan")

	// Installing twice must be idempotent, since upgrades reinstall.
	_, err = InstallSkills(client)
	require.NoError(t, err, "reinstall over an existing copy should succeed")

	removed, err := UninstallSkills(client)
	require.NoError(t, err)
	assert.Contains(t, removed, "kusari-scan")
	_, err = os.Stat(filepath.Join(root, "kusari-scan"))
	assert.True(t, os.IsNotExist(err), "skill directory should be gone")

	// Uninstalling again is not an error.
	_, err = UninstallSkills(client)
	assert.NoError(t, err)
}

// Uninstall must not remove skills the user wrote themselves.
func TestUninstallSkills_LeavesForeignSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	client, err := GetClient("claude-code")
	require.NoError(t, err)
	root, err := GetSkillsPath(client)
	require.NoError(t, err)

	_, err = InstallSkills(client)
	require.NoError(t, err)

	// A skill that is not ours.
	foreign := filepath.Join(root, "my-own-skill")
	require.NoError(t, os.MkdirAll(foreign, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(foreign, "SKILL.md"), []byte("mine"), 0644))

	_, err = UninstallSkills(client)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(foreign, "SKILL.md"))
	assert.NoError(t, err, "uninstall must not touch skills the user created")
}

func TestValidSkillName_RejectsTraversal(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "../evil", "a/b", `a\b`, "/abs"} {
		assert.Error(t, validSkillName(bad), "should reject %q", bad)
	}
	assert.NoError(t, validSkillName("kusari-scan"))
}
