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
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateMeta_OverrideBranch(t *testing.T) {
	tests := []struct {
		name           string
		overrideBranch string
		wantBranch     string // empty means "expect git-detected branch (not HEAD)"
	}{
		{
			name:           "uses override branch when provided",
			overrideBranch: "feature/my-pr-branch",
			wantBranch:     "feature/my-pr-branch",
		},
		{
			name:           "uses override branch simulating detached HEAD in CI",
			overrideBranch: "main",
			wantBranch:     "main",
		},
		{
			name:           "falls back to git when override is empty",
			overrideBranch: "",
			wantBranch:     "", // checked separately: must be non-empty and not "HEAD"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := t.TempDir()
			tempDir := t.TempDir()

			originalDir, err := os.Getwd()
			require.NoError(t, err)
			defer func() {
				_ = os.Chdir(originalDir)
			}()

			require.NoError(t, os.Chdir(repoDir))

			runCmd(t, repoDir, "git", "init")
			runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
			runCmd(t, repoDir, "git", "config", "user.name", "Test User")
			writeFile(t, filepath.Join(repoDir, "test.txt"), "content")
			runCmd(t, repoDir, "git", "add", ".")
			runCmd(t, repoDir, "git", "commit", "-m", "initial commit")

			tarballDir = tempDir
			workingDir = filepath.Join(tempDir, workingDirName)
			require.NoError(t, os.Mkdir(workingDir, 0700))
			metaName = filepath.Join(workingDir, metaFile)
			patchName = filepath.Join(workingDir, patchFile)

			meta, err := createMeta("HEAD", false, tt.overrideBranch)
			require.NoError(t, err)

			if tt.wantBranch != "" {
				assert.Equal(t, tt.wantBranch, meta.CurrentBranch)
			} else {
				// git-detected branch: must be non-empty and not "HEAD"
				assert.NotEmpty(t, meta.CurrentBranch)
				assert.NotEqual(t, "HEAD", meta.CurrentBranch)
			}
		})
	}
}

func TestSanitizeRemoteURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "strips gitlab-ci-token credentials",
			raw:  "https://gitlab-ci-token:secret-token@gitlab.com/group/project.git",
			want: "https://gitlab.com/group/project.git",
		},
		{
			name: "strips github x-access-token credentials",
			raw:  "https://x-access-token:ghp_secret@github.com/owner/repo.git",
			want: "https://github.com/owner/repo.git",
		},
		{
			name: "strips github oauth2 credentials",
			raw:  "https://oauth2:secret@github.com/owner/repo.git",
			want: "https://github.com/owner/repo.git",
		},
		{
			name: "strips user:pat credentials",
			raw:  "https://user:ghp_secret@github.com/owner/repo.git",
			want: "https://github.com/owner/repo.git",
		},
		{
			name: "strips username-only userinfo",
			raw:  "https://token@github.com/owner/repo.git",
			want: "https://github.com/owner/repo.git",
		},
		{
			name: "leaves clean https URL unchanged",
			raw:  "https://github.com/owner/repo.git",
			want: "https://github.com/owner/repo.git",
		},
		{
			name: "leaves scp-style ssh remote unchanged",
			raw:  "git@github.com:owner/repo.git",
			want: "git@github.com:owner/repo.git",
		},
		{
			name: "leaves ssh:// URL unchanged (login user, not a secret)",
			raw:  "ssh://git@github.com/owner/repo.git",
			want: "ssh://git@github.com/owner/repo.git",
		},
		{
			name: "leaves git:// URL unchanged",
			raw:  "git://github.com/owner/repo.git",
			want: "git://github.com/owner/repo.git",
		},
		{
			name: "strips password from ssh:// URL but keeps login user",
			raw:  "ssh://git:secret@github.com/owner/repo.git",
			want: "ssh://git@github.com/owner/repo.git",
		},
		{
			name: "strips password from git:// URL but keeps login user",
			raw:  "git://user:secret@github.com/owner/repo.git",
			want: "git://user@github.com/owner/repo.git",
		},
		{
			name: "trims surrounding whitespace from git output",
			raw:  "https://gitlab-ci-token:secret@gitlab.com/group/project.git\n",
			want: "https://gitlab.com/group/project.git",
		},
		{
			name: "returns empty for empty input",
			raw:  "",
			want: "",
		},
		{
			name: "returns empty for whitespace-only input",
			raw:  "   \n",
			want: "",
		},
		{
			name: "drops unparseable URL rather than leak it",
			raw:  "https://gitlab-ci-token:secret@gitlab.com:notaport/project.git",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeRemoteURL(tt.raw))
		})
	}
}

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

