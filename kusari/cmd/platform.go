// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"

	"github.com/kusaridev/kusari-cli/v2/pkg/auth"
	"github.com/kusaridev/kusari-cli/v2/pkg/pico"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	platformTenantEndpoint string
	platformTenant         string
)

func init() {
	platformCmd.PersistentFlags().StringVarP(&platformTenantEndpoint, "tenant-endpoint", "t", "", "Kusari Tenant endpoint URL (for dev/testing, overrides --tenant)")
	platformCmd.PersistentFlags().StringVar(&platformTenant, "tenant", "", "Tenant name (e.g., 'demo' for https://demo.api.us.kusari.cloud)")

	// Bind flags to viper
	mustBindPFlag("tenant-endpoint", platformCmd.PersistentFlags().Lookup("tenant-endpoint"))
	mustBindPFlag("tenant", platformCmd.PersistentFlags().Lookup("tenant"))
}

// newPicoClient returns a Pico API client for the configured tenant endpoint.
// It returns an error if no tenant was resolved from --tenant-endpoint, --tenant, or the workspace config.
func newPicoClient() (*pico.Client, error) {
	if platformTenantEndpoint == "" {
		return nil, fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
	}
	return pico.NewClient(platformTenantEndpoint), nil
}

func Platform() *cobra.Command {
	platformCmd.AddCommand(upload())
	platformCmd.AddCommand(vulnerabilities())
	platformCmd.AddCommand(packages())
	platformCmd.AddCommand(software())
	platformCmd.AddCommand(components())
	platformCmd.AddCommand(generate())
	platformCmd.AddCommand(sboms())

	return platformCmd
}

var platformCmd = &cobra.Command{
	Use:   "platform",
	Short: "Kusari platform operations",
	Long:  "Handle interactions with the Kusari platform operations ",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Update from viper (this gets env vars + config + flags)
		platformTenantEndpoint = viper.GetString("tenant-endpoint")
		platformTenant = viper.GetString("tenant")

		// If tenant-endpoint is provided, use it directly (for dev/testing)
		if platformTenantEndpoint != "" {
			return
		}

		// If tenant is provided via flag, construct the endpoint
		if platformTenant != "" {
			platformTenantEndpoint = fmt.Sprintf("https://%s.api.us.kusari.cloud", platformTenant)
			return
		}

		// Neither flag provided - try to load from workspace config
		workspace, err := auth.LoadWorkspace(platformUrl, "")
		if err != nil {
			// Store the error to provide helpful message later if command fails
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: Could not load workspace configuration: %v\n", err)
			}
			return
		}

		if workspace.Tenant != "" {
			platformTenant = workspace.Tenant
			platformTenantEndpoint = fmt.Sprintf("https://%s.api.us.kusari.cloud", platformTenant)
		} else if verbose {
			fmt.Fprintf(os.Stderr, "Warning: Workspace loaded but no tenant configured\n")
		}
	},
}
