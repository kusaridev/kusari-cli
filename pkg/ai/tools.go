// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package ai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kusaridev/kusari-cli/pkg/repo"
)

// ScanToolResult contains the result of a scan operation.
type ScanToolResult struct {
	Success    bool
	ConsoleURL string
	Results    string
	Error      string
}

// executeScanLocalChanges performs a local changes scan using internal packages.
func (s *Server) executeScanLocalChanges(ctx context.Context, args ScanLocalChangesArgs) (*ScanToolResult, error) {
	repoPath := s.normalizeRepoPath(args.RepoPath)
	baseRef := s.normalizeBaseRef(args.BaseRef)
	outputFormat := s.normalizeOutputFormat(args.OutputFormat)

	// Validate inputs
	if err := validateDirectory(repoPath); err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}

	if err := validateGitRepo(repoPath); err != nil {
		return nil, err
	}

	// Ensure we have valid authentication before scanning
	if err := s.ensureAuthenticated(ctx); err != nil {
		return nil, fmt.Errorf("authentication required: %w", err)
	}

	if s.config.Verbose {
		fmt.Fprintf(os.Stderr, "[kusari-mcp] Scanning local changes in %s (base: %s, format: %s)\n",
			repoPath, baseRef, outputFormat)
	}

	// Capture stdout/stderr from the scan function
	stdout, stderr, err := captureOutput(func() error {
		return repo.Scan(
			repoPath,
			baseRef,
			s.config.PlatformURL,
			s.config.ConsoleURL,
			s.config.Verbose,
			true, // wait for results
			outputFormat,
			"", // no comment platform for MCP
		)
	})

	// If scan fails due to auth, try to re-authenticate and retry once
	if err != nil && isAuthError(err) {
		fmt.Fprintln(os.Stderr, "[kusari-mcp] Authentication error during scan, attempting re-authentication...")
		if authErr := s.triggerBrowserAuth(ctx); authErr != nil {
			return nil, fmt.Errorf("re-authentication failed: %w", authErr)
		}

		// Retry the scan after re-auth
		stdout, stderr, err = captureOutput(func() error {
			return repo.Scan(
				repoPath,
				baseRef,
				s.config.PlatformURL,
				s.config.ConsoleURL,
				s.config.Verbose,
				true,
				outputFormat,
				"",
			)
		})
	}

	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	// Extract console URL from stderr
	consoleURL := extractConsoleURL(stderr)

	// Format results with console URL banner
	results := stdout
	if results == "" {
		results = "Scan completed successfully."
	}
	results = formatResultWithConsoleURL(results, consoleURL)

	return &ScanToolResult{
		Success:    true,
		ConsoleURL: consoleURL,
		Results:    results,
	}, nil
}

// normalizeRepoPath returns the repo path, defaulting to current directory if empty.
func (s *Server) normalizeRepoPath(path string) string {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "."
		}
		return cwd
	}
	// Clean and expand the path
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return filepath.Clean(path)
}

// normalizeBaseRef returns the base ref, defaulting to HEAD if empty.
func (s *Server) normalizeBaseRef(ref string) string {
	if ref == "" {
		return "HEAD"
	}
	return ref
}

// normalizeOutputFormat returns the output format, defaulting to sarif if empty or invalid.
func (s *Server) normalizeOutputFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "markdown", "sarif":
		return format
	default:
		return "sarif"
	}
}

// validateDirectory checks if a path exists and is a directory.
func validateDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", path)
		}
		return fmt.Errorf("cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	return nil
}

// validateGitRepo checks if a directory contains a .git folder.
func validateGitRepo(path string) error {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("not a git repository (no .git directory found in %s)", path)
		}
		return fmt.Errorf("cannot access .git directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf(".git is not a directory in %s", path)
	}
	return nil
}

// extractConsoleURL extracts the Kusari console URL from stderr output.
// It looks for patterns like "Once completed, you can see results at: URL"
// or "You can also view your results here: URL"
func extractConsoleURL(stderr string) string {
	// Pattern 1: "Once completed, you can see results at: URL"
	// Pattern 2: "You can also view your results here: URL"
	patterns := []string{
		`(?:Once completed, you can see results at:|You can also view your results here:)\s*(https://[^\s]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(stderr)
		if len(matches) >= 2 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// formatResultWithConsoleURL adds a console URL banner to the results if URL is available.
func formatResultWithConsoleURL(results string, consoleURL string) string {
	if consoleURL == "" {
		return results
	}
	return fmt.Sprintf("View detailed results: %s\n\n---\n\n%s", consoleURL, results)
}

// captureOutput captures stdout and stderr from a function execution.
func captureOutput(fn func() error) (stdout string, stderr string, err error) {
	// Save original stdout/stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	// Create pipes
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	os.Stdout = wOut
	os.Stderr = wErr

	// Capture output in goroutines
	outCh := make(chan string)
	errCh := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		outCh <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		errCh <- buf.String()
	}()

	// Run the function
	err = fn()

	// Restore stdout/stderr and close writers
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	// Get captured output
	stdout = <-outCh
	stderr = <-errCh

	return stdout, stderr, err
}
