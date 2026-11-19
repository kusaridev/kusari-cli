// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package ai

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanLocalChanges_ValidatesRepoPath(t *testing.T) {
	cfg := NewConfig()
	server, err := NewServer(cfg)
	require.NoError(t, err)

	// Test with non-existent path
	args := ScanLocalChangesArgs{
		RepoPath:     "/nonexistent/path",
		BaseRef:      "HEAD",
		OutputFormat: "sarif",
	}

	result, err := server.executeScanLocalChanges(context.Background(), args)

	// Should return error for non-existent path
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestScanLocalChanges_ValidatesGitRepo(t *testing.T) {
	// Create a temp directory that is NOT a git repo
	tmpDir := t.TempDir()

	cfg := NewConfig()
	server, err := NewServer(cfg)
	require.NoError(t, err)

	args := ScanLocalChangesArgs{
		RepoPath:     tmpDir,
		BaseRef:      "HEAD",
		OutputFormat: "sarif",
	}

	result, err := server.executeScanLocalChanges(context.Background(), args)

	// Should return error for non-git directory
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ".git")
	assert.Nil(t, result)
}

func TestScanLocalChanges_DefaultsToCurrentDir(t *testing.T) {
	cfg := NewConfig()
	server, err := NewServer(cfg)
	require.NoError(t, err)

	// Empty repo_path should default to current directory
	args := ScanLocalChangesArgs{
		RepoPath:     "",
		BaseRef:      "HEAD",
		OutputFormat: "sarif",
	}

	// Get current dir for comparison
	cwd, err := os.Getwd()
	require.NoError(t, err)

	normalizedPath := server.normalizeRepoPath(args.RepoPath)
	assert.Equal(t, cwd, normalizedPath)
}

func TestScanLocalChanges_DefaultsBaseRef(t *testing.T) {
	cfg := NewConfig()
	server, err := NewServer(cfg)
	require.NoError(t, err)

	args := ScanLocalChangesArgs{
		RepoPath:     "",
		BaseRef:      "",
		OutputFormat: "sarif",
	}

	normalizedRef := server.normalizeBaseRef(args.BaseRef)
	assert.Equal(t, "HEAD", normalizedRef)
}

func TestScanLocalChanges_DefaultsOutputFormat(t *testing.T) {
	cfg := NewConfig()
	server, err := NewServer(cfg)
	require.NoError(t, err)

	args := ScanLocalChangesArgs{
		RepoPath:     "",
		BaseRef:      "HEAD",
		OutputFormat: "",
	}

	normalizedFormat := server.normalizeOutputFormat(args.OutputFormat)
	assert.Equal(t, "sarif", normalizedFormat)
}

func TestScanLocalChanges_ValidatesOutputFormat(t *testing.T) {
	cfg := NewConfig()
	server, err := NewServer(cfg)
	require.NoError(t, err)

	tests := []struct {
		format   string
		expected string
		valid    bool
	}{
		{"sarif", "sarif", true},
		{"markdown", "markdown", true},
		{"SARIF", "sarif", true},
		{"Markdown", "markdown", true},
		{"invalid", "sarif", false}, // defaults to sarif for invalid
		{"", "sarif", true},         // empty defaults to sarif
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := server.normalizeOutputFormat(tt.format)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateGitRepo(t *testing.T) {
	// Test non-git directory
	tmpDir := t.TempDir()
	err := validateGitRepo(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ".git")

	// Test git directory (create .git folder)
	gitDir := filepath.Join(tmpDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	require.NoError(t, err)

	err = validateGitRepo(tmpDir)
	assert.NoError(t, err)
}

func TestValidateDirectory(t *testing.T) {
	// Test non-existent path
	err := validateDirectory("/nonexistent/path/to/nowhere")
	assert.Error(t, err)

	// Test existing directory
	tmpDir := t.TempDir()
	err = validateDirectory(tmpDir)
	assert.NoError(t, err)

	// Test file (not directory)
	tmpFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(tmpFile, []byte("test"), 0644)
	require.NoError(t, err)

	err = validateDirectory(tmpFile)
	assert.Error(t, err)
}
