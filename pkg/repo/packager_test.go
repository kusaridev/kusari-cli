// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"archive/tar"
	"compress/bzip2"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageDirectory(t *testing.T) {
	tests := []struct {
		name             string
		setupRepo        func(t *testing.T, repoDir string)
		full             bool
		expectError      bool
		errorContains    string
		expectedFiles    []string
		notExpectedFiles []string
	}{
		{
			name: "successful packaging with tracked files",
			setupRepo: func(t *testing.T, repoDir string) {
				// Initialize git repo
				runCmd(t, repoDir, "git", "init")
				runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
				runCmd(t, repoDir, "git", "config", "user.name", "Test User")

				// Create files
				writeFile(t, filepath.Join(repoDir, "main.go"), "package main")
				writeFile(t, filepath.Join(repoDir, "README.md"), "# Test")

				// Track files
				runCmd(t, repoDir, "git", "add", ".")
				runCmd(t, repoDir, "git", "commit", "-m", "Initial commit")
			},
			full:          false,
			expectError:   false,
			expectedFiles: []string{"main.go", "README.md"},
		},
		{
			name: "respects gitignore - excludes ignored files",
			setupRepo: func(t *testing.T, repoDir string) {
				runCmd(t, repoDir, "git", "init")
				runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
				runCmd(t, repoDir, "git", "config", "user.name", "Test User")

				// Create .gitignore
				writeFile(t, filepath.Join(repoDir, ".gitignore"), "*.log\nbuild/\n.env")

				// Create tracked files
				writeFile(t, filepath.Join(repoDir, "main.go"), "package main")
				runCmd(t, repoDir, "git", "add", "main.go", ".gitignore")
				runCmd(t, repoDir, "git", "commit", "-m", "Initial commit")

				// Create ignored files (these should NOT be in the tarball)
				writeFile(t, filepath.Join(repoDir, "debug.log"), "log content")
				writeFile(t, filepath.Join(repoDir, ".env"), "SECRET=value")
				if err := os.Mkdir(filepath.Join(repoDir, "build"), 0755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(repoDir, "build", "output.bin"), "binary")
			},
			full:             false,
			expectError:      false,
			expectedFiles:    []string{"main.go", ".gitignore"},
			notExpectedFiles: []string{"debug.log", ".env", "build/output.bin"},
		},
		{
			name: "includes untracked files not in gitignore",
			setupRepo: func(t *testing.T, repoDir string) {
				runCmd(t, repoDir, "git", "init")
				runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
				runCmd(t, repoDir, "git", "config", "user.name", "Test User")

				// Create .gitignore
				writeFile(t, filepath.Join(repoDir, ".gitignore"), "*.log")

				// Create tracked file
				writeFile(t, filepath.Join(repoDir, "main.go"), "package main")
				runCmd(t, repoDir, "git", "add", "main.go", ".gitignore")
				runCmd(t, repoDir, "git", "commit", "-m", "Initial commit")

				// Create untracked file that's NOT ignored (should be included)
				writeFile(t, filepath.Join(repoDir, "new-file.go"), "package main")

				// Create untracked file that IS ignored (should NOT be included)
				writeFile(t, filepath.Join(repoDir, "debug.log"), "logs")
			},
			full:             false,
			expectError:      false,
			expectedFiles:    []string{"main.go", ".gitignore", "new-file.go"},
			notExpectedFiles: []string{"debug.log"},
		},
		{
			name: "excludes .git directory",
			setupRepo: func(t *testing.T, repoDir string) {
				runCmd(t, repoDir, "git", "init")
				runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
				runCmd(t, repoDir, "git", "config", "user.name", "Test User")

				writeFile(t, filepath.Join(repoDir, "main.go"), "package main")
				runCmd(t, repoDir, "git", "add", ".")
				runCmd(t, repoDir, "git", "commit", "-m", "Initial commit")
			},
			full:             false,
			expectError:      false,
			expectedFiles:    []string{"main.go"},
			notExpectedFiles: []string{".git/config", ".git/HEAD"},
		},
		{
			name: "handles nested directories",
			setupRepo: func(t *testing.T, repoDir string) {
				runCmd(t, repoDir, "git", "init")
				runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
				runCmd(t, repoDir, "git", "config", "user.name", "Test User")

				// Create nested structure
				if err := os.MkdirAll(filepath.Join(repoDir, "pkg", "repo"), 0755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(repoDir, "pkg", "repo", "packager.go"), "package repo")
				writeFile(t, filepath.Join(repoDir, "main.go"), "package main")

				runCmd(t, repoDir, "git", "add", ".")
				runCmd(t, repoDir, "git", "commit", "-m", "Initial commit")
			},
			full:          false,
			expectError:   false,
			expectedFiles: []string{"main.go", "pkg/repo/packager.go"},
		},
		{
			name: "full scan mode",
			setupRepo: func(t *testing.T, repoDir string) {
				runCmd(t, repoDir, "git", "init")
				runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
				runCmd(t, repoDir, "git", "config", "user.name", "Test User")

				writeFile(t, filepath.Join(repoDir, "main.go"), "package main")
				runCmd(t, repoDir, "git", "add", ".")
				runCmd(t, repoDir, "git", "commit", "-m", "Initial commit")
			},
			full:          true,
			expectError:   false,
			expectedFiles: []string{"main.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for the test repo
			repoDir := t.TempDir()

			// Create temporary directory for tarball output
			tempDir := t.TempDir()
			tarballDir = tempDir
			workingDir = filepath.Join(tempDir, workingDirName)

			// Setup the test repository
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Logf("Failed to restore directory: %v", err)
				}
			}()

			if err := os.Chdir(repoDir); err != nil {
				t.Fatal(err)
			}

			tt.setupRepo(t, repoDir)

			// Create working directory and metadata files
			if err := os.Mkdir(workingDir, 0700); err != nil {
				t.Fatal(err)
			}
			metaName = filepath.Join(workingDir, metaFile)
			patchName = filepath.Join(workingDir, patchFile)

			// Create dummy meta and patch files
			writeFile(t, metaName, `{"test": "meta"}`)
			if !tt.full {
				writeFile(t, patchName, "patch content")
			}

			// Execute packageDirectory
			size, err := packageDirectory(tt.full)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error containing '%s', got nil", tt.errorContains)
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Fatalf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if size <= 0 {
				t.Errorf("Expected positive size, got %d", size)
			}

			// Verify the tarball was created and compressed
			tarballPath := filepath.Join(tarballDir, tarballName)
			if _, err := os.Stat(tarballPath); err != nil {
				t.Fatalf("Tarball not created: %v", err)
			}

			// Extract and verify tarball contents
			filesInTarball := extractTarballContents(t, tarballPath)

			// Check expected files are present
			for _, expectedFile := range tt.expectedFiles {
				if !containsFile(filesInTarball, expectedFile) {
					t.Errorf("Expected file '%s' not found in tarball. Found: %v", expectedFile, filesInTarball)
				}
			}

			// Check that not-expected files are absent
			for _, notExpectedFile := range tt.notExpectedFiles {
				if containsFile(filesInTarball, notExpectedFile) {
					t.Errorf("Unexpected file '%s' found in tarball. Should have been excluded.", notExpectedFile)
				}
			}

			// Verify metadata files are in tarball
			// Note: metadata files are appended with just their base names
			if !containsFile(filesInTarball, metaFile) {
				t.Errorf("Meta file not found in tarball. Files: %v", filesInTarball)
			}
			if !tt.full && !containsFile(filesInTarball, patchFile) {
				t.Errorf("Patch file not found in tarball (diff mode). Files: %v", filesInTarball)
			}
		})
	}
}

