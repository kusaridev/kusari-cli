// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/auth"
	l "github.com/kusaridev/kusari-cli/pkg/login"
	"github.com/spf13/cobra"
)

var selectTenantCmd = &cobra.Command{
	Use:   "select-tenant",
	Short: "Select or change your active tenant",
	Long:  `Select or change your active tenant for the current workspace. This allows you to switch between tenants without re-authenticating.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// Load the token to verify user is authenticated
		token, err := auth.LoadToken("kusari")
		if err != nil {
			return fmt.Errorf("you must be logged in to select a tenant. Run `kusari auth login`")
		}

		if err := auth.CheckTokenExpiry(token); err != nil {
			return err
		}

		// Load current workspace
		currentWorkspace, err := auth.LoadWorkspace(platformUrl, "")
		if err != nil {
			return fmt.Errorf("no workspace selected. Run `kusari auth login` to select a workspace first")
		}

		fmt.Printf("Current workspace: %s\n", currentWorkspace.Description)
		if currentWorkspace.Tenant != "" {
			fmt.Printf("Current tenant: %s\n", currentWorkspace.Tenant)
		}

		// Fetch available workspaces to get tenant list for current workspace
		_, workspaceTenants, err := l.FetchWorkspaces(platformUrl, token.AccessToken)
		if err != nil {
			return fmt.Errorf("failed to fetch workspaces: %w", err)
		}

		// Get tenants for current workspace from the map
		tenants, ok := workspaceTenants[currentWorkspace.ID]
		if !ok || len(tenants) == 0 {
			return fmt.Errorf("no tenants available for this workspace")
		}

		// Prompt user to select tenant
		selectedTenant, err := auth.SelectTenant(tenants)
		if err != nil {
			return fmt.Errorf("failed to select tenant: %w", err)
		}

		// Update the workspace with the new tenant
		currentWorkspace.Tenant = selectedTenant
		if err := auth.SaveWorkspace(*currentWorkspace); err != nil {
			return fmt.Errorf("failed to save tenant selection: %w", err)
		}

		fmt.Printf("\nTenant '%s' has been set as your active tenant.\n", selectedTenant)
		return nil
	},
}

func selectTenant() *cobra.Command {
	return selectTenantCmd
}
