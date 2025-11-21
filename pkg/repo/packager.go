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
func packageDirectory(full bool) error {
	if err := os.Mkdir(tarballDir, 0700); err != nil {
		if !errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("failed to make Kusari directory: %w", err)
		}
	}
	outFile := filepath.Join(tarballDir, tarballNameUncompressed)
	// Write the repo contents to the tarball, uncompressed so that we can append to it
	// tar -cf ./kusari-archive/kusari-inspector.tar.bz2 --dereference --exclude=.git .
	tc := exec.Command("tar", "-cf", outFile, "--dereference", "--exclude=.git", ".")
	tc.Env = append(tc.Env, "COPYFILE_DISABLE=1")
	if err := tc.Run(); err != nil {
		return fmt.Errorf("error taring source code: %w", err)
	}
	// Append our Inspector files
	args := []string{"-C", workingDir, "--append", "-f", outFile, metaFile}
	if !full {
		args = append(args, patchFile)
	}

	if err := exec.Command("tar", args...).Run(); err != nil {
		return fmt.Errorf("error tarring Inspector metadata: %w", err)
	}
	// Compress it
	if err := exec.Command("bzip2", outFile).Run(); err != nil {
		return fmt.Errorf("error compressing file: %w", err)
	}

	return nil
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
