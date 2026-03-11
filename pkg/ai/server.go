// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/kusaridev/kusari-cli/pkg/auth"
	"github.com/kusaridev/kusari-cli/pkg/pico"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server is the MCP server that exposes Kusari Inspector tools.
type Server struct {
	config     *Config
	mcpServer  *mcp.Server
	scanQueue  chan ScanRequest
	tools      []ToolDefinition
	picoClient *pico.Client
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

	// Initialize Pico client - load tenant from workspace
	workspace, err := auth.LoadWorkspace(cfg.PlatformURL, "")
	if err == nil && workspace.Tenant != "" {
		s.picoClient = pico.NewClient(workspace.Tenant)
	}
	// Note: picoClient may be nil if not authenticated yet, handlers will check

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

// GetSoftwareIDsByRepoArgs defines the input for get_software_ids_by_repo tool.
type GetSoftwareIDsByRepoArgs struct {
	RepoPath string `json:"repo_path,omitempty" mcp:"Path to the git repository. Defaults to current directory. Will traverse parent directories if not found."`
}

// GetSoftwareVulnerabilitiesArgs defines the input for get_software_vulnerabilities tool.
type GetSoftwareVulnerabilitiesArgs struct {
	SoftwareID int `json:"software_id" mcp:"The ID of the software to retrieve vulnerabilities for."`
	Page       int `json:"page,omitempty" mcp:"Page number for pagination (default 0)."`
	Size       int `json:"size,omitempty" mcp:"Number of results per page (default 20, max 100)."`
}

// GetSoftwareVulnerabilityByIDArgs defines the input for get_software_vulnerability_by_id tool.
type GetSoftwareVulnerabilityByIDArgs struct {
	SoftwareID int `json:"software_id" mcp:"The ID of the software."`
	VulnID     int `json:"vuln_id" mcp:"The ID of the vulnerability."`
}

// GetVulnerabilitiesArgs defines the input for get_vulnerabilities tool.
type GetVulnerabilitiesArgs struct {
	Search      string `json:"search,omitempty" mcp:"Search glob for affected/vulnerable package name."`
	KusariScore string `json:"kusari_score,omitempty" mcp:"Minimum Kusari score to filter on (0-10)."`
	Page        int    `json:"page,omitempty" mcp:"Page number for pagination (default 0)."`
	Size        int    `json:"size,omitempty" mcp:"Number of results per page (default 20, max 100)."`
}

// GetVulnerabilityByIDArgs defines the input for get_vulnerability_by_id tool.
type GetVulnerabilityByIDArgs struct {
	ExternalID string `json:"external_id" mcp:"The external vulnerability identifier (CVE, GHSA, GO-, etc.)."`
}

// SearchPackagesArgs defines the input for search_packages tool.
type SearchPackagesArgs struct {
	Name    string `json:"name" mcp:"Package name to search for."`
	Version string `json:"version,omitempty" mcp:"Optional version to filter by."`
}

// GetSoftwareListArgs defines the input for get_software_list tool.
type GetSoftwareListArgs struct {
	Search string `json:"search" mcp:"Search term to filter software by name."`
	Page   int    `json:"page,omitempty" mcp:"Page number for pagination (default 0)."`
	Size   int    `json:"size,omitempty" mcp:"Number of results per page (default 20, max 100)."`
}

// GetSoftwareDetailsArgs defines the input for get_software_details tool.
type GetSoftwareDetailsArgs struct {
	SoftwareID int `json:"software_id" mcp:"The ID of the software to retrieve details for."`
}

// GetPackagesWithLifecycleArgs defines the input for get_packages_with_lifecycle tool.
type GetPackagesWithLifecycleArgs struct {
	IsEOL              *bool  `json:"is_eol,omitempty" mcp:"Filter by end-of-life status."`
	IsDeprecated       *bool  `json:"is_deprecated,omitempty" mcp:"Filter by deprecation status."`
	HasLifecycleRisk   *bool  `json:"has_lifecycle_risk,omitempty" mcp:"Returns packages that are deprecated, EOL, or have upcoming EOL date."`
	DaysUntilEOLMax    *int   `json:"days_until_eol_max,omitempty" mcp:"Maximum days until EOL."`
	DaysUntilEOLMin    *int   `json:"days_until_eol_min,omitempty" mcp:"Minimum days until EOL."`
	Ecosystem          string `json:"ecosystem,omitempty" mcp:"Filter by package ecosystem (npm, pypi, golang, maven, cargo)."`
	SoftwareID         *int   `json:"software_id,omitempty" mcp:"Filter to packages used by this specific software ID."`
	Sort               string `json:"sort,omitempty" mcp:"Sort order (default: impact_desc)."`
	Page               int    `json:"page,omitempty" mcp:"Page number for pagination (default 0)."`
	Size               int    `json:"size,omitempty" mcp:"Number of results per page (default 100, max 1000)."`
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
			Name:        "get_software_ids_by_repo",
			Description: "STEP 1: Find software IDs for the current repository. Use this FIRST when the user asks about vulnerabilities affecting them/their code. Automatically traverses parent directories in monorepos to find registered software. Returns software IDs needed for get_software_vulnerabilities.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repo_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the git repository. Defaults to current directory. Will traverse parent directories if software not found at this path.",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "get_software_vulnerabilities",
			Description: "STEP 2: Get all vulnerabilities affecting a specific software by its ID (from get_software_ids_by_repo). Use this to answer 'what vulnerabilities are affecting me?'. Returns list with vulnerability IDs, severity, CVEs, and affected packages.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"software_id": map[string]interface{}{
						"type":        "number",
						"description": "The ID of the software to retrieve vulnerabilities for (from get_software_ids_by_repo response).",
					},
					"page": map[string]interface{}{
						"type":        "number",
						"description": "Page number for pagination (default 0).",
					},
					"size": map[string]interface{}{
						"type":        "number",
						"description": "Number of results per page (default 20, max 100).",
					},
				},
				"required": []string{"software_id"},
			},
		},
		{
			Name:        "get_software_vulnerability_by_id",
			Description: "STEP 3: Get detailed fix information for a specific vulnerability. Use when user wants to fix/remediate a vulnerability. Returns AI-generated remediation plan, exploit details, affected code paths, and step-by-step fix guidance.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"software_id": map[string]interface{}{
						"type":        "number",
						"description": "The ID of the software (from get_software_ids_by_repo).",
					},
					"vuln_id": map[string]interface{}{
						"type":        "number",
						"description": "The vulnerability ID (use the 'id' field from get_software_vulnerabilities response, NOT the CVE).",
					},
				},
				"required": []string{"software_id", "vuln_id"},
			},
		},
		{
			Name:        "get_vulnerabilities",
			Description: "Get a list of vulnerabilities affecting the user's software. REQUIRED: Must provide either search term OR kusari_score parameter - never call without filters. Returns vulnerability details including severity, affected packages, and fix information.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"search": map[string]interface{}{
						"type":        "string",
						"description": "Search glob for affected/vulnerable package name (e.g., 'lodash', 'react'). Required if kusari_score not provided.",
					},
					"kusari_score": map[string]interface{}{
						"type":        "string",
						"description": "Minimum Kusari score to filter on (0-10). Returns vulnerabilities with score >= this value. Use 9-10 for critical, 7-8 for high severity. Required if search not provided.",
					},
					"page": map[string]interface{}{
						"type":        "number",
						"description": "Page number for pagination (default 0)",
					},
					"size": map[string]interface{}{
						"type":        "number",
						"description": "Number of results per page (default 20, max 100)",
					},
				},
			},
		},
		{
			Name:        "get_vulnerability_by_id",
			Description: "Get detailed information about a specific vulnerability by its external ID (CVE, GHSA, GO-, etc.). Use this when the user asks about a specific CVE or vulnerability ID.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"external_id": map[string]interface{}{
						"type":        "string",
						"description": "The external vulnerability identifier (e.g., CVE-2023-12345, GHSA-xxxx-xxxx-xxxx, GO-2022-0635)",
					},
				},
				"required": []string{"external_id"},
			},
		},
		{
			Name:        "search_packages",
			Description: "Search for packages by name to see if the user is using a specific dependency. Supports fuzzy matching. Returns package details including vulnerability count and version info.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Package name to search for (e.g., lodash, react, express)",
					},
					"version": map[string]interface{}{
						"type":        "string",
						"description": "Optional version to filter by (supports wildcards like 1.2.*)",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "get_software_list",
			Description: "Get a list of internal software/applications being tracked by the user. REQUIRED: Must provide search term - never call without filter. Software refers to applications they created or control (not open source dependencies).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"search": map[string]interface{}{
						"type":        "string",
						"description": "REQUIRED: Search term to filter software by name",
					},
					"page": map[string]interface{}{
						"type":        "number",
						"description": "Page number for pagination (default 0)",
					},
					"size": map[string]interface{}{
						"type":        "number",
						"description": "Number of results per page (default 20, max 100)",
					},
				},
				"required": []string{"search"},
			},
		},
		{
			Name:        "get_software_details",
			Description: "Get detailed information about a specific software/application including its vulnerabilities and dependencies.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"software_id": map[string]interface{}{
						"type":        "number",
						"description": "The ID of the software to retrieve details for",
					},
				},
				"required": []string{"software_id"},
			},
		},
		{
			Name:        "get_stats",
			Description: "Get aggregate statistics about the user's vulnerabilities including counts by severity. Good for getting an overview of security posture.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_packages_with_lifecycle",
			Description: "Get packages filtered by lifecycle status. Use is_eol for EOL packages, is_deprecated for deprecated packages. Only use has_lifecycle_risk if user explicitly asks for BOTH EOL and deprecated together. Packages are open source dependencies (not internal software).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"is_eol": map[string]interface{}{
						"type":        "boolean",
						"description": "PREFERRED for EOL queries. Filter by end-of-life status. true = EOL packages only.",
					},
					"is_deprecated": map[string]interface{}{
						"type":        "boolean",
						"description": "PREFERRED for deprecated queries. Filter by deprecation status. true = deprecated packages only.",
					},
					"has_lifecycle_risk": map[string]interface{}{
						"type":        "boolean",
						"description": "ONLY use when user asks for BOTH EOL AND deprecated together. Returns packages that are deprecated, EOL, or have upcoming EOL date.",
					},
					"days_until_eol_max": map[string]interface{}{
						"type":        "number",
						"description": "Maximum days until EOL. Returns packages with EOL date within this many days.",
					},
					"days_until_eol_min": map[string]interface{}{
						"type":        "number",
						"description": "Minimum days until EOL. Returns packages with EOL date at least this many days away.",
					},
					"ecosystem": map[string]interface{}{
						"type":        "string",
						"description": "Filter by package ecosystem (npm, pypi, golang, maven, cargo)",
					},
					"software_id": map[string]interface{}{
						"type":        "number",
						"description": "Filter to packages used by this specific software ID",
					},
					"sort": map[string]interface{}{
						"type":        "string",
						"description": "Sort order: eol_date_asc, eol_date_desc, name_asc, name_desc, impact_desc, impact_asc (default: impact_desc)",
					},
					"page": map[string]interface{}{
						"type":        "number",
						"description": "Page number for pagination (default 0)",
					},
					"size": map[string]interface{}{
						"type":        "number",
						"description": "Number of results per page (default 100, max 1000)",
					},
				},
			},
		},
	}

	// Register scan_local_changes
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "scan_local_changes",
		Description: "Scan uncommitted changes in the current git repository for security vulnerabilities, secrets, and SAST issues. This performs a diff-based scan of your local changes using AWS Lambda.",
	}, s.handleScanLocalChanges)

	// Register Pico API tools - Priority tools (vulnerability workflow)
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_software_ids_by_repo",
		Description: "STEP 1: Find software IDs for the current repository. Use this FIRST when the user asks about vulnerabilities affecting them/their code.",
	}, s.handleGetSoftwareIDsByRepo)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_software_vulnerabilities",
		Description: "STEP 2: Get all vulnerabilities affecting a specific software by its ID (from get_software_ids_by_repo). Use this to answer 'what vulnerabilities are affecting me?'.",
	}, s.handleGetSoftwareVulnerabilities)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_software_vulnerability_by_id",
		Description: "STEP 3: Get detailed fix information for a specific vulnerability. Use when user wants to fix/remediate a vulnerability. Returns AI-generated remediation plan.",
	}, s.handleGetSoftwareVulnerabilityByID)

	// Register remaining Pico API tools
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_vulnerabilities",
		Description: "Get a list of vulnerabilities affecting the user's software. REQUIRED: Must provide either search term OR kusari_score parameter - never call without filters.",
	}, s.handleGetVulnerabilities)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_vulnerability_by_id",
		Description: "Get detailed information about a specific vulnerability by its external ID (CVE, GHSA, GO-, etc.).",
	}, s.handleGetVulnerabilityByID)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "search_packages",
		Description: "Search for packages by name to see if the user is using a specific dependency.",
	}, s.handleSearchPackages)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_software_list",
		Description: "Get a list of internal software/applications being tracked by the user. REQUIRED: Must provide search term.",
	}, s.handleGetSoftwareList)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_software_details",
		Description: "Get detailed information about a specific software/application including its vulnerabilities and dependencies.",
	}, s.handleGetSoftwareDetails)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_stats",
		Description: "Get aggregate statistics about the user's vulnerabilities including counts by severity.",
	}, s.handleGetStats)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_packages_with_lifecycle",
		Description: "Get packages filtered by lifecycle status (EOL, deprecated, etc.).",
	}, s.handleGetPackagesWithLifecycle)
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
