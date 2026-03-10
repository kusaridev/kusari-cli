// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// RepoInfo contains repository information extracted from git.
type RepoInfo struct {
	Forge       string
	Org         string
	Repo        string
	SubrepoPath string
}

// ExtractGitRemoteInfo extracts forge, org, repo, and subrepo_path from git remote.
// If repoPath is empty, it uses the current working directory.
func ExtractGitRemoteInfo(repoPath string) (*RepoInfo, error) {
	// Use current directory if not specified
	if repoPath == "" {
		var err error
		repoPath, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Get git remote URL
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git remote URL: %w", err)
	}

	remoteURL := strings.TrimSpace(string(output))

	// Parse remote URL to extract forge, org, and repo
	// Supports both HTTPS and SSH formats:
	// - https://github.com/kusaridev/pico.git
	// - git@github.com:kusaridev/pico.git
	var rePattern *regexp.Regexp
	if strings.HasPrefix(remoteURL, "git@") {
		// SSH format: git@github.com:kusaridev/pico.git
		rePattern = regexp.MustCompile(`git@([^:]+):([^/]+)/(.+?)(?:\.git)?$`)
	} else {
		// HTTPS format: https://github.com/kusaridev/pico.git
		rePattern = regexp.MustCompile(`https?://([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)
	}

	matches := rePattern.FindStringSubmatch(remoteURL)
	if len(matches) < 4 {
		return nil, fmt.Errorf("failed to parse git remote URL: %s", remoteURL)
	}

	info := &RepoInfo{
		Forge: matches[1],
		Org:   matches[2],
		Repo:  matches[3],
	}

	// Get repository root
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--show-toplevel")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git root: %w", err)
	}
	gitRoot := strings.TrimSpace(string(output))

	// Calculate subrepo_path (relative path from git root to current directory)
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	relPath, err := filepath.Rel(gitRoot, absRepoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// If at git root, use "."
	if relPath == "" || relPath == "." {
		info.SubrepoPath = "."
	} else {
		info.SubrepoPath = relPath
	}

	return info, nil
}