func TestPackageDirectory_NonGitRepo(t *testing.T) {
	// Create temporary directory that's NOT a git repo
	repoDir := t.TempDir()
	tempDir := t.TempDir()
	tarballDir = tempDir
	workingDir = filepath.Join(tempDir, workingDirName)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()

	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	// Create a file
	writeFile(t, filepath.Join(repoDir, "test.txt"), "content")

	// Try to package - should fail because it's not a git repo
	_, err = packageDirectory(false)
	if err == nil {
		t.Error("Expected error when packaging non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "error getting git files list") {
		t.Errorf("Expected 'error getting git files list', got: %v", err)
	}
}

// Helper functions

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Command '%s %v' failed: %v\nOutput: %s", name, args, err, output)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

func extractTarballContents(t *testing.T, tarballPath string) []string {
	t.Helper()

	// Open the compressed file
	f, err := os.Open(tarballPath)
	if err != nil {
		t.Fatalf("Failed to open tarball: %v", err)
	}
	defer func() {
		_ = f.Close()
	}()

	// Decompress bzip2
	bzr := bzip2.NewReader(f)

	// Read tar contents
	tr := tar.NewReader(bzr)
	var files []string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read tar: %v", err)
		}

		// Only record files, not directories
		if header.Typeflag == tar.TypeReg {
			files = append(files, header.Name)
		}
	}

	return files
}

func containsFile(files []string, target string) bool {
	for _, f := range files {
		if f == target {
			return true
		}
	}
	return false
}
