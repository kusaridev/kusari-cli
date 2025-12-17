// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package login

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kusaridev/kusari-cli/pkg/auth"
	urlBuilder "github.com/kusaridev/kusari-cli/pkg/url"
)

func Login(ctx context.Context, clientId, clientSecret, redirectUrl, authEndpoint, redirectPort, consoleUrl, platformUrl string, verbose bool) error {
	// Store authEndpoint for workspace validation
	currentAuthEndpoint := authEndpoint
	if verbose {
		fmt.Printf(" AuthEndpoint: %s\n", authEndpoint)
		fmt.Printf(" ConsoleUrl: %s\n", consoleUrl)
		fmt.Printf(" PlatformUrl: %s\n", platformUrl)
		fmt.Printf(" ClientId: %s\n", clientId)
		fmt.Printf(" CallbackUrl: %s\n", redirectUrl)
		fmt.Println()
	}

	// Check if there's a previously stored workspace for this platform and auth endpoint
	// This allows us to include the workspace in the redirect URL
	workspaceId := ""
	previousWorkspace, _ := auth.LoadWorkspace(platformUrl, currentAuthEndpoint)
	if previousWorkspace != nil {
		workspaceId = previousWorkspace.ID
		fmt.Printf("\nUsing stored workspace: %s\n", previousWorkspace.Description)
	}

	token, err := auth.Authenticate(ctx, clientId, clientSecret, redirectUrl, authEndpoint, redirectPort, consoleUrl, workspaceId)
	if err != nil {
		return err
	}

	fmt.Println("Successfully logged in!")

	// If we already had a workspace, we're done
	if previousWorkspace != nil {
		fmt.Printf("\nYour current workspace is: %s\n", previousWorkspace.Description)
		fmt.Println("To change workspaces, run: kusari auth select-workspace")
		fmt.Println("\033[1m\033[34mFor more information, visit:\033[0m https://docs.kusari.cloud")
		return nil
	}

	// No workspace stored - fetch available workspaces and prompt user to select
	fmt.Println("\nNo workspace configured. Let's set one up.")
	workspaces, workspaceTenants, err := FetchWorkspaces(platformUrl, token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to fetch workspaces: %w", err)
	}

	// Convert to auth.WorkspaceInfo format and select workspace
	var selectedWorkspace *auth.WorkspaceInfo

	// If client secret is provided (CI/CD mode), auto-select first workspace and tenant
	if clientSecret != "" {
		firstWorkspace := &workspaces[0]
		selectedTenant := ""
		// Get tenants for first workspace from the map
		if tenants, ok := workspaceTenants[firstWorkspace.ID]; ok && len(tenants) > 0 {
			selectedTenant = tenants[0]
		}
		selectedWorkspace = &auth.WorkspaceInfo{
			ID:           firstWorkspace.ID,
			Description:  firstWorkspace.Description,
			PlatformUrl:  platformUrl,
			AuthEndpoint: currentAuthEndpoint,
			Tenant:       selectedTenant,
		}
		fmt.Printf("Auto-selecting workspace for CI/CD: %s\n", selectedWorkspace.Description)
		if selectedTenant != "" {
			fmt.Printf("Auto-selecting tenant: %s\n", selectedTenant)
		}
	} else {
		// Interactive mode - prompt user to select workspace
		authWorkspaces := make([]auth.WorkspaceInfo, len(workspaces))
		for i, ws := range workspaces {
			authWorkspaces[i] = auth.WorkspaceInfo{
				ID:           ws.ID,
				Description:  ws.Description,
				PlatformUrl:  platformUrl,
				AuthEndpoint: currentAuthEndpoint,
			}
		}

		selectedWorkspace, err = auth.SelectWorkspace(authWorkspaces)
		if err != nil {
			return fmt.Errorf("failed to select workspace: %w", err)
		}

		// Get tenants for selected workspace from the map
		if tenants, ok := workspaceTenants[selectedWorkspace.ID]; ok && len(tenants) > 0 {
			selectedTenant, err := auth.SelectTenant(tenants)
			if err != nil {
				return fmt.Errorf("failed to select tenant: %w", err)
			}
			selectedWorkspace.Tenant = selectedTenant
		}
	}

	// Save the selected workspace
	if err := auth.SaveWorkspace(*selectedWorkspace); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	fmt.Printf("\nWorkspace '%s' has been set as your active workspace.\n", selectedWorkspace.Description)
	if selectedWorkspace.Tenant != "" {
		fmt.Printf("Tenant '%s' has been set as your active tenant.\n", selectedWorkspace.Tenant)
	}
	fmt.Println("To change workspaces later, run: kusari auth select-workspace")
	if selectedWorkspace.Tenant != "" {
		fmt.Println("To change tenants later, run: kusari auth select-tenant")
	}

	// Now that we have a workspace, redirect to the console with the workspace parameter
	baseURL, err := urlBuilder.Build(consoleUrl, "/analysis")
	if err == nil && baseURL != nil {
		// Parse the URL and add workspace as query parameter
		parsedURL, parseErr := url.Parse(*baseURL)
		if parseErr == nil {
			query := parsedURL.Query()
			query.Set("workspaceId", selectedWorkspace.ID)
			parsedURL.RawQuery = query.Encode()

			fmt.Println("\nOpening console in your browser...")
			if err := auth.OpenBrowser(parsedURL.String()); err != nil {
				fmt.Printf("Failed to open browser automatically. Please visit: %s\n", parsedURL.String())
			}
		}
	}

	// ANSI escape codes:
	// \033[1m = bold
	// \033[34m = blue
	// \033[0m = reset
	fmt.Println("\033[1m\033[34mFor more information, visit:\033[0m https://docs.kusari.cloud")
	return nil
}

// Workspace represents a workspace with its ID and description
type Workspace struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// FetchWorkspaces retrieves all workspaces and workspace-tenant mapping for the authenticated user
func FetchWorkspaces(platformUrl string, accessToken string) ([]Workspace, map[string][]string, error) {

	userEndpoint, err := urlBuilder.Build(platformUrl, "/user")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build endpoint url: %w", err)
	}
	if userEndpoint == nil {
		return nil, nil, fmt.Errorf("failed to build endpoint url: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", *userEndpoint, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch workspaces: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("failed to fetch workspaces, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	type userInfoResponse struct {
		Workspaces       []Workspace         `json:"workspaces"`
		Groups           []string            `json:"groups"`
		WorkspaceTenants map[string][]string `json:"workspaceTenants"`
	}

	var result userInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal workspaces: %w", err)
	}

	if len(result.Workspaces) == 0 {
		return nil, nil, fmt.Errorf("no workspaces found for this user")
	}

	for i := range result.Workspaces {
		if result.Workspaces[i].Description == "" {
			result.Workspaces[i].Description = "My Workspace"
		}
	}

	return result.Workspaces, result.WorkspaceTenants, nil
}
