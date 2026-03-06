// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server is the MCP server that exposes Kusari Inspector tools.
type Server struct {
	config    *Config
	mcpServer *mcp.Server
	scanQueue chan ScanRequest
	tools     []ToolDefinition
}

// ToolDefinition describes an MCP tool.
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// NewServer creates a new MCP server with the given configuration.
func NewServer(cfg *Config) (*Server, error) {
	if cfg == nil {
		cfg = NewConfig()
	}

	s := &Server{
		config:    cfg,
		scanQueue: make(chan ScanRequest, 10),
	}

	// Initialize MCP server with implementation info
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "kusari-inspector",
			Version: "1.0.0",
		},
		nil,
	)

	s.mcpServer = mcpServer

	// Register tools
	s.registerTools()

	return s, nil
}

// ScanLocalChangesArgs defines the input for scan_local_changes tool.
type ScanLocalChangesArgs struct {
	RepoPath     string `json:"repo_path,omitempty" mcp:"Path to the git repository to scan. Defaults to current directory."`
	BaseRef      string `json:"base_ref,omitempty" mcp:"Base git reference for diff. Defaults to HEAD."`
	OutputFormat string `json:"output_format,omitempty" mcp:"Output format: markdown or sarif. Defaults to sarif."`
}

// ScanFullRepoArgs defines the input for scan_full_repo tool.
type ScanFullRepoArgs struct {
	RepoPath string `json:"repo_path,omitempty" mcp:"Path to the git repository to scan. Defaults to current directory."`
}

// CheckScanStatusArgs defines the input for check_scan_status tool.
type CheckScanStatusArgs struct {
	ScanID string `json:"scan_id" mcp:"required,The scan ID returned from a previous scan operation"`
}

// GetScanResultsArgs defines the input for get_scan_results tool.
type GetScanResultsArgs struct {
	ScanID string `json:"scan_id" mcp:"required,The scan ID to retrieve results for"`
}

// registerTools registers all MCP tools with the server.
func (s *Server) registerTools() {
	s.tools = []ToolDefinition{
		{
			Name:        "scan_local_changes",
			Description: "Scan uncommitted changes in the current git repository for security vulnerabilities, secrets, and SAST issues. This performs a diff-based scan of your local changes using AWS Lambda.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repo_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the git repository to scan. Defaults to current directory.",
					},
					"base_ref": map[string]interface{}{
						"type":        "string",
						"description": "Base git reference for diff (e.g., 'HEAD', 'main', 'origin/main'). Defaults to 'HEAD'.",
						"default":     "HEAD",
					},
					"output_format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"markdown", "sarif"},
						"description": "Output format - 'markdown' for human-readable text or 'sarif' for JSON format. Defaults to 'sarif'.",
						"default":     "sarif",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "scan_full_repo",
			Description: "Perform a comprehensive security audit of the entire repository including OpenSSF Scorecard analysis, full dependency scanning, vulnerability detection, and SAST analysis using AWS Lambda. This is more thorough but takes longer than a diff scan.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repo_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the git repository to scan. Defaults to current directory.",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "check_scan_status",
			Description: "Check the status of a previously submitted scan. Use the scan_id returned from a previous scan operation.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"scan_id": map[string]interface{}{
						"type":        "string",
						"description": "The scan ID returned from a previous scan operation",
					},
				},
				"required": []string{"scan_id"},
			},
		},
		{
			Name:        "get_scan_results",
			Description: "Retrieve detailed results from a completed scan. Returns comprehensive security analysis including vulnerabilities, secrets, SAST findings, and recommendations.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"scan_id": map[string]interface{}{
						"type":        "string",
						"description": "The scan ID to retrieve results for",
					},
				},
				"required": []string{"scan_id"},
			},
		},
	}

	// Register scan_local_changes
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scan_local_changes",
		Description: "Scan uncommitted changes in the current git repository for security vulnerabilities, secrets, and SAST issues. This performs a diff-based scan of your local changes using AWS Lambda.",
	}, s.handleScanLocalChanges)

	// Register scan_full_repo
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scan_full_repo",
		Description: "Perform a comprehensive security audit of the entire repository including OpenSSF Scorecard analysis, full dependency scanning, vulnerability detection, and SAST analysis using AWS Lambda. This is more thorough but takes longer than a diff scan.",
	}, s.handleScanFullRepo)

	// Register check_scan_status
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "check_scan_status",
		Description: "Check the status of a previously submitted scan. Use the scan_id returned from a previous scan operation.",
	}, s.handleCheckScanStatus)

	// Register get_scan_results
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_scan_results",
		Description: "Retrieve detailed results from a completed scan. Returns comprehensive security analysis including vulnerabilities, secrets, SAST findings, and recommendations.",
	}, s.handleGetScanResults)
}

// GetRegisteredTools returns all registered tool definitions.
func (s *Server) GetRegisteredTools() []ToolDefinition {
	return s.tools
}

// Run starts the MCP server using stdio transport.
func (s *Server) Run(ctx context.Context) error {
	if s.config.Verbose {
		fmt.Fprintln(os.Stderr, "Starting Kusari Inspector MCP server...")
	}

	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// Tool handlers - implemented using internal packages

func (s *Server) handleScanLocalChanges(ctx context.Context, req *mcp.CallToolRequest, args ScanLocalChangesArgs) (*mcp.CallToolResult, any, error) {
	result, err := s.executeScanLocalChanges(ctx, args)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result.Results},
		},
	}, nil, nil
}

func (s *Server) handleScanFullRepo(ctx context.Context, req *mcp.CallToolRequest, args ScanFullRepoArgs) (*mcp.CallToolResult, any, error) {
	// Stub - full repo scan (risk-check) is not currently available
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "scan_full_repo is not currently available. Please use scan_local_changes for diff-based security scanning."},
		},
	}, nil, nil
}

func (s *Server) handleCheckScanStatus(ctx context.Context, req *mcp.CallToolRequest, args CheckScanStatusArgs) (*mcp.CallToolResult, any, error) {
	// Stub - CLI mode doesn't support async status checking
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "check_scan_status is not available in CLI mode. Scans complete synchronously."},
		},
	}, nil, nil
}

func (s *Server) handleGetScanResults(ctx context.Context, req *mcp.CallToolRequest, args GetScanResultsArgs) (*mcp.CallToolResult, any, error) {
	// Stub - CLI mode returns results immediately
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "get_scan_results is not available in CLI mode. Results are returned directly from scan operations."},
		},
	}, nil, nil
}
