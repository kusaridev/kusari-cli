// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/auth"
	l "github.com/kusaridev/kusari-cli/pkg/login"
	"github.com/spf13/cobra"
)

var selectWorkspaceCmd = &cobra.Command{
	Use:   "select-workspace",
	Short: "Select or change your active workspace",
	Long:  `Select or change your active workspace. This allows you to switch between workspaces without re-authenticating.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// Load the token to verify user is authenticated
		token, err := auth.LoadToken("kusari")
		if err != nil {
			return fmt.Errorf("you must be logged in to select a workspace. Run `kusari auth login`")
		}

		if err := auth.CheckTokenExpiry(token); err != nil {
			return err
		}

		// Fetch available workspaces
		workspaces, workspaceTenants, err := l.FetchWorkspaces(platformUrl, token.AccessToken)
		if err != nil {
			return fmt.Errorf("failed to fetch workspaces: %w", err)
		}

		// Convert to auth.WorkspaceInfo format
		authWorkspaces := make([]auth.WorkspaceInfo, len(workspaces))
		for i, ws := range workspaces {
			authWorkspaces[i] = auth.WorkspaceInfo{
				ID:           ws.ID,
				Description:  ws.Description,
				PlatformUrl:  platformUrl,
				AuthEndpoint: authEndpoint,
			}
		}

		// Show current workspace if one exists for this platform and auth endpoint
		currentWorkspace, err := auth.LoadWorkspace(platformUrl, authEndpoint)
		if err == nil {
			fmt.Printf("Current workspace: %s\n", currentWorkspace.Description)
			if currentWorkspace.Tenant != "" {
				fmt.Printf("Current tenant: %s\n", currentWorkspace.Tenant)
			}
		}

		// Prompt user to select workspace
		selectedWorkspace, err := auth.SelectWorkspace(authWorkspaces)
		if err != nil {
			return fmt.Errorf("failed to select workspace: %w", err)
		}

		// Get tenants for selected workspace from the map and prompt for tenant selection
		if tenants, ok := workspaceTenants[selectedWorkspace.ID]; ok && len(tenants) > 0 {
			selectedTenant, err := auth.SelectTenant(tenants)
			if err != nil {
				return fmt.Errorf("failed to select tenant: %w", err)
			}
			selectedWorkspace.Tenant = selectedTenant
		}

		// Save the selected workspace
		if err := auth.SaveWorkspace(*selectedWorkspace); err != nil {
			return fmt.Errorf("failed to save workspace: %w", err)
		}

		fmt.Printf("\nWorkspace '%s' has been set as your active workspace.\n", selectedWorkspace.Description)
		if selectedWorkspace.Tenant != "" {
			fmt.Printf("Tenant '%s' has been set as your active tenant.\n", selectedWorkspace.Tenant)
		}
		return nil
	},
}

func selectWorkspace() *cobra.Command {
	return selectWorkspaceCmd
}
