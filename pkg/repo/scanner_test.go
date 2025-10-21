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
			presignedURLGetter: func(apiEndpoint string, jwtToken string, filePath, workspace string, full bool) (string, error) {
				return "https://s3.example.com/upload?epoch=1234567890", nil
			},
			defaultWorkspaceGetter: func(apiEndpoint string, jwtToken string) (string, error) {
				return "1f961986-c9f3-4760-9d55-1298136cbe2a", nil
			},
			token: "token",
		}

		// Run the scan with dependencies injection
		err := scan(testDir, "HEAD", "https://platform.example.com", "https://console.example.com", false, false, full, mock)
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
