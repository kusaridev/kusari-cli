// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initDiffRepo creates a git repo with one committed file and chdirs into it,
// wiring up the package-level patch path that generateDiff writes to.
func initDiffRepo(t *testing.T, committed string) string {
	t.Helper()

	repoDir := t.TempDir()
	tempDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})
	require.NoError(t, os.Chdir(repoDir))

	runCmd(t, repoDir, "git", "init")
	runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, repoDir, "git", "config", "user.name", "Test User")
	writeFile(t, filepath.Join(repoDir, "tracked.txt"), committed)
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial commit")

	workingDir = filepath.Join(tempDir, workingDirName)
	require.NoError(t, os.Mkdir(workingDir, 0700))
	patchName = filepath.Join(workingDir, patchFile)

	return repoDir
}

// gitOutput runs a git command in dir and returns trimmed stdout.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err, "git %v", args)
	return strings.TrimSpace(string(out))
}

// generateDiff must intent-to-add untracked files so they land in the diff, then
// remove only those entries. It must not disturb what the developer staged.
func TestGenerateDiff_PreservesStagedChanges(t *testing.T) {
	repoDir := initDiffRepo(t, "original\n")

	// The developer stages a change to a tracked file...
	writeFile(t, filepath.Join(repoDir, "tracked.txt"), "staged edit\n")
	runCmd(t, repoDir, "git", "add", "tracked.txt")
	// ...and leaves an untracked file lying around, which is what triggers the
	// intent-to-add pass.
	writeFile(t, filepath.Join(repoDir, "untracked.txt"), "brand new\n")

	stagedBefore := gitOutput(t, repoDir, "diff", "--cached", "--name-only")
	require.Equal(t, "tracked.txt", stagedBefore, "precondition: change should be staged")

	require.NoError(t, generateDiff("HEAD"))

	// The staged change must survive untouched.
	assert.Equal(t, "tracked.txt", gitOutput(t, repoDir, "diff", "--cached", "--name-only"),
		"generateDiff must not unstage the developer's staged changes")
	assert.Equal(t, "staged edit", gitOutput(t, repoDir, "show", ":tracked.txt"),
		"staged content must be preserved exactly")

	// The intent-to-add entry we created must be cleaned up, leaving the file
	// untracked exactly as we found it.
	assert.Equal(t, "untracked.txt", gitOutput(t, repoDir, "ls-files", "--others", "--exclude-standard"),
		"untracked file should be untracked again after the scan")

	// And the untracked file still has to appear in the generated patch.
	patch, err := os.ReadFile(patchName)
	require.NoError(t, err)
	assert.Contains(t, string(patch), "untracked.txt", "untracked file must be in the diff")
	assert.Contains(t, string(patch), "brand new", "untracked file content must be in the diff")
}

// A partially staged file is the damaging case: hunks staged with `git add -p`
// exist only in the index, so resetting it discards work that cannot be
// reconstructed from the working tree.
func TestGenerateDiff_PreservesPartiallyStagedHunks(t *testing.T) {
	repoDir := initDiffRepo(t, "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\n")

	// Stage only the first hunk by writing that exact intermediate state to the
	// index, which is what `git add -p` leaves behind.
	writeFile(t, filepath.Join(repoDir, "tracked.txt"),
		"CHANGED1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\n")
	runCmd(t, repoDir, "git", "add", "tracked.txt")
	// Then edit a second, unstaged hunk on top.
	writeFile(t, filepath.Join(repoDir, "tracked.txt"),
		"CHANGED1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nCHANGED9\n")

	writeFile(t, filepath.Join(repoDir, "untracked.txt"), "brand new\n")

	indexBefore := gitOutput(t, repoDir, "show", ":tracked.txt")
	require.Contains(t, indexBefore, "CHANGED1")
	require.NotContains(t, indexBefore, "CHANGED9", "precondition: only hunk 1 is staged")

	require.NoError(t, generateDiff("HEAD"))

	indexAfter := gitOutput(t, repoDir, "show", ":tracked.txt")
	assert.Equal(t, indexBefore, indexAfter,
		"partially staged hunks must survive; they cannot be recovered from the working tree")
}

