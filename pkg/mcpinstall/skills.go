// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcpinstall

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Skills are embedded rather than read from disk so they ship inside the
// binary: a user who installs via `go install` or a release archive has no
// checkout to copy them from. The repo's own .claude/ directory is gitignored
// and only ever affects someone running an agent inside this repository, so it
// cannot be the source of truth for what customers get.
//
//go:embed all:skills
var skillFS embed.FS

// Skill is one agent skill that can be written into a client's skills
// directory.
type Skill struct {
	// Name is the directory name the skill is installed under.
	Name string
	// Files maps a path relative to the skill directory to its contents.
	Files map[string][]byte
}

// Skills returns every embedded skill, ready to be written to disk.
func Skills() ([]Skill, error) {
	entries, err := fs.ReadDir(skillFS, "skills")
	if err != nil {
		return nil, err
	}

	skills := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skill := Skill{Name: entry.Name(), Files: map[string][]byte{}}
		root := "skills/" + entry.Name()
		err := fs.WalkDir(skillFS, root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, err := skillFS.ReadFile(path)
			if err != nil {
				return err
			}
			// embed.FS always uses forward slashes, so the prefix trim is exact.
			skill.Files[strings.TrimPrefix(path, root+"/")] = data
			return nil
		})
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

// SupportsSkills reports whether this client has a skills directory on the
// current platform.
func SupportsSkills(client ClientConfig) bool {
	if client.SkillsPaths == nil {
		return false
	}
	_, ok := client.SkillsPaths[string(GetPlatform())]
	return ok
}

// GetSkillsPath returns the directory skills are installed into for this
// client, or an error if the client has no skills mechanism.
func GetSkillsPath(client ClientConfig) (string, error) {
	if client.SkillsPaths == nil {
		return "", fmt.Errorf("client %s does not support agent skills", client.ID)
	}
	platform := GetPlatform()
	path, ok := client.SkillsPaths[string(platform)]
	if !ok {
		return "", fmt.Errorf("client %s does not support agent skills on platform %s", client.ID, platform)
	}
	return ExpandConfigPath(path), nil
}

// InstallSkills writes every embedded skill into the client's skills directory,
// returning the names installed. Existing files are overwritten: these are
// managed artifacts, and an upgrade has to be able to replace an older copy.
func InstallSkills(client ClientConfig) ([]string, error) {
	root, err := GetSkillsPath(client)
	if err != nil {
		return nil, err
	}

	skills, err := Skills()
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded skills: %w", err)
	}

	installed := make([]string, 0, len(skills))
	for _, skill := range skills {
		if err := validSkillName(skill.Name); err != nil {
			return installed, err
		}
		dir := filepath.Join(root, skill.Name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return installed, fmt.Errorf("failed to create %s: %w", dir, err)
		}
		for name, data := range skill.Files {
			// Embedded paths are ours, but they still land under the user's home
			// directory, so refuse anything that could escape the skill dir.
			target := filepath.Join(dir, filepath.FromSlash(name))
			if !strings.HasPrefix(target, dir+string(os.PathSeparator)) {
				return installed, fmt.Errorf("refusing to write %s outside %s", target, dir)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return installed, fmt.Errorf("failed to create %s: %w", filepath.Dir(target), err)
			}
			if err := os.WriteFile(target, data, 0644); err != nil {
				return installed, fmt.Errorf("failed to write %s: %w", target, err)
			}
		}
		installed = append(installed, skill.Name)
	}
	return installed, nil
}

// UninstallSkills removes only the skills this binary ships, leaving anything
// the user wrote themselves alone.
func UninstallSkills(client ClientConfig) ([]string, error) {
	root, err := GetSkillsPath(client)
	if err != nil {
		return nil, err
	}

	skills, err := Skills()
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded skills: %w", err)
	}

	removed := make([]string, 0, len(skills))
	for _, skill := range skills {
		if err := validSkillName(skill.Name); err != nil {
			return removed, err
		}
		dir := filepath.Join(root, skill.Name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return removed, fmt.Errorf("failed to remove %s: %w", dir, err)
		}
		removed = append(removed, skill.Name)
	}
	return removed, nil
}

// validSkillName guards the directory join: these names become paths under the
// user's home directory, and RemoveAll on a bad one would be destructive.
func validSkillName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid skill name %q", name)
	}
	if strings.ContainsAny(name, `/\`) || filepath.IsAbs(name) {
		return fmt.Errorf("invalid skill name %q: must be a single path element", name)
	}
	return nil
}
