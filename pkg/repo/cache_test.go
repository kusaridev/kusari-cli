// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"github.com/kusaridev/kusari-cli/v2/api"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useHomeDir points os.UserHomeDir at dir for the duration of the test.
//
// Setting only HOME is not enough: os.UserHomeDir reads USERPROFILE on Windows,
// so a HOME-only override leaves these tests reading and writing the real
// ~/.kusari of whoever runs them -- which for the ClearCache tests means
// deleting a developer's actual scan cache.
func useHomeDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestLoadCache_EmptyFile(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	useHomeDir(t, tmpDir)

	cache, err := loadCache()
	require.NoError(t, err)
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.Entries)
	assert.Empty(t, cache.Entries)
}

func TestSaveAndLoadCache(t *testing.T) {
	tmpDir := t.TempDir()
	useHomeDir(t, tmpDir)

	// Create cache directory
	kusariDir := filepath.Join(tmpDir, ".kusari")
	require.NoError(t, os.MkdirAll(kusariDir, 0700))

	// Create a cache with an entry
	cache := &ScanCache{
		Entries: map[string]ScanCacheEntry{
			"/test/repo": {
				DiffHash:   "abc123",
				BaseRef:    "HEAD",
				Results:    "test results",
				ConsoleURL: "https://console.kusari.dev/test",
				Timestamp:  time.Now(),
			},
		},
	}

	// Save cache
	err := saveCache(cache)
	require.NoError(t, err)

	// Load cache
	loadedCache, err := loadCache()
	require.NoError(t, err)

	assert.Equal(t, 1, len(loadedCache.Entries))
	entry := loadedCache.Entries["/test/repo"]
	assert.Equal(t, "abc123", entry.DiffHash)
	assert.Equal(t, "HEAD", entry.BaseRef)
	assert.Equal(t, "test results", entry.Results)
	assert.Equal(t, "https://console.kusari.dev/test", entry.ConsoleURL)
}

func TestCleanupOldEntries(t *testing.T) {
	cache := &ScanCache{
		Entries: map[string]ScanCacheEntry{
			"/fresh/repo": {
				DiffHash:  "fresh",
				Timestamp: time.Now(),
			},
			"/old/repo": {
				DiffHash:  "old",
				Timestamp: time.Now().Add(-48 * time.Hour), // 2 days old
			},
		},
	}

	cleanupOldEntries(cache)

	assert.Equal(t, 1, len(cache.Entries))
	_, exists := cache.Entries["/fresh/repo"]
	assert.True(t, exists)
	_, exists = cache.Entries["/old/repo"]
	assert.False(t, exists)
}

func TestClearCache(t *testing.T) {
	tmpDir := t.TempDir()
	useHomeDir(t, tmpDir)

	// Create cache directory and file
	kusariDir := filepath.Join(tmpDir, ".kusari")
	require.NoError(t, os.MkdirAll(kusariDir, 0700))
	cachePath := filepath.Join(kusariDir, cacheFileName)
	require.NoError(t, os.WriteFile(cachePath, []byte("{}"), 0600))

	// Clear cache
	err := ClearCache()
	require.NoError(t, err)

	// Verify file is gone
	_, err = os.Stat(cachePath)
	assert.True(t, os.IsNotExist(err))
}

func TestClearCache_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	useHomeDir(t, tmpDir)

	// Clear cache when no file exists should not error
	err := ClearCache()
	require.NoError(t, err)
}

func TestLoadCache_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	useHomeDir(t, tmpDir)

	// Create cache directory with corrupted file
	kusariDir := filepath.Join(tmpDir, ".kusari")
	require.NoError(t, os.MkdirAll(kusariDir, 0700))
	cachePath := filepath.Join(kusariDir, cacheFileName)
	require.NoError(t, os.WriteFile(cachePath, []byte("not valid json"), 0600))

	// Should return empty cache, not error
	cache, err := loadCache()
	require.NoError(t, err)
	assert.NotNil(t, cache)
	assert.Empty(t, cache.Entries)
}

func TestScanCacheEntry_Fields(t *testing.T) {
	now := time.Now()
	entry := ScanCacheEntry{
		DiffHash:   "hash123",
		BaseRef:    "main",
		Results:    "test results",
		ConsoleURL: "https://example.com",
		Timestamp:  now,
	}

	assert.Equal(t, "hash123", entry.DiffHash)
	assert.Equal(t, "main", entry.BaseRef)
	assert.Equal(t, "test results", entry.Results)
	assert.Equal(t, "https://example.com", entry.ConsoleURL)
	assert.Equal(t, now, entry.Timestamp)
}

func TestCacheResult_Fields(t *testing.T) {
	result := CacheResult{
		Hit:        true,
		Results:    "cached results",
		ConsoleURL: "https://console.kusari.dev/cached",
	}

	assert.True(t, result.Hit)
	assert.Equal(t, "cached results", result.Results)
	assert.Equal(t, "https://console.kusari.dev/cached", result.ConsoleURL)
}

func TestCheckCache_NoEntry(t *testing.T) {
	tmpDir := t.TempDir()
	useHomeDir(t, tmpDir)

	result, err := CheckCache("/nonexistent/repo", "HEAD", "markdown", false)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Hit)
}

// cacheRepo builds a repo with an uncommitted change and points the cache at a
// sandboxed HOME, so these tests cannot touch the developer's real cache.
func cacheRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "T"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("base\n"), 0644))
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("changed\n"), 0644))
	return dir
}

