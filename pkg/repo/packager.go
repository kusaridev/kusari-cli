// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kusaridev/kusari-cli/v2/api"
)

// packageDirectory builds the bzip2-compressed tarball we upload, from the
// current working directory, and returns its size in bytes.
func packageDirectory(full bool) (int64, error) {
	// The caller already created this directory; MkdirAll makes that a no-op
	// rather than a platform-specific "already exists" errno to special-case.
	if err := os.MkdirAll(tarballDir, 0700); err != nil {
		return 0, fmt.Errorf("failed to make Kusari directory: %w", err)
	}

	// Get list of files from git (respects .gitignore).
	// This includes tracked files and untracked files that aren't in .gitignore.
	files, err := listRepoFiles()
	if err != nil {
		return 0, fmt.Errorf("error getting git files list: %w", err)
	}

	// The Inspector files go in FIRST so they own their names at the root of the
	// archive. The archiver keeps the first entry written for a given name, and
	// a repository is perfectly capable of containing a file called
	// kusari-inspector.json: were it to win, the backend would parse the repo's
	// file as bundle metadata. Valid JSON would unmarshal into an empty
	// BundleMeta and the scan would proceed against meaningless values rather
	// than failing outright.
	reserved := map[string]bool{metaFile: true}
	entries := make([]archiveEntry, 0, len(files)+2)
	entries = append(entries, archiveEntry{name: metaFile, path: filepath.Join(workingDir, metaFile)})
	if !full {
		reserved[patchFile] = true
		entries = append(entries, archiveEntry{name: patchFile, path: filepath.Join(workingDir, patchFile)})
	}

	for _, f := range files {
		if reserved[f] {
			fmt.Fprintf(os.Stderr,
				"Warning: %s is reserved for Kusari Inspector metadata; the repository's own copy is excluded from this scan\n", f)
			continue
		}
		// git reports slash-separated, repo-relative paths on every platform;
		// they are already valid tar names.
		entries = append(entries, archiveEntry{name: f, path: filepath.FromSlash(f)})
	}

	// The caller has already chdir'd to the repository root, so "." is it.
	// Resolve it once so the archiver can tell whether a symlinked directory
	// stays inside the repository by comparing real locations.
	//
	// Absolute first, then EvalSymlinks -- the same order the archiver applies
	// to every path it compares against this one. The order matters on Windows:
	// EvalSymlinks canonicalises 8.3 short names to their long form, so a root
	// that skipped it would sit at C:\Users\RUNNER~1\... while its own children
	// resolved to C:\Users\runneradmin\..., and every one of them would look
	// like it lived outside the repository.
	root, err := filepath.Abs(".")
	if err != nil {
		return 0, fmt.Errorf("error resolving repository root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return 0, fmt.Errorf("error resolving repository root: %w", err)
	}

	outFile := filepath.Join(tarballDir, tarballName)
	if err := writeTarBz2(outFile, root, entries); err != nil {
		return 0, err
	}

	fi, err := os.Stat(outFile)
	if err != nil {
		return 0, fmt.Errorf("error stating file: %w", err)
	}

	return fi.Size(), nil
}

// listRepoFiles returns every file git considers part of the working tree:
// tracked files plus untracked files that .gitignore doesn't exclude.
func listRepoFiles() ([]string, error) {
	tracked, err := gitListFiles()
	if err != nil {
		return nil, err
	}
	untracked, err := gitListFiles("--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return append(tracked, untracked...), nil
}

// gitListFiles runs `git ls-files -z` with the given extra arguments. The -z
// form is NUL-delimited and unquoted, so paths containing spaces, newlines or
// non-ASCII characters survive intact regardless of core.quotePath.
func gitListFiles(extraArgs ...string) ([]string, error) {
	return gitNulFields(append([]string{"ls-files", "-z"}, extraArgs...)...)
}

// gitNulFields runs a git command whose output is NUL-delimited (the -z form of
// ls-files, diff, status, ...) and returns its non-empty fields. Every path git
// hands us must come through here or an equivalent -z listing: with the default
// core.quotePath, git C-quotes any path containing non-ASCII bytes
// ("...10.45.37\342\200\257AM.png"), and that quoted string never matches a
// real file on disk.
func gitNulFields(args ...string) ([]string, error) {
	out, err := runGit(args...)
	if err != nil {
		return nil, err
	}

	fields := strings.Split(string(out), "\x00")
	files := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// runGit runs a git command and returns its stdout, folding git's stderr into
// the error so failures are diagnosable instead of a bare "exit status 128".
func runGit(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// sanitizeRemoteURL strips any embedded credentials from a git remote URL so
// that CI-injected tokens (gitlab-ci-token, GitHub x-access-token/oauth2/PAT,
// etc.) never end up in the bundle metadata. Secrets live in the userinfo
// component (user:pass@host) of a URL, which is handled as follows:
//   - A password is always a secret, so it is stripped on every scheme.
//   - A lone username is ambiguous: on http(s) it is frequently the token
//     itself (e.g. https://<PAT>@github.com), so it is stripped; on other
//     schemes (ssh, git, ...) it is a login name (e.g. "git") guarded by
//     key-based auth, not a secret, so it is kept.
func sanitizeRemoteURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// scp-style SSH (git@host:path) has no URL scheme and uses key-based auth.
	if !strings.Contains(raw, "://") {
		return raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		// A value with embedded credentials that won't parse: drop it rather
		// than risk leaking a token.
		return ""
	}

	if u.User == nil {
		return u.String()
	}

	switch u.Scheme {
	case "http", "https":
		// On http(s) even a lone username is commonly a token itself, so strip
		// the whole userinfo component.
		u.User = nil
	default:
		// Other schemes use key-based auth; a bare username is a login name,
		// not a secret. Only the password (if any) needs to be stripped.
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.User(u.User.Username())
		}
	}
	return u.String()
}

func createMeta(rev string, full bool, overrideBranch string) (*api.BundleMeta, error) {
	repoDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo directory: %w", err)
	}

	var branch []byte
	if overrideBranch != "" {
		branch = []byte(overrideBranch)
	} else {
		var err error
		branch, err = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
		if err != nil {
			return nil, fmt.Errorf("failed to run git rev-parse: %w", err)
		}
		if len(branch) == 0 {
			return nil, fmt.Errorf("git rev-parse command produced no output")
		}
	}

	remote, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		// Probably just a local git repo
		remote = []byte{}
	}
	// Strip any embedded credentials (e.g. gitlab-ci-token, GitHub
	// x-access-token / PAT) that CI systems bake into the remote URL.
	remote = []byte(sanitizeRemoteURL(string(remote)))

	status, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run git status: %w", err)
	}

	// Get current commit SHA for incremental scanning support
	commitSHA, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		// Non-fatal: commit SHA is optional for incremental scanning
		commitSHA = []byte{}
	}

	// Get list of changed files for incremental scanning support.
	//
	// Both listings are NUL-delimited: a quoted path would not match anything on
	// disk, so computeFileHash below would silently skip it (its error is
	// swallowed) and the server would receive a changed-files list that
	// disagrees with the tarball. Non-fatal on error, as before — incremental
	// scanning is an optimisation, not a requirement.
	var changedFiles []string
	if !full && rev != "" {
		// For diff scans, get the list of files that changed (tracked files)
		if files, err := gitNulFields("diff", "-z", "--name-only", rev); err == nil {
			changedFiles = append(changedFiles, files...)
		}

		// Also include untracked files (new files not yet added to git)
		if files, err := gitListFiles("--others", "--exclude-standard"); err == nil {
			changedFiles = append(changedFiles, files...)
		}
	}

	// Compute content hashes for changed files (for incremental scanning)
	changedFileHashes := make(map[string]string)
	for _, file := range changedFiles {
		hash, err := computeFileHash(file)
		if err != nil {
			// Skip files that can't be hashed (deleted, binary, etc.)
			continue
		}
		changedFileHashes[file] = hash
	}

	meta := &api.BundleMeta{
		PatchName:         patchName,
		CurrentBranch:     strings.TrimSpace(string(branch)),
		DirName:           filepath.Base(repoDir),
		DiffCmd:           rev,
		Remote:            strings.TrimSpace(string(remote)),
		GitDirty:          len(status) != 0,
		CommitSHA:         strings.TrimSpace(string(commitSHA)),
		ChangedFiles:      changedFiles,
		ChangedFileHashes: changedFileHashes,
	}
	if full {
		meta.ScanType = "full"
	} else {
		meta.ScanType = "diff"
	}

	metab, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json meta: %w", err)
	}

	f, err := os.Create(metaName)
	if err != nil {
		return nil, fmt.Errorf("failed to open meta file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	if _, err := io.Copy(f, bytes.NewReader(metab)); err != nil {
		return nil, fmt.Errorf("failed to write meta file: %w", err)
	}

	return meta, nil
}

// computeFileHash computes SHA256 hash of a file's contents
func computeFileHash(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}
