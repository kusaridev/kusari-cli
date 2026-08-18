// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// binarySniffLen mirrors git's own heuristic: git reads the first 8000 bytes of
// a file and calls it binary if any of them is NUL.
const binarySniffLen = 8000

// isBinaryFile reports whether path looks binary by git's definition.
func isBinaryFile(path string) (bool, error) {
	f, err := os.Open(filepath.FromSlash(path))
	if err != nil {
		return false, err
	}
	defer func() {
		_ = f.Close()
	}()

	buf := make([]byte, binarySniffLen)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, err
	}
	return bytes.IndexByte(buf[:n], 0) >= 0, nil
}

func generateDiff(rev string) error {
	if err := validateRev(rev); err != nil {
		return err
	}

	// Untracked files are invisible to git diff until the index knows about
	// them, so they need an intent-to-add entry first.
	untracked, err := gitListFiles("--others", "--exclude-standard")
	if err != nil {
		return fmt.Errorf("failed to list untracked files: %w", err)
	}

	// Binary files are held back from that pass. git diff --binary inlines a
	// binary file into the patch as base85, so a single stray build artifact
	// dominates everything real: a ~30 MB executable becomes a ~37 MB patch and
	// roughly 19 million tokens, which the analyzer rejects outright with a
	// message that names none of this. Nothing downstream reads a binary diff,
	// so the only thing lost is the failure.
	var addable, skipped []string
	for _, f := range untracked {
		binary, err := isBinaryFile(f)
		if err != nil {
			// Vanished or unreadable between listing and now; it would not have
			// produced a usable diff either.
			continue
		}
		if binary {
			skipped = append(skipped, f)
			continue
		}
		addable = append(addable, f)
	}
	if len(skipped) > 0 {
		fmt.Fprintf(os.Stderr, "Excluding %d untracked binary file(s) from the diff: %s\n",
			len(skipped), summarizeFiles(skipped))
	}

	var hasUntrackedFiles bool
	if len(addable) > 0 {
		hasUntrackedFiles = true
		// Use git add -N (intent-to-add, no content staged) so the diff shows
		// these as new files.
		args := append([]string{"add", "-N", "--"}, addable...)
		if err := exec.Command("git", args...).Run(); err != nil {
			return fmt.Errorf("failed to add untracked files to index: %w", err)
		}
		// Ensure we reset the index afterward.
		defer func() {
			_ = exec.Command("git", "reset", "--").Run()
		}()
	}

	// Generate diff including both tracked and untracked files.
	output, err := exec.Command("git", "diff", "--binary", rev).Output()
	if err != nil {
		return fmt.Errorf("failed to run git diff: %w", err)
	}
	if len(output) == 0 && !hasUntrackedFiles {
		if len(skipped) > 0 {
			return fmt.Errorf("git diff produced no output against %v: the only changes are "+
				"binary files, which are not scanned. Add them to .gitignore if they are build artifacts", rev)
		}
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

// summarizeFiles renders a short, bounded list for a warning; a repository with
// hundreds of untracked binaries should not flood the terminal.
func summarizeFiles(files []string) string {
	const max = 5
	if len(files) <= max {
		return strings.Join(files, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(files[:max], ", "), len(files)-max)
}

func validateRev(rev string) error {
	if err := exec.Command("git", "rev-parse", "--verify", "--quiet", "--end-of-options", rev).Run(); err != nil {
		return fmt.Errorf("not a valid git rev: %w, %v", err, rev)
	}
	return nil
}
