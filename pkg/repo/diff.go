package repo

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func generateDiff(dir string, diffCmd []string) error {
	args := []string{"diff"}
	args = append(args, diffCmd...)
	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return fmt.Errorf("failed to run git diff: %w", err)
	}
	if len(output) == 0 {
		return fmt.Errorf("git diff command produced no output: %v", diffCmd)
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