// setupPackagerTest wires the package-level globals packageDirectory reads,
// chdirs into a fresh git repo, and returns its path. Cleanup is registered on
// t, so callers just use the returned directory.
func setupPackagerTest(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	tempDir := t.TempDir()
	// Note that tempDir already exists: scan() creates it with os.MkdirTemp
	// before calling packageDirectory, so "the output directory is already
	// there" is the normal case, not an edge case.
	tarballDir = tempDir
	workingDir = filepath.Join(tempDir, workingDirName)
	require.NoError(t, os.Mkdir(workingDir, 0700))
	metaName = filepath.Join(workingDir, metaFile)
	patchName = filepath.Join(workingDir, patchFile)
	writeFile(t, metaName, `{"test": "meta"}`)
	writeFile(t, patchName, "patch content")

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})
	require.NoError(t, os.Chdir(repoDir))

	runCmd(t, repoDir, "git", "init")
	runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, repoDir, "git", "config", "user.name", "Test User")

	return repoDir
}

// TestPackageDirectory_UnusualFilenames covers paths that git quotes by default
// (core.quotePath), which the old files.txt + `tar -T` pipeline could not
// round-trip.
func TestPackageDirectory_UnusualFilenames(t *testing.T) {
	repoDir := setupPackagerTest(t)

	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "deep", "nest"), 0755))
	writeFile(t, filepath.Join(repoDir, "deep", "nest", "a b c.txt"), "spaces")
	writeFile(t, filepath.Join(repoDir, "deep", "nest", "üñîçø∂é.txt"), "unicode")
	writeFile(t, filepath.Join(repoDir, "dollar$sign.txt"), "shell metacharacter")
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "Initial commit")

	_, err := packageDirectory(false)
	require.NoError(t, err)

	files := extractTarballContents(t, filepath.Join(tarballDir, tarballName))
	for _, want := range []string{
		"deep/nest/a b c.txt",
		"deep/nest/üñîçø∂é.txt",
		"dollar$sign.txt",
	} {
		assert.True(t, containsFile(files, want), "%q missing from tarball; got %v", want, files)
	}
}

// TestPackageDirectory_DereferencesSymlinks pins the behavior `tar --dereference`
// used to provide: a symlink is archived as a copy of its target's contents.
func TestPackageDirectory_DereferencesSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on windows requires developer mode or elevation")
	}

	repoDir := setupPackagerTest(t)

	writeFile(t, filepath.Join(repoDir, "target.txt"), "real contents")
	require.NoError(t, os.Symlink(filepath.Join(repoDir, "target.txt"), filepath.Join(repoDir, "link.txt")))
	// A dangling symlink must be skipped rather than fail the scan.
	require.NoError(t, os.Symlink(filepath.Join(repoDir, "gone.txt"), filepath.Join(repoDir, "dangling.txt")))
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "Initial commit")

	_, err := packageDirectory(false)
	require.NoError(t, err)

	contents := extractTarballFiles(t, filepath.Join(tarballDir, tarballName))
	assert.Equal(t, "real contents", contents["link.txt"], "symlink should be stored as its target's contents")
	assert.NotContains(t, contents, "dangling.txt")
}

// TestPackageDirectory_ProducesValidBzip2 checks the whole-archive round trip,
// including that entries carry no local account information.
func TestPackageDirectory_ProducesValidBzip2(t *testing.T) {
	repoDir := setupPackagerTest(t)

	// Large enough to span more than one bzip2 block.
	big := make([]byte, 2<<20)
	for i := range big {
		big[i] = byte(i % 251)
	}
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "big.bin"), big, 0644))
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "Initial commit")

	size, err := packageDirectory(false)
	require.NoError(t, err)
	assert.Positive(t, size)

	f, err := os.Open(filepath.Join(tarballDir, tarballName))
	require.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()

	tr := tar.NewReader(bzip2.NewReader(f))
	var sawBig bool
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		assert.Zero(t, hdr.Uid, "uid should not be recorded for %s", hdr.Name)
		assert.Zero(t, hdr.Gid, "gid should not be recorded for %s", hdr.Name)
		assert.Empty(t, hdr.Uname, "username should not leak into %s", hdr.Name)

		if hdr.Name == "big.bin" {
			sawBig = true
			got, err := io.ReadAll(tr)
			require.NoError(t, err)
			assert.Equal(t, big, got)
		}
	}
	assert.True(t, sawBig, "big.bin missing from tarball")
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

// extractTarballFiles returns the archive's regular files keyed by tar name,
// with their contents.
func extractTarballFiles(t *testing.T, tarballPath string) map[string]string {
	t.Helper()

	f, err := os.Open(tarballPath)
	if err != nil {
		t.Fatalf("Failed to open tarball: %v", err)
	}
	defer func() {
		_ = f.Close()
	}()

	tr := tar.NewReader(bzip2.NewReader(f))
	files := make(map[string]string)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read tar: %v", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("Failed to read %s from tar: %v", header.Name, err)
		}
		files[header.Name] = string(content)
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
