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
func packageDirectory() error {
	if err := os.Mkdir(tarballDir, 0700); err != nil {
		if !errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("failed to make Kusari directory: %w", err)
		}
	}
	outFile := filepath.Join(tarballDir, tarballName)
	excludePath1 := fmt.Sprintf("./%s", tarballDir)
	excludePath2 := "./.git"

	// Use find to only include regular files and directories, excluding symlinks
	cmd := fmt.Sprintf(`find . \( -path "%s" -o -path "%s" \) -prune -o \( -type f -o -type d \) -print | tar -jcf "%s" -T -`,
		excludePath1, excludePath2, outFile)

	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		return fmt.Errorf("error taring source code: %w", err)
	}

	return nil
}

func createMeta(diffCmd []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	branch, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("failed to run git rev-parse: %w", err)
	}
	if len(branch) == 0 {
		return fmt.Errorf("git rev-parse command produced no output: %v", diffCmd)
	}

	remote, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return fmt.Errorf("failed to run git remote: %w", err)
	}
	if len(remote) == 0 {
		return fmt.Errorf("git remote command produced no output: %v", diffCmd)
	}

	status, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return fmt.Errorf("failed to run git status: %w", err)
	}

	meta := &api.BundleMeta{
		PatchName:     patchName,
		CurrentBranch: strings.TrimSpace(string(branch)),
		DirName:       filepath.Base(wd),
		DiffCmd:       strings.Join(diffCmd, " "),
		Remote:        strings.TrimSpace(string(remote)),
		GitDirty:      len(status) != 0,
	}

	metab, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal json meta: %w", err)
	}

	f, err := os.Create(metaName)
	if err != nil {
		return fmt.Errorf("failed to open meta file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	if _, err := io.Copy(f, bytes.NewReader(metab)); err != nil {
		return fmt.Errorf("failed to write meta file: %w", err)
	}

	return nil
}
