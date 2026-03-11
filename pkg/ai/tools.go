// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kusaridev/kusari-cli/pkg/auth"
	"github.com/kusaridev/kusari-cli/pkg/pico"
	"github.com/kusaridev/kusari-cli/pkg/repo"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	if err := s.ensureAuthenticated(); err != nil {
		return nil, fmt.Errorf("authentication required: %w", err)
	}

	if s.config.Verbose {
		fmt.Fprintf(os.Stderr, "[kusari-ai] Scanning local changes in %s (base: %s, format: %s)\n",
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

// getPicoClient returns a Pico client, initializing it if needed.
// Auto-authenticates via browser if no valid credentials are found.
func (s *Server) getPicoClient() (*pico.Client, error) {
	if s.picoClient != nil {
		return s.picoClient, nil
	}

	// Ensure we have valid authentication
	if err := s.ensureAuthenticated(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Load workspace to get tenant
	workspace, err := auth.LoadWorkspace(s.config.PlatformURL, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace: %w", err)
	}

	if workspace.Tenant == "" {
		return nil, fmt.Errorf("this workspace does not have a tenant associated with it. Please run 'kusari auth select-workspace' to select a workspace that has a tenant configured")
	}

	if s.config.Verbose {
		fmt.Fprintf(os.Stderr, "[kusari-ai] Initializing Pico client with tenant: %s\n", workspace.Tenant)
	}

	s.picoClient = pico.NewClient(workspace.Tenant)
	return s.picoClient, nil
}

// handleGetSoftwareIDsByRepo handles the get_software_ids_by_repo tool.
// Uses programmatic traversal to walk up parent directories until software is found.
func (s *Server) handleGetSoftwareIDsByRepo(ctx context.Context, req *mcp.CallToolRequest, args GetSoftwareIDsByRepoArgs) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	repoPath := args.RepoPath
	if repoPath == "" {
		repoPath, _ = os.Getwd()
	}

	// Try current directory first, then traverse upwards
	currentPath := repoPath
	maxDepth := 10 // Prevent infinite loops
	attempts := []string{}

	for i := 0; i < maxDepth; i++ {
		// Extract git repo info from current path
		repoInfo, err := pico.ExtractGitRemoteInfo(currentPath)
		if err != nil {
			// Not a git repo, try parent
			parent := filepath.Dir(currentPath)
			if parent == currentPath {
				// Reached filesystem root
				break
			}
			currentPath = parent
			continue
		}

		attempts = append(attempts, fmt.Sprintf("Attempting: forge=%s, org=%s, repo=%s, subrepo_path=%s",
			repoInfo.Forge, repoInfo.Org, repoInfo.Repo, repoInfo.SubrepoPath))

		// Query API with repo info
		result, err := client.GetSoftwareIDsByRepo(ctx, repoInfo.Forge, repoInfo.Org, repoInfo.Repo, repoInfo.SubrepoPath)
		if err != nil {
			// Try parent directory
			parent := filepath.Dir(currentPath)
			if parent == currentPath {
				// Reached filesystem root
				attempts = append(attempts, fmt.Sprintf("Error at path %s: %v", currentPath, err))
				break
			}
			attempts = append(attempts, fmt.Sprintf("Not found at %s, trying parent directory", currentPath))
			currentPath = parent
			continue
		}

		// Success! Return the result
		var formatted interface{}
		if err := json.Unmarshal(result, &formatted); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		output, err := json.MarshalIndent(formatted, "", "  ")
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		successMsg := fmt.Sprintf("Found software at: %s\n\n%s", currentPath, string(output))
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: successMsg},
			},
		}, nil, nil
	}

	// If we get here, we didn't find any software
	attemptLog := strings.Join(attempts, "\n")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("No software found after traversing parent directories.\n\nSearch attempts:\n%s\n\nMake sure this repository has been uploaded to Kusari platform using 'kusari platform upload'.", attemptLog)},
		},
		IsError: true,
	}, nil, nil
}

// handleGetSoftwareVulnerabilities handles the get_software_vulnerabilities tool.
func (s *Server) handleGetSoftwareVulnerabilities(ctx context.Context, req *mcp.CallToolRequest, args GetSoftwareVulnerabilitiesArgs) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	result, err := client.GetSoftwareVulnerabilities(ctx, args.SoftwareID, args.Page, args.Size)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching vulnerabilities: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	var formatted interface{}
	if err := json.Unmarshal(result, &formatted); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(output)},
		},
	}, nil, nil
}

