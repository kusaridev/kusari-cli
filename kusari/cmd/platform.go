// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
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
	platformCmd.PersistentFlags().StringVar(&platformTenant, "tenant", "", "Tenant name (e.g., 'demo' for https://demo.api.us.kusari.cloud; the host follows --platform-url)")

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
	// json.Indent re-formats the bytes as sent, keeping the server's key order and full integer
	// precision, unlike a round trip through map[string]any.
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Println(out.String())
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
// maxSize is the API's upper bound for this endpoint and is only used in the help text; pass 0 when
// the spec declares no maximum.
func addPaginationFlags(cmd *cobra.Command, page, size *int, defaultSize, maxSize int) {
	cmd.Flags().IntVar(page, "page", 0, "Page number, starting at 0")
	sizeHelp := "Number of results per page"
	if maxSize > 0 {
		sizeHelp = fmt.Sprintf("%s (max %d)", sizeHelp, maxSize)
	}
	cmd.Flags().IntVar(size, "size", defaultSize, sizeHelp)
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Update from viper (this gets env vars + config + flags)
		platformTenantEndpoint = viper.GetString("tenant-endpoint")
		platformTenant = viper.GetString("tenant")

		// If tenant-endpoint is provided, use it directly (for dev/testing)
		if platformTenantEndpoint != "" {
			return nil
		}

		// If tenant is provided via flag, build the endpoint from the platform URL
		if platformTenant != "" {
			endpoint, err := pico.TenantEndpoint(platformUrl, platformTenant)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}
			platformTenantEndpoint = endpoint
			return nil
		}

		// Neither flag provided - try to load from workspace config
		workspace, err := auth.LoadWorkspace(platformUrl, "")
		if err != nil {
			// Store the error to provide helpful message later if command fails
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: Could not load workspace configuration: %v\n", err)
			}
			return nil
		}

		if workspace.Tenant == "" {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: Workspace loaded but no tenant configured\n")
			}
			return nil
		}

		// Not every platform subcommand needs a tenant, so a platform URL we can't build from
		// is a warning here; commands that do need one fail later with "no tenant configured".
		platformTenant = workspace.Tenant
		endpoint, err := pico.TenantEndpoint(platformUrl, platformTenant)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			return nil
		}
		platformTenantEndpoint = endpoint
		return nil
	},
}
