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
	}

	// Register scan_local_changes
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scan_local_changes",
		Description: "Scan uncommitted changes in the current git repository for security vulnerabilities, secrets, and SAST issues. This performs a diff-based scan of your local changes using AWS Lambda.",
	}, s.handleScanLocalChanges)
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
