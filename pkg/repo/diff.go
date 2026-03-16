// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func generateDiff(rev string) error {
	if err := validateRev(rev); err != nil {
		return err
	}

	// First, get list of untracked files (not in .gitignore)
	untrackedOutput, err := exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return fmt.Errorf("failed to list untracked files: %w", err)
	}

	// Use git add -N to add untracked files to index (intent-to-add, no content staging)
	// This makes git diff show them as new files
	var hasUntrackedFiles bool
	if len(bytes.TrimSpace(untrackedOutput)) > 0 {
		hasUntrackedFiles = true
		// Split untracked files by newline and add each one
		untrackedFiles := bytes.Split(bytes.TrimSpace(untrackedOutput), []byte("\n"))
		args := []string{"add", "-N", "--"}
		for _, file := range untrackedFiles {
			if len(file) > 0 {
				args = append(args, string(file))
			}
		}
		addCmd := exec.Command("git", args...)
		if err := addCmd.Run(); err != nil {
			return fmt.Errorf("failed to add untracked files to index: %w", err)
		}
		// Ensure we reset the index afterward
		defer func() {
			_ = exec.Command("git", "reset", "--").Run()
		}()
	}

	// Generate diff including both tracked and untracked files
	output, err := exec.Command("git", "diff", "--binary", rev).Output()
	if err != nil {
		return fmt.Errorf("failed to run git diff: %w", err)
	}
	if len(output) == 0 && !hasUntrackedFiles {
		return fmt.Errorf("git diff command produced no output: git diff %v", rev)
	}

	f, err := os.Create(patchName)
	if err != nil {
		return fmt.Errorf("failed to open patch file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	if _, err := io.Copy(f, bytes.NewReader(output)); err != nil {
		return fmt.Errorf("failed to write patch file: %w", err)
	}
	return nil
}

func validateRev(rev string) error {
	if err := exec.Command("git", "rev-parse", "--verify", "--quiet", "--end-of-options", rev).Run(); err != nil {
		return fmt.Errorf("not a valid git rev: %w, %v", err, rev)
	}
	return nil
}
