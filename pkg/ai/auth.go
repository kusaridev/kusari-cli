// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package ai

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kusaridev/kusari-cli/pkg/auth"
	"github.com/kusaridev/kusari-cli/pkg/login"
	"github.com/kusaridev/kusari-cli/pkg/port"
)

const (
	// Default auth configuration (matches CLI defaults)
	defaultAuthEndpoint = "https://auth.us.kusari.cloud/"
	defaultClientID     = "4lnk6jccl3hc4lkcudai5lt36u"
)

// isAuthError checks if an error is related to authentication (missing or expired token).
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "no stored tokens found") ||
		strings.Contains(errStr, "Token is expired") ||
		strings.Contains(errStr, "failed to load auth token")
}

// ensureAuthenticated checks if we have a valid token.
// Returns an error if authentication is needed (token missing or expired).
// The caller should handle the error by calling the authenticate tool.
func (s *Server) ensureAuthenticated() error {
	// Try to load existing token
	token, err := auth.LoadToken("kusari")
	if err != nil {
		return fmt.Errorf("authentication required: no stored token found. Please call the 'authenticate' tool to log in")
	}

	// Token exists, check if expired
	if err := auth.CheckTokenExpiry(token); err != nil {
		return fmt.Errorf("authentication required: token is expired. Please call the 'authenticate' tool to refresh your session")
	}

	// Token is valid
	return nil
}

// triggerBrowserAuth opens a browser for OAuth and auto-selects the first workspace.
func (s *Server) triggerBrowserAuth(ctx context.Context) error {
	redirectPort := port.GenerateRandomPortOrDefault()
	redirectURL := fmt.Sprintf("http://localhost:%s/callback", redirectPort)

	fmt.Fprintln(os.Stderr, "[kusari-ai] Opening browser for authentication...")

	// Authenticate - this opens the browser
	token, err := auth.Authenticate(
		ctx,
		defaultClientID,
		"", // no client secret for interactive auth
		redirectURL,
		defaultAuthEndpoint,
		redirectPort,
		s.config.ConsoleURL,
		"", // no pre-selected workspace
	)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Fprintln(os.Stderr, "[kusari-ai] Authentication successful!")

	// Fetch workspaces and auto-select the first one
	workspaces, workspaceTenants, err := login.FetchWorkspaces(s.config.PlatformURL, token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to fetch workspaces: %w", err)
	}

	if len(workspaces) == 0 {
		return fmt.Errorf("no workspaces found for this user")
	}

	// Auto-select first workspace (MCP is non-interactive)
	selectedWorkspace := auth.WorkspaceInfo{
		ID:           workspaces[0].ID,
		Description:  workspaces[0].Description,
		PlatformUrl:  s.config.PlatformURL,
		AuthEndpoint: defaultAuthEndpoint,
	}

	// Auto-select first tenant if available
	if tenants, ok := workspaceTenants[selectedWorkspace.ID]; ok && len(tenants) > 0 {
		selectedWorkspace.Tenant = tenants[0]
	}

	fmt.Fprintf(os.Stderr, "[kusari-ai] Auto-selected workspace: %s\n", selectedWorkspace.Description)
	if selectedWorkspace.Tenant != "" {
		fmt.Fprintf(os.Stderr, "[kusari-ai] Auto-selected tenant: %s\n", selectedWorkspace.Tenant)
	}

	// Inform user how to change workspace/tenant if needed
	if len(workspaces) > 1 {
		fmt.Fprintln(os.Stderr, "[kusari-ai] To change workspace, run: kusari auth select-workspace")
	}
	if tenants, ok := workspaceTenants[selectedWorkspace.ID]; ok && len(tenants) > 1 {
		fmt.Fprintln(os.Stderr, "[kusari-ai] To change tenant, run: kusari auth select-tenant")
	}

	// Save the selected workspace
	if err := auth.SaveWorkspace(selectedWorkspace); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	return nil
}

// AuthenticateResult contains the result of an authentication operation.
type AuthenticateResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Workspace string `json:"workspace,omitempty"`
	Tenant    string `json:"tenant,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleAuthenticate handles the authenticate tool call.
// This opens a browser for OAuth authentication and auto-selects the first workspace.
func (s *Server) handleAuthenticate(ctx context.Context) (*AuthenticateResult, error) {
	// Check if already authenticated
	token, err := auth.LoadToken("kusari")
	if err == nil {
		if err := auth.CheckTokenExpiry(token); err == nil {
			// Already have a valid token
			workspace, _ := auth.LoadWorkspace(s.config.PlatformURL, "")
			return &AuthenticateResult{
				Success:   true,
				Message:   "Already authenticated with a valid token",
				Workspace: workspace.Description,
				Tenant:    workspace.Tenant,
			}, nil
		}
	}

	// Perform authentication
	if err := s.triggerBrowserAuth(ctx); err != nil {
		return &AuthenticateResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Load workspace info to return in result
	workspace, err := auth.LoadWorkspace(s.config.PlatformURL, "")
	if err != nil {
		return &AuthenticateResult{
			Success: true,
			Message: "Authentication successful, but could not load workspace info",
		}, nil
	}

	return &AuthenticateResult{
		Success:   true,
		Message:   "Authentication successful!",
		Workspace: workspace.Description,
		Tenant:    workspace.Tenant,
	}, nil
}
