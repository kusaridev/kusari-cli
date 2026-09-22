// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

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

// printJSON pretty-prints a raw JSON API response to stdout.
func printJSON(raw json.RawMessage) error {
	var formatted any
	if err := json.Unmarshal(raw, &formatted); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	output, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

// parseIDArg parses a positional Kusari Platform ID argument, naming the ID kind (e.g. "SBOM") in the error.
func parseIDArg(arg, name string) (int, error) {
	id, err := strconv.Atoi(arg)
	if err != nil {
		return 0, fmt.Errorf("invalid %s ID: %w", name, err)
	}
	return id, nil
}

// addPaginationFlags registers the --page and --size flags shared by paginated list commands.
// maxSize is the API's upper bound for this endpoint and is only used in the help text.
func addPaginationFlags(cmd *cobra.Command, page, size *int, defaultSize, maxSize int) {
	cmd.Flags().IntVar(page, "page", 0, "Page number, starting at 0")
	cmd.Flags().IntVar(size, "size", defaultSize, fmt.Sprintf("Number of results per page (max %d)", maxSize))
}

// validateSbomTagPair enforces the API rule that sbom_tag_label and sbom_tag_value are supplied together.
func validateSbomTagPair(label, value string) error {
	if (label == "") != (value == "") {
		return fmt.Errorf("--sbom-tag-label and --sbom-tag-value must be supplied together")
	}
	return nil
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
