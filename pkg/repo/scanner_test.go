// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"archive/tar"
	"compress/bzip2"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kusaridev/kusari-cli/api"
	"github.com/kusaridev/kusari-cli/pkg/login"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan_ArchiveFormat(t *testing.T) {
	for _, full := range []bool{true, false} {
		// Create a temporary test directory with git repo
		testDir := t.TempDir()

		// Initialize git repo
		require.NoError(t, os.Chdir(testDir))
		require.NoError(t, runCommand("git", "init"))
		require.NoError(t, runCommand("git", "config", "user.email", "test@example.com"))
		require.NoError(t, runCommand("git", "config", "user.name", "Test User"))

		// Create initial commit
		testFile := filepath.Join(testDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))
		require.NoError(t, runCommand("git", "add", "."))
		require.NoError(t, runCommand("git", "commit", "-m", "initial commit"))

		// Make uncommitted change
		require.NoError(t, os.WriteFile(testFile, []byte("uncommitted change"), 0644))

		uploadCalled := false

		const preservedArchive = "preserved-archive.tar.bz2"

		mock := &scanMock{
			fileUploader: func(presignedURL, filePath string) error {
				uploadCalled = true

				// Copy the file to preserve it for inspection
				preservedPath := filepath.Join(testDir, preservedArchive)
				data, err := os.ReadFile(filePath)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(preservedPath, data, 0644))

				return nil
			},
			presignedURLGetter: func(apiEndpoint string, jwtToken string, filePath, workspace string, full bool, size int64) (string, error) {
				return "https://example.com/workspace/test-workspace-id/user/human/test-user-id/diff/blob/123", nil
			},
			defaultWorkspaceGetter: func(platformUrl string, jwtToken string) ([]login.Workspace, map[string][]string, error) {
				return []login.Workspace{
						{
							ID:          "1f961986-c9f3-4760-9d55-1298136cbe2a",
							Description: "Test Workspace",
						},
					}, map[string][]string{
						"1f961986-c9f3-4760-9d55-1298136cbe2a": {"test-tenant"},
					}, nil
			},
			token: "token",
		}

		// Run the scan with dependencies injection
		err := scan(testDir, "HEAD", "https://platform.example.com", "https://console.example.com", false, false, full, "markdown", "", mock)
		require.NoError(t, err)

		// Verify upload was called
		assert.True(t, uploadCalled, "Upload should have been called")

		// Verify the archive format
		preservedPath := filepath.Join(testDir, preservedArchive)
		verifyArchiveFormat(t, preservedPath, filepath.Base(testDir), full)
	}
}

