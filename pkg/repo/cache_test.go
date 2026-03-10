// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCache_EmptyFile(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cache, err := loadCache()
	require.NoError(t, err)
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.Entries)
	assert.Empty(t, cache.Entries)
}

func TestSaveAndLoadCache(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Clear cache when no file exists should not error
	err := ClearCache()
	require.NoError(t, err)
}

func TestLoadCache_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	result, err := CheckCache("/nonexistent/repo", "HEAD", false)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Hit)
}