// A SARIF document is not a valid answer to a request for markdown. The rest of
// the cache key is identical for the same diff, so without the format the second
// caller silently receives the first caller's format.
func TestCache_OutputFormatIsPartOfTheKey(t *testing.T) {
	dir := cacheRepo(t)

	require.NoError(t, SaveToCache(dir, "HEAD", "sarif", `{"runs":[]}`, "https://console/x", nil, false))

	hit, err := CheckCache(dir, "HEAD", "sarif", false)
	require.NoError(t, err)
	require.True(t, hit.Hit, "the same format should hit")
	assert.Equal(t, `{"runs":[]}`, hit.Results)

	miss, err := CheckCache(dir, "HEAD", "markdown", false)
	require.NoError(t, err)
	assert.False(t, miss.Hit, "a different output format must not be served from cache")
}

// Entries written before output_format existed carry an empty value. Guessing
// which format they hold would serve the wrong one, so they must miss.
func TestCache_LegacyEntryWithoutFormatMisses(t *testing.T) {
	dir := cacheRepo(t)

	require.NoError(t, SaveToCache(dir, "HEAD", "", "legacy content", "https://console/x", nil, false))

	result, err := CheckCache(dir, "HEAD", "markdown", false)
	require.NoError(t, err)
	assert.False(t, result.Hit, "an entry with no recorded format must not be reused")
}

// A gated scan must reach the same decision from cache that it reached live.
// Serving the report and returning success because the work was skipped would
// silently disable the gate on exactly the case it exists for: a re-run of an
// unchanged diff.
func TestCache_GatedScanServedFromCacheStillBlocks(t *testing.T) {
	dir := cacheRepo(t)

	blocking := &CachedVerdict{
		ShouldProceed:         false,
		CodeMitigations:       2,
		DependencyMitigations: 1,
		Justification:         "hardcoded credential",
	}
	require.NoError(t, SaveToCache(dir, "HEAD", "sarif", `{"runs":[]}`, "https://console/x", blocking, false))

	hit, err := CheckCache(dir, "HEAD", "sarif", false)
	require.NoError(t, err)
	require.True(t, hit.Hit)
	require.NotNil(t, hit.Verdict, "the verdict must survive the round trip through disk")

	// Gated: the cached verdict must produce the same error a live scan would.
	gErr := hit.Verdict.findingsError(true, hit.ConsoleURL)
	require.Error(t, gErr)
	var fe *FindingsError
	require.ErrorAs(t, gErr, &fe)
	assert.Equal(t, 2, fe.CodeMitigations)
	assert.Equal(t, 1, fe.DependencyMitigations)
	assert.Equal(t, "hardcoded credential", fe.Reason)

	// Ungated: the same cached entry must not fail anything.
	assert.NoError(t, hit.Verdict.findingsError(false, hit.ConsoleURL))
}

func TestCache_GatedScanServedFromCacheAllowsOnProceed(t *testing.T) {
	dir := cacheRepo(t)

	proceed := &CachedVerdict{ShouldProceed: true, CodeMitigations: 3}
	require.NoError(t, SaveToCache(dir, "HEAD", "markdown", "report", "https://console/x", proceed, false))

	hit, err := CheckCache(dir, "HEAD", "markdown", false)
	require.NoError(t, err)
	require.True(t, hit.Hit)

	// should_proceed is the verdict: findings attached to it are advisory and
	// must not fail a gated scan.
	assert.NoError(t, hit.Verdict.findingsError(true, hit.ConsoleURL))
}

// A nil verdict must behave exactly as the live path does with a nil analysis,
// so a cache hit and a fresh scan never disagree.
func TestCache_NilVerdictMatchesLiveBehavior(t *testing.T) {
	var cached *CachedVerdict
	assert.NoError(t, cached.findingsError(true, "https://console/x"))
	assert.NoError(t, findingsResult(true, nil, "https://console/x"),
		"the live path returns no error for a nil analysis; the cache must agree")
}

// verdictFrom must capture what the gate needs from a live analysis.
func TestVerdictFrom(t *testing.T) {
	assert.Nil(t, verdictFrom(nil))

	v := verdictFrom(&api.SecurityAnalysis{
		ShouldProceed: false,
		Justification: "why",
		RequiredCodeMitigations: []api.CodeMitigationItem{
			{Path: "a.js", LineNumber: 1}, {Path: "b.js", LineNumber: 2},
		},
		RequiredDependencyMitigations: []api.DependencyMitigationItem{{Content: "bump lodash"}},
	})
	require.NotNil(t, v)
	assert.False(t, v.ShouldProceed)
	assert.Equal(t, 2, v.CodeMitigations)
	assert.Equal(t, 1, v.DependencyMitigations)
	assert.Equal(t, "why", v.Justification)
}

// The MCP server scans with sarif and the CLI often with markdown, against the
// same repo and diff. Each must get its own format back.
func TestCache_McpAndCliFormatsDoNotCollide(t *testing.T) {
	dir := cacheRepo(t)

	// MCP-shaped scan: sarif, full output, waits for results.
	require.NoError(t, SaveToCache(dir, "HEAD", "sarif", `{"runs":[{"results":[]}]}`, "https://console/x",
		&CachedVerdict{ShouldProceed: true}, false))

	mcp, err := CheckCache(dir, "HEAD", "sarif", false)
	require.NoError(t, err)
	require.True(t, mcp.Hit, "the MCP path should be served from cache")
	assert.Contains(t, mcp.Results, `"runs"`)

	cli, err := CheckCache(dir, "HEAD", "markdown", false)
	require.NoError(t, err)
	assert.False(t, cli.Hit, "a markdown request must not be served the cached SARIF")
}
