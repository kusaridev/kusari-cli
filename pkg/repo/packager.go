// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
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

	meta := &api.BundleMeta{
		PatchName:     patchName,
		CurrentBranch: strings.TrimSpace(string(branch)),
		DirName:       filepath.Base(repoDir),
		DiffCmd:       rev,
		Remote:        strings.TrimSpace(string(remote)),
		GitDirty:      len(status) != 0,
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
