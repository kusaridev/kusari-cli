// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	cacheFileName = "scan-cache.json"
	// CacheMaxAge is the maximum age of a cache entry before it's considered stale
	CacheMaxAge = 24 * time.Hour
)

// ScanCacheEntry represents a cached scan result for a repository.
type ScanCacheEntry struct {
	DiffHash   string    `json:"diff_hash"`
	BaseRef    string    `json:"base_ref"`
	Results    string    `json:"results"`     // The scan output (SARIF or markdown)
	ConsoleURL string    `json:"console_url"` // Link to console results
	Timestamp  time.Time `json:"timestamp"`
}

// ScanCache manages cached scan results.
type ScanCache struct {
	Entries map[string]ScanCacheEntry `json:"entries"` // keyed by repo path
}

// CacheResult represents the result of a cache check.
type CacheResult struct {
	Hit        bool
	Results    string
	ConsoleURL string
}

// getCachePath returns the path to the cache file.
func getCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".kusari", cacheFileName), nil
}

// loadCache loads the scan cache from disk.
func loadCache() (*ScanCache, error) {
	cachePath, err := getCachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ScanCache{Entries: make(map[string]ScanCacheEntry)}, nil
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache ScanCache
	if err := json.Unmarshal(data, &cache); err != nil {
		// If cache is corrupted, start fresh
		return &ScanCache{Entries: make(map[string]ScanCacheEntry)}, nil
	}

	if cache.Entries == nil {
		cache.Entries = make(map[string]ScanCacheEntry)
	}

	return &cache, nil
}

// saveCache saves the scan cache to disk.
func saveCache(cache *ScanCache) error {
	cachePath, err := getCachePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// computeDiffHash computes a SHA256 hash of the git diff output and untracked files.
func computeDiffHash(repoPath, baseRef string) (string, error) {
	// Get diff of tracked files
	diffCmd := exec.Command("git", "-C", repoPath, "diff", "--binary", baseRef)
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run git diff: %w", err)
	}

	// Get list of untracked files (excluding ignored files)
	untrackedCmd := exec.Command("git", "-C", repoPath, "ls-files", "--others", "--exclude-standard")
	untrackedOutput, err := untrackedCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list untracked files: %w", err)
	}

	// If there's no diff and no untracked files, return empty hash
	if len(diffOutput) == 0 && len(untrackedOutput) == 0 {
		return "", nil
	}

	// Create a hash that combines diff output and untracked file contents
	hasher := sha256.New()

	// Add diff output to hash
	hasher.Write(diffOutput)

	// Add untracked files and their contents to hash
	if len(untrackedOutput) > 0 {
		// Add a separator to distinguish diff from untracked content
		hasher.Write([]byte("\x00UNTRACKED\x00"))

		untrackedFiles := strings.Split(strings.TrimSpace(string(untrackedOutput)), "\n")
		for _, file := range untrackedFiles {
			if file == "" {
				continue
			}
			// Add filename to hash
			hasher.Write([]byte(file))
			hasher.Write([]byte("\x00"))

			// Add file content to hash
			filePath := filepath.Join(repoPath, file)
			content, err := os.ReadFile(filePath)
			if err != nil {
				// If we can't read the file, just hash the error message
				// This still ensures changes are detected
				hasher.Write([]byte(err.Error()))
			} else {
				hasher.Write(content)
			}
			hasher.Write([]byte("\x00"))
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CheckCache checks if there's a valid cached result for the given repo and base ref.
// Returns CacheResult with Hit=true if cache is valid, or Hit=false if scan needed.
// Returns error only for the special case of no changes to scan.
func CheckCache(repoPath, baseRef string, verbose bool) (*CacheResult, error) {
	// Normalize repo path to absolute
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		absPath = repoPath
	}

	cache, err := loadCache()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Cache load failed: %v\n", err)
		}
		return &CacheResult{Hit: false}, nil
	}

	entry, exists := cache.Entries[absPath]
	if !exists {
		if verbose {
			fmt.Fprintf(os.Stderr, "No cache entry for %s\n", absPath)
		}
		return &CacheResult{Hit: false}, nil
	}

	// Check if base ref matches
	if entry.BaseRef != baseRef {
		if verbose {
			fmt.Fprintf(os.Stderr, "Cache base ref mismatch: %s vs %s\n", entry.BaseRef, baseRef)
		}
		return &CacheResult{Hit: false}, nil
	}

	// Check if cache is too old
	if time.Since(entry.Timestamp) > CacheMaxAge {
		if verbose {
			fmt.Fprintf(os.Stderr, "Cache entry expired (age: %v)\n", time.Since(entry.Timestamp))
		}
		return &CacheResult{Hit: false}, nil
	}

	// Compute current diff hash
	currentHash, err := computeDiffHash(absPath, baseRef)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Failed to compute diff hash: %v\n", err)
		}
		return &CacheResult{Hit: false}, nil
	}

	// Empty diff means no changes - nothing to scan
	if currentHash == "" {
		return nil, fmt.Errorf("no changes to scan (git diff is empty)")
	}

	// Compare hashes
	if entry.DiffHash == currentHash {
		if verbose {
			fmt.Fprintf(os.Stderr, "Cache hit! Diff hash matches: %s\n", currentHash[:16])
		}
		return &CacheResult{
			Hit:        true,
			Results:    entry.Results,
			ConsoleURL: entry.ConsoleURL,
		}, nil
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Cache miss: diff hash changed\n")
	}
	return &CacheResult{Hit: false}, nil
}

// SaveToCache stores a scan result in the cache.
func SaveToCache(repoPath, baseRef, results, consoleURL string, verbose bool) error {
	// Normalize repo path to absolute
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		absPath = repoPath
	}

	cache, err := loadCache()
	if err != nil {
		return err
	}

	// Compute current diff hash
	diffHash, err := computeDiffHash(absPath, baseRef)
	if err != nil {
		return err
	}

	cache.Entries[absPath] = ScanCacheEntry{
		DiffHash:   diffHash,
		BaseRef:    baseRef,
		Results:    results,
		ConsoleURL: consoleURL,
		Timestamp:  time.Now(),
	}

	// Clean up old entries while we're at it
	cleanupOldEntries(cache)

	if verbose {
		fmt.Fprintf(os.Stderr, "Scan result cached for future use\n")
	}

	return saveCache(cache)
}

// cleanupOldEntries removes cache entries older than CacheMaxAge.
func cleanupOldEntries(cache *ScanCache) {
	for path, entry := range cache.Entries {
		if time.Since(entry.Timestamp) > CacheMaxAge {
			delete(cache.Entries, path)
		}
	}
}

// ClearCache removes all cached scan results.
func ClearCache() error {
	cachePath, err := getCachePath()
	if err != nil {
		return err
	}

	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}

	return nil
}