// With nothing staged, the index must come back exactly as clean as it started.
func TestGenerateDiff_LeavesCleanIndexClean(t *testing.T) {
	repoDir := initDiffRepo(t, "original\n")

	writeFile(t, filepath.Join(repoDir, "untracked.txt"), "brand new\n")

	require.NoError(t, generateDiff("HEAD"))

	assert.Empty(t, gitOutput(t, repoDir, "diff", "--cached", "--name-only"),
		"index should be empty again after the scan")
	assert.Equal(t, "untracked.txt", gitOutput(t, repoDir, "ls-files", "--others", "--exclude-standard"),
		"untracked file should still be untracked")
}

// gitSupportsPathspecFromFile reports whether the git on PATH understands
// --pathspec-from-file, which arrived in git 2.25.
func gitSupportsPathspecFromFile(t *testing.T) bool {
	t.Helper()
	// An empty pathspec list is a no-op for a git that knows the flag, and an
	// "unknown option" failure for one that does not.
	cmd := exec.Command("git", "add", "-N", "--pathspec-from-file=-", "--pathspec-file-nul")
	cmd.Stdin = strings.NewReader("")
	return cmd.Run() == nil
}

// A very large untracked file set must not be passed through argv. The limit
// bounds the whole exec, so a repo full of generated files used to fail with
// "argument list too long" before git ran at all.
func TestGenerateDiff_ManyUntrackedFilesExceedingArgMax(t *testing.T) {
	if testing.Short() {
		t.Skip("creates thousands of files; skipped under -short")
	}

	repoDir := initDiffRepo(t, "original\n")

	if !gitSupportsPathspecFromFile(t) {
		// Nothing can succeed here: argv is too long and the stdin fallback does
		// not exist. This git fails either way, as it did before the fallback
		// was added, so there is no behavior to assert.
		t.Skip("git predates --pathspec-from-file (2.25)")
	}

	// Sizing is platform-specific. A Windows command line caps out near 32 KB,
	// so a modest pathspec already forces the stdin path -- and MAX_PATH means
	// the names have to stay short. Unix has to clear ARG_MAX, as much as 2 MiB
	// on Linux, so it uses long names to reach the target without creating an
	// unreasonable number of files.
	targetBytes, segLen := 4<<20, 200
	if runtime.GOOS == "windows" {
		targetBytes, segLen = 96<<10, 24
	}

	dirName := strings.Repeat("g", segLen)
	dir := filepath.Join(repoDir, dirName)
	require.NoError(t, os.MkdirAll(dir, 0755))

	// Keep creating files until the accumulated pathspec actually clears the
	// target, rather than predicting the count from an estimated path length.
	stem := strings.Repeat("f", segLen-16) // leave room for the index suffix

	var argvBytes, files int
	for argvBytes <= targetBytes {
		name := filepath.Join(dir, stem+strconv.Itoa(files)+".go")
		require.NoError(t, os.WriteFile(name, []byte("package pb\n"), 0644))
		rel, err := filepath.Rel(repoDir, name)
		require.NoError(t, err)
		argvBytes += len(rel) + 1
		files++
	}
	t.Logf("%s: %d untracked files, ~%d bytes of pathspec (target %d)",
		runtime.GOOS, files, argvBytes, targetBytes)
	require.Greater(t, argvBytes, targetBytes,
		"test must generate more pathspec than the platform argument limit")

	require.NoError(t, generateDiff("HEAD"), "large untracked file sets must not overflow argv")

	// The index must be returned to its original state, at this scale too.
	assert.Empty(t, gitOutput(t, repoDir, "diff", "--cached", "--name-only"),
		"index should be clean again")
	assert.Len(t, strings.Split(gitOutput(t, repoDir, "ls-files", "--others", "--exclude-standard"), "\n"),
		files, "every generated file should be untracked again")
}

// Filenames containing a newline must survive the NUL-separated pathspec round
// trip rather than being split into two bogus paths.
func TestGenerateDiff_FilenameWithNewline(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Win32 forbids newlines in filenames outright, so this case cannot be
		// constructed there.
		t.Skip("Windows filenames cannot contain newlines")
	}

	repoDir := initDiffRepo(t, "original\n")

	odd := "we\nird.txt"
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, odd), []byte("newline in name\n"), 0644))

	require.NoError(t, generateDiff("HEAD"))

	assert.Empty(t, gitOutput(t, repoDir, "diff", "--cached", "--name-only"),
		"index should be clean again even with a newline in the filename")
	patch, err := os.ReadFile(patchName)
	require.NoError(t, err)
	assert.Contains(t, string(patch), "newline in name", "file content must reach the diff")
}
