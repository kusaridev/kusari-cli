// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcp

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

// Phase 7: Console URL extraction tests (T048)

func TestExtractConsoleURL_FromStderr(t *testing.T) {
	tests := []struct {
		name     string
		stderr   string
		expected string
	}{
		{
			name:     "extracts URL from typical stderr output",
			stderr:   "Generating diff...\nPackaging directory...\nOnce completed, you can see results at: https://console.us.kusari.cloud/workspaces/123/analysis/456/result\nUpload successful",
			expected: "https://console.us.kusari.cloud/workspaces/123/analysis/456/result",
		},
		{
			name:     "extracts URL with different domain",
			stderr:   "Once completed, you can see results at: https://console.example.com/workspaces/abc/analysis/def/result",
			expected: "https://console.example.com/workspaces/abc/analysis/def/result",
		},
		{
			name:     "extracts URL from 'view your results here' line",
			stderr:   "You can also view your results here: https://console.us.kusari.cloud/workspaces/w1/analysis/s1/result\n",
			expected: "https://console.us.kusari.cloud/workspaces/w1/analysis/s1/result",
		},
		{
			name:     "returns empty when no URL present",
			stderr:   "Some error output without URL",
			expected: "",
		},
		{
			name:     "handles empty stderr",
			stderr:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractConsoleURL(tt.stderr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatResultWithConsoleURL(t *testing.T) {
	tests := []struct {
		name       string
		results    string
		consoleURL string
		expected   string
	}{
		{
			name:       "adds console URL banner to results",
			results:    "Scan results here",
			consoleURL: "https://console.example.com/results",
			expected:   "View detailed results: https://console.example.com/results\n\n---\n\nScan results here",
		},
		{
			name:       "returns results unchanged when no URL",
			results:    "Scan results here",
			consoleURL: "",
			expected:   "Scan results here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatResultWithConsoleURL(tt.results, tt.consoleURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}
