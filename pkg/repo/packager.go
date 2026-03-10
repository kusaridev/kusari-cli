// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kusaridev/kusari-cli/api"
)

// PackageDirectory creates a zip file from a directory
func packageDirectory(full bool) (int64, error) {
	if err := os.Mkdir(tarballDir, 0700); err != nil {
		if !errors.Is(err, syscall.EEXIST) {
			return 0, fmt.Errorf("failed to make Kusari directory: %w", err)
		}
	}
	outFile := filepath.Join(tarballDir, tarballNameUncompressed)

	// Get list of files from git (respects .gitignore)
	// This includes tracked files and untracked files that aren't in .gitignore
	filesListPath := filepath.Join(tarballDir, "files.txt")
	defer func() {
		_ = os.Remove(filesListPath)
	}()

	// Get tracked files and untracked files (excluding .gitignore entries)
	gitCmd := exec.Command("sh", "-c", "git ls-files && git ls-files --others --exclude-standard")
	filesOutput, err := gitCmd.Output()
	if err != nil {
		return 0, fmt.Errorf("error getting git files list: %w", err)
	}

	// Write file list to a temporary file
	if err := os.WriteFile(filesListPath, filesOutput, 0600); err != nil {
		return 0, fmt.Errorf("error writing files list: %w", err)
	}

	// Write the repo contents to the tarball, uncompressed so that we can append to it
	// Use -T to specify files from list (respects .gitignore)
	tc := exec.Command("tar", "-cf", outFile, "--dereference", "-T", filesListPath)
	tc.Env = append(tc.Env, "COPYFILE_DISABLE=1")
	if err := tc.Run(); err != nil {
		return 0, fmt.Errorf("error taring source code: %w", err)
	}
	// Append our Inspector files
	args := []string{"-C", workingDir, "--append", "-f", outFile, metaFile}
	if !full {
		args = append(args, patchFile)
	}

	if err := exec.Command("tar", args...).Run(); err != nil {
		return 0, fmt.Errorf("error tarring Inspector metadata: %w", err)
	}
	// Compress it
	if err := exec.Command("bzip2", outFile).Run(); err != nil {
		return 0, fmt.Errorf("error compressing file: %w", err)
	}

	fi, err := os.Stat(outFile + ".bz2")
	if err != nil {
		return 0, fmt.Errorf("error stating file: %w", err)
	}

	return fi.Size(), nil
}

func createMeta(rev string, full bool) (*api.BundleMeta, error) {
	repoDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo directory: %w", err)
	}

	branch, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run git rev-parse: %w", err)
	}
	if len(branch) == 0 {
		return nil, fmt.Errorf("git rev-parse command produced no output")
	}

	remote, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		// Probably just a local git repo
		remote = []byte{}
	}

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

	// Get list of changed files for incremental scanning support
	var changedFiles []string
	if !full && rev != "" {
		// For diff scans, get the list of files that changed (tracked files)
		diffOutput, err := exec.Command("git", "diff", "--name-only", rev).Output()
		if err == nil && len(diffOutput) > 0 {
			files := strings.SplitSeq(strings.TrimSpace(string(diffOutput)), "\n")
			for f := range files {
				if f != "" {
					changedFiles = append(changedFiles, f)
				}
			}
		}

		// Also include untracked files (new files not yet added to git)
		untrackedOutput, err := exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
		if err == nil && len(untrackedOutput) > 0 {
			files := strings.SplitSeq(strings.TrimSpace(string(untrackedOutput)), "\n")
			for f := range files {
				if f != "" {
					changedFiles = append(changedFiles, f)
				}
			}
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
