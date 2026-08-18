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

// gitPathspec runs a git subcommand over a list of paths.
//
// The paths go in argv, which every git version supports. But the argument list
// has a hard platform limit -- ARG_MAX is 1 MiB on macOS and typically 2 MiB on
// Linux, while a Windows command line caps out around 32 KB -- and exceeding it
// fails the spawn before git runs at all. A repository full of generated or
// vendored files clears that limit easily, especially on Windows.
//
// So argv is tried first and the path list only moves to stdin if the spawn
// itself failed. That keeps the common case byte-identical to what shipped
// before, including on git older than 2.25, where --pathspec-from-file does not
// exist yet: anything that used to work still takes the argv path. The retry
// only ever runs where the old code was guaranteed to fail outright.
//
// The two failure modes are distinguishable: git rejecting the command exits
// nonzero and yields an *exec.ExitError, whereas a spawn that never happened
// yields an *fs.PathError.
func gitPathspec(paths []string, args ...string) error {
	argv := make([]string, 0, len(args)+1+len(paths))
	argv = append(argv, args...)
	argv = append(argv, "--")
	argv = append(argv, paths...)

	err := runGitPathspec(nil, argv...)
	var exitErr *exec.ExitError
	if err == nil || errors.As(err, &exitErr) {
		// Either it worked, or git itself objected and a retry would not help.
		return err
	}

	// The spawn never happened -- almost certainly the argument list length.
	// Feed the pathspec on stdin instead. NUL separation additionally keeps a
	// newline in a filename from splitting one path into two.
	stdin := strings.NewReader(strings.Join(paths, "\x00") + "\x00")
	return runGitPathspec(stdin, append(args, "--pathspec-from-file=-", "--pathspec-file-nul")...)
}

// runGitPathspec runs git with the given args, optionally piping stdin, and
// folds git's stderr into the returned error so the caller can report it.
func runGitPathspec(stdin io.Reader, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
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
		if err := gitPathspec(addable, "add", "-N"); err != nil {
			return fmt.Errorf("failed to add untracked files to index: %w", err)
		}
		// Undo only the entries we just added. A pathspec-less `git reset`
		// resets the whole index to HEAD, silently discarding whatever the
		// developer had staged. Working tree content survives, but the staging
		// itself does not -- and partial hunks from `git add -p` cannot be
		// reconstructed from the working tree, so that is real lost work.
		defer func() {
			if err := gitPathspec(addable, "reset", "--quiet"); err != nil {
				fmt.Fprintf(os.Stderr,
					"Warning: failed to remove %d intent-to-add index entry/entries: %v\n"+
						"Your staged changes are untouched; run 'git reset -- %s' to clean up.\n",
					len(addable), err, summarizeFiles(addable))
			}
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