func verifyArchiveFormat(t *testing.T, archivePath, testDir string, full bool) {
	// Open the bz2 compressed file
	file, err := os.Open(archivePath)
	require.NoError(t, err)
	defer file.Close() //nolint:errcheck

	bzReader := bzip2.NewReader(file)
	tarReader := tar.NewReader(bzReader)

	foundMeta := false
	foundPatch := false
	foundTestFile := false
	var metaContent api.BundleMeta

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		// Check for expected files
		switch {
		case strings.HasSuffix(header.Name, "kusari-inspector.json"):
			foundMeta = true
			data, err := io.ReadAll(tarReader)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &metaContent))

			// Verify metadata content
			mainOrMaster := false
			if metaContent.CurrentBranch == "main" || metaContent.CurrentBranch == "master" {
				mainOrMaster = true
			}
			assert.True(t, mainOrMaster)
			assert.Equal(t, testDir, metaContent.DirName)
			assert.Equal(t, "HEAD", metaContent.DiffCmd)
			assert.Equal(t, "kusari-inspector.patch", filepath.Base(metaContent.PatchName))
			assert.True(t, metaContent.GitDirty)
			if full {
				assert.Equal(t, "full", metaContent.ScanType)
			} else {
				assert.Equal(t, "diff", metaContent.ScanType)
			}

		case strings.TrimPrefix(header.Name, "./") == "kusari-inspector.patch":
			foundPatch = true
			data, err := io.ReadAll(tarReader)
			require.NoError(t, err)
			assert.Contains(t, string(data), "test content")
			assert.Contains(t, string(data), "uncommitted change")

		case strings.HasSuffix(header.Name, "test.txt"):
			foundTestFile = true
		}
	}

	// Verify all expected files were found
	assert.True(t, foundMeta, "kusari-inspector.json should be in archive")
	if !full {
		assert.True(t, foundPatch, "kusari-inspector.patch should be in archive")
	}
	assert.True(t, foundTestFile, "test.txt should be in archive")
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func TestDetectMonoRepo(t *testing.T) {
	tests := []struct {
		name             string
		setupFunc        func(dir string) error
		expectMonoRepo   bool
		expectIndicators []string
	}{
		{
			name: "not a monorepo - single project",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644)
			},
			expectMonoRepo: false,
		},
		{
			name: "monorepo - lerna.json present",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "lerna.json"), []byte(`{}`), 0644)
			},
			expectMonoRepo:   true,
			expectIndicators: []string{"monorepo config: lerna.json"},
		},
		{
			name: "monorepo - nx.json present",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "nx.json"), []byte(`{}`), 0644)
			},
			expectMonoRepo:   true,
			expectIndicators: []string{"monorepo config: nx.json"},
		},
		{
			name: "monorepo - pnpm-workspace.yaml present",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte(`packages:\n  - 'packages/*'`), 0644)
			},
			expectMonoRepo:   true,
			expectIndicators: []string{"monorepo config: pnpm-workspace.yaml"},
		},
		{
			name: "monorepo - package.json with workspaces",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"workspaces": ["packages/*"]}`), 0644)
			},
			expectMonoRepo:   true,
			expectIndicators: []string{"package.json with workspaces"},
		},
		{
			name: "monorepo - Cargo.toml with workspace",
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(`[workspace]\nmembers = ["crate1", "crate2"]`), 0644)
			},
			expectMonoRepo:   true,
			expectIndicators: []string{"Cargo.toml with [workspace]"},
		},
		{
			name: "monorepo - multiple go.mod files",
			setupFunc: func(dir string) error {
				// Root go.mod
				if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module example.com/root`), 0644); err != nil {
					return err
				}
				// Create subdirectory with another go.mod
				subdir := filepath.Join(dir, "service1")
				if err := os.Mkdir(subdir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(subdir, "go.mod"), []byte(`module example.com/service1`), 0644); err != nil {
					return err
				}
				// Create another subdirectory with go.mod
				subdir2 := filepath.Join(dir, "service2")
				if err := os.Mkdir(subdir2, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(subdir2, "go.mod"), []byte(`module example.com/service2`), 0644)
			},
			expectMonoRepo:   true,
			expectIndicators: []string{"multiple go.mod files in subdirectories"},
		},
		{
			name: "monorepo - multiple package.json files",
			setupFunc: func(dir string) error {
				// Root package.json
				if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "root"}`), 0644); err != nil {
					return err
				}
				// Create packages directory with multiple package.json
				packagesDir := filepath.Join(dir, "packages")
				if err := os.Mkdir(packagesDir, 0755); err != nil {
					return err
				}
				pkg1 := filepath.Join(packagesDir, "pkg1")
				if err := os.Mkdir(pkg1, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(pkg1, "package.json"), []byte(`{"name": "pkg1"}`), 0644); err != nil {
					return err
				}
				pkg2 := filepath.Join(packagesDir, "pkg2")
				if err := os.Mkdir(pkg2, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(pkg2, "package.json"), []byte(`{"name": "pkg2"}`), 0644)
			},
			expectMonoRepo:   true,
			expectIndicators: []string{"multiple package.json files in subdirectories"},
		},
		{
			name: "not monorepo - single package.json with node_modules",
			setupFunc: func(dir string) error {
				// Root package.json
				if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "root"}`), 0644); err != nil {
					return err
				}
				// node_modules should be ignored
				nodeModules := filepath.Join(dir, "node_modules", "some-package")
				if err := os.MkdirAll(nodeModules, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(nodeModules, "package.json"), []byte(`{"name": "dep"}`), 0644)
			},
			expectMonoRepo: false,
		},
		{
			name: "monorepo - multiple pom.xml files",
			setupFunc: func(dir string) error {
				// Root pom.xml
				if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(`<project></project>`), 0644); err != nil {
					return err
				}
				// Create subdirectories with pom.xml
				module1 := filepath.Join(dir, "module1")
				if err := os.Mkdir(module1, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(module1, "pom.xml"), []byte(`<project></project>`), 0644); err != nil {
					return err
				}
				module2 := filepath.Join(dir, "module2")
				if err := os.Mkdir(module2, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(module2, "pom.xml"), []byte(`<project></project>`), 0644)
			},
			expectMonoRepo:   true,
			expectIndicators: []string{"multiple pom.xml files in subdirectories"},
		},
		{
			name: "monorepo - polyglot (go + nodejs)",
			setupFunc: func(dir string) error {
				// Backend service with Go
				backend := filepath.Join(dir, "backend")
				if err := os.Mkdir(backend, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(backend, "go.mod"), []byte(`module example.com/backend`), 0644); err != nil {
					return err
				}
				// Frontend service with Node.js
				frontend := filepath.Join(dir, "frontend")
				if err := os.Mkdir(frontend, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(frontend, "package.json"), []byte(`{"name": "frontend"}`), 0644)
			},
			expectMonoRepo:   true,
			expectIndicators: []string{"multiple project types detected"},
		},
		{
			name: "not monorepo - single project with docs",
			setupFunc: func(dir string) error {
				// Main Go project in cmd/
				cmd := filepath.Join(dir, "cmd")
				if err := os.Mkdir(cmd, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(cmd, "go.mod"), []byte(`module example.com/cmd`), 0644); err != nil {
					return err
				}
				// Documentation with Jekyll/Gemfile
				docs := filepath.Join(dir, "docs")
				if err := os.Mkdir(docs, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(docs, "Gemfile"), []byte(`gem "github-pages"`), 0644)
			},
			expectMonoRepo: false,
		},
		{
			name: "not monorepo - single project with examples",
			setupFunc: func(dir string) error {
				// Root package.json
				if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "root"}`), 0644); err != nil {
					return err
				}
				// Examples directory should be ignored
				examples := filepath.Join(dir, "examples", "demo")
				if err := os.MkdirAll(examples, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(examples, "package.json"), []byte(`{"name": "demo"}`), 0644)
			},
			expectMonoRepo: false,
		},
		{
			name: "not monorepo - single project with integration tests",
			setupFunc: func(dir string) error {
				// Root go.mod
				if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module example.com/root`), 0644); err != nil {
					return err
				}
				// Integration test directory should be ignored (pattern match)
				integrationTest := filepath.Join(dir, "pkg", "db", "integrationtest", "tool")
				if err := os.MkdirAll(integrationTest, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(integrationTest, "package.json"), []byte(`{"name": "test-tool"}`), 0644)
			},
			expectMonoRepo: false,
		},
		{
			name: "not monorepo - Go project with generated TypeScript client",
			setupFunc: func(dir string) error {
				// Root go.mod
				if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module example.com/root`), 0644); err != nil {
					return err
				}
				// Generated API client should be ignored
				apiGen := filepath.Join(dir, "api", "npm")
				if err := os.MkdirAll(apiGen, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(apiGen, "package.json"), []byte(`{"name": "@example/api-client"}`), 0644)
			},
			expectMonoRepo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			testDir := t.TempDir()

			// Setup the test directory
			if tt.setupFunc != nil {
				err := tt.setupFunc(testDir)
				require.NoError(t, err)
			}

			// Run detection
			isMonoRepo, indicators, err := detectMonoRepo(testDir)
			require.NoError(t, err)

			// Verify results
			assert.Equal(t, tt.expectMonoRepo, isMonoRepo, "monorepo detection mismatch")

			if tt.expectMonoRepo {
				assert.NotEmpty(t, indicators, "expected indicators to be present")
				for _, expectedIndicator := range tt.expectIndicators {
					found := false
					for _, indicator := range indicators {
						// Use Contains for partial matching since some indicators include details
						if strings.Contains(indicator, expectedIndicator) {
							found = true
							break
						}
					}
					assert.True(t, found, "expected indicator %q not found in %v", expectedIndicator, indicators)
				}
			}
		})
	}
}

func TestMonoRepoCheck_OnlyForRiskCheck(t *testing.T) {
	// Create a temporary test directory that looks like a monorepo
	testDir := t.TempDir()

	// Initialize git repo
	require.NoError(t, os.Chdir(testDir))
	require.NoError(t, runCommand("git", "init"))
	require.NoError(t, runCommand("git", "config", "user.email", "test@example.com"))
	require.NoError(t, runCommand("git", "config", "user.name", "Test User"))

	// Create a monorepo structure with lerna.json
	require.NoError(t, os.WriteFile(filepath.Join(testDir, "lerna.json"), []byte(`{}`), 0644))

	// Create initial commit
	testFile := filepath.Join(testDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))
	require.NoError(t, runCommand("git", "add", "."))
	require.NoError(t, runCommand("git", "commit", "-m", "initial commit"))

	// Make uncommitted change
	require.NoError(t, os.WriteFile(testFile, []byte("uncommitted change"), 0644))

	mock := &scanMock{
		fileUploader: func(presignedURL, filePath string) error {
			return nil
		},
		presignedURLGetter: func(apiEndpoint string, jwtToken string, filePath, workspace string, full bool, size int64) (string, error) {
			return "https://example.com/workspace/test-workspace-id/user/human/test-user-id/diff/blob/123", nil
		},
		defaultWorkspaceGetter: func(platformUrl string, jwtToken string) ([]login.Workspace, map[string][]string, error) {
			return []login.Workspace{
					{
						ID:          "1f961986-c9f3-4760-9d55-1298136cbe2a",
						Description: "Test Workspace",
					},
				}, map[string][]string{
					"1f961986-c9f3-4760-9d55-1298136cbe2a": {"test-tenant"},
				}, nil
		},
		token: "token",
	}

	t.Run("diff scan should succeed on monorepo", func(t *testing.T) {
		// Diff scan (full=false) should succeed even with monorepo
		err := scan(testDir, "HEAD", "https://platform.example.com", "https://console.example.com", false, false, false, "markdown", "", mock)
		assert.NoError(t, err, "diff scan should succeed on monorepo")
	})

	t.Run("risk check should fail on monorepo", func(t *testing.T) {
		// Risk check (full=true) should detect monorepo and exit
		// Since the code calls os.Exit(1), we can't directly test this in a unit test
		// Instead, we verify that detectMonoRepo correctly identifies it
		isMonoRepo, indicators, err := detectMonoRepo(testDir)
		require.NoError(t, err)
		assert.True(t, isMonoRepo, "should detect monorepo")
		assert.Contains(t, indicators, "monorepo config: lerna.json", "should detect lerna.json")
	})
}
