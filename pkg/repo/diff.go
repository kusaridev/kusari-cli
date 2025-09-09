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
	output, err := exec.Command("git", "diff", "--binary", rev).Output()
	if err != nil {
		return fmt.Errorf("failed to run git diff: %w", err)
	}
	if len(output) == 0 {
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
