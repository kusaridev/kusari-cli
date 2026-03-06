// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcp

// ScanType represents the type of security scan.
type ScanType string

const (
	// ScanTypeDiff scans only uncommitted changes.
	ScanTypeDiff ScanType = "diff"
	// ScanTypeFull performs a comprehensive repository scan.
	ScanTypeFull ScanType = "full"
)

// ScanRequest represents a queued scan request.
type ScanRequest struct {
	// ID is the unique request identifier.
	ID string
	// Type is the scan type (diff or full).
	Type ScanType
	// RepoPath is the repository path to scan.
	RepoPath string
	// BaseRef is the base git reference for diff scans.
	BaseRef string
	// OutputFormat is the output format (markdown or sarif).
	OutputFormat string
	// ResultChan is the channel for returning results.
	ResultChan chan ScanResult
}

// ScanResult represents the result of a scan operation.
type ScanResult struct {
	// Success indicates whether the scan completed successfully.
	Success bool
	// ConsoleURL is the URL to view results in Kusari console.
	ConsoleURL string
	// Results contains the formatted scan results.
	Results string
	// Error contains the error message if scan failed.
	Error string
	// QueuePosition is the position in queue (0 if processing).
	QueuePosition int
}