// handleGetSoftwareVulnerabilityByID handles the get_software_vulnerability_by_id tool.
func (s *Server) handleGetSoftwareVulnerabilityByID(ctx context.Context, req *mcp.CallToolRequest, args GetSoftwareVulnerabilityByIDArgs) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	result, err := client.GetSoftwareVulnerabilityByID(ctx, args.SoftwareID, args.VulnID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching vulnerability details: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	var formatted interface{}
	if err := json.Unmarshal(result, &formatted); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(output)},
		},
	}, nil, nil
}

// handleGetVulnerabilities handles the get_vulnerabilities tool.
func (s *Server) handleGetVulnerabilities(ctx context.Context, req *mcp.CallToolRequest, args GetVulnerabilitiesArgs) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	result, err := client.GetVulnerabilities(ctx, args.Search, args.KusariScore, args.Page, args.Size)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching vulnerabilities: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	var formatted interface{}
	if err := json.Unmarshal(result, &formatted); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(output)},
		},
	}, nil, nil
}

// handleGetVulnerabilityByID handles the get_vulnerability_by_id tool.
func (s *Server) handleGetVulnerabilityByID(ctx context.Context, req *mcp.CallToolRequest, args GetVulnerabilityByIDArgs) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	result, err := client.GetVulnerabilityByExternalID(ctx, args.ExternalID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching vulnerability: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	var formatted interface{}
	if err := json.Unmarshal(result, &formatted); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(output)},
		},
	}, nil, nil
}

// handleSearchPackages handles the search_packages tool.
func (s *Server) handleSearchPackages(ctx context.Context, req *mcp.CallToolRequest, args SearchPackagesArgs) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	result, err := client.SearchPackages(ctx, args.Name, args.Version)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error searching packages: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	var formatted interface{}
	if err := json.Unmarshal(result, &formatted); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(output)},
		},
	}, nil, nil
}

// handleGetSoftwareList handles the get_software_list tool.
func (s *Server) handleGetSoftwareList(ctx context.Context, req *mcp.CallToolRequest, args GetSoftwareListArgs) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	result, err := client.GetSoftwareList(ctx, args.Search, args.Page, args.Size)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching software list: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	var formatted interface{}
	if err := json.Unmarshal(result, &formatted); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(output)},
		},
	}, nil, nil
}

// handleGetSoftwareDetails handles the get_software_details tool.
func (s *Server) handleGetSoftwareDetails(ctx context.Context, req *mcp.CallToolRequest, args GetSoftwareDetailsArgs) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	result, err := client.GetSoftwareByID(ctx, args.SoftwareID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching software details: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	var formatted interface{}
	if err := json.Unmarshal(result, &formatted); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(output)},
		},
	}, nil, nil
}

// handleGetStats handles the get_stats tool.
func (s *Server) handleGetStats(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	result, err := client.GetStats(ctx)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching stats: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	var formatted interface{}
	if err := json.Unmarshal(result, &formatted); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(output)},
		},
	}, nil, nil
}

// handleGetPackagesWithLifecycle handles the get_packages_with_lifecycle tool.
func (s *Server) handleGetPackagesWithLifecycle(ctx context.Context, req *mcp.CallToolRequest, args GetPackagesWithLifecycleArgs) (*mcp.CallToolResult, any, error) {
	client, err := s.getPicoClient()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Build params map
	params := make(map[string]string)
	if args.IsEOL != nil {
		params["is_eol"] = fmt.Sprintf("%t", *args.IsEOL)
	}
	if args.IsDeprecated != nil {
		params["is_deprecated"] = fmt.Sprintf("%t", *args.IsDeprecated)
	}
	if args.HasLifecycleRisk != nil {
		params["has_lifecycle_risk"] = fmt.Sprintf("%t", *args.HasLifecycleRisk)
	}
	if args.DaysUntilEOLMax != nil {
		params["days_until_eol_max"] = fmt.Sprintf("%d", *args.DaysUntilEOLMax)
	}
	if args.DaysUntilEOLMin != nil {
		params["days_until_eol_min"] = fmt.Sprintf("%d", *args.DaysUntilEOLMin)
	}
	if args.Ecosystem != "" {
		params["ecosystem"] = args.Ecosystem
	}
	if args.SoftwareID != nil {
		params["software_id"] = fmt.Sprintf("%d", *args.SoftwareID)
	}
	if args.Sort != "" {
		params["sort"] = args.Sort
	}
	if args.Page > 0 {
		params["page"] = fmt.Sprintf("%d", args.Page)
	}
	if args.Size > 0 {
		params["size"] = fmt.Sprintf("%d", args.Size)
	}

	result, err := client.GetPackagesWithLifecycle(ctx, params)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching packages: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	var formatted interface{}
	if err := json.Unmarshal(result, &formatted); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error parsing response: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error formatting output: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(output)},
		},
	}, nil, nil
}
