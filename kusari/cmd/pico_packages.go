// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kusaridev/kusari-cli/pkg/pico"
	"github.com/spf13/cobra"
)

func packages() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packages",
		Short: "Query packages",
		Long:  "Search packages and retrieve lifecycle information",
	}

	cmd.AddCommand(picoPackagesSearch())
	cmd.AddCommand(picoPackagesLifecycle())

	return cmd
}

func picoPackagesSearch() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "search <name>",
		Short: "Search for packages by name",
		Long:  "Search for a particular package and its associated software",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.SearchPackages(ctx, name, version)
			if err != nil {
				return fmt.Errorf("failed to search packages: %w", err)
			}

			// Pretty print JSON
			var formatted interface{}
			if err := json.Unmarshal(result, &formatted); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			output, err := json.MarshalIndent(formatted, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			fmt.Println(string(output))
			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "Optional version to filter by (supports wildcards like 1.2.*)")

	return cmd
}

func picoPackagesLifecycle() *cobra.Command {
	var isEOL bool
	var isDeprecated bool
	var hasLifecycleRisk bool
	var daysUntilEOLMax int
	var daysUntilEOLMin int
	var ecosystem string
	var softwareID int
	var sortBy string
	var page int
	var size int

	cmd := &cobra.Command{
		Use:   "lifecycle",
		Short: "Get packages filtered by lifecycle status",
		Long:  "Get packages that are EOL, deprecated, or have lifecycle risks",
		RunE: func(cmd *cobra.Command, args []string) error {
			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			// Build query parameters
			params := make(map[string]string)
			if cmd.Flags().Changed("eol") {
				params["is_eol"] = strconv.FormatBool(isEOL)
			}
			if cmd.Flags().Changed("deprecated") {
				params["is_deprecated"] = strconv.FormatBool(isDeprecated)
			}
			if cmd.Flags().Changed("lifecycle-risk") {
				params["has_lifecycle_risk"] = strconv.FormatBool(hasLifecycleRisk)
			}
			if daysUntilEOLMax > 0 {
				params["days_until_eol_max"] = strconv.Itoa(daysUntilEOLMax)
			}
			if daysUntilEOLMin > 0 {
				params["days_until_eol_min"] = strconv.Itoa(daysUntilEOLMin)
			}
			if ecosystem != "" {
				params["ecosystem"] = ecosystem
			}
			if softwareID > 0 {
				params["software_id"] = strconv.Itoa(softwareID)
			}
			if sortBy != "" {
				params["sort"] = sortBy
			}
			params["page"] = strconv.Itoa(page)
			params["size"] = strconv.Itoa(size)

			ctx := context.Background()
			result, err := client.GetPackagesWithLifecycle(ctx, params)
			if err != nil {
				return fmt.Errorf("failed to fetch lifecycle packages: %w", err)
			}

			// Pretty print JSON
			var formatted interface{}
			if err := json.Unmarshal(result, &formatted); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			output, err := json.MarshalIndent(formatted, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			fmt.Println(string(output))
			return nil
		},
	}

	cmd.Flags().BoolVar(&isEOL, "eol", false, "Filter by end-of-life status")
	cmd.Flags().BoolVar(&isDeprecated, "deprecated", false, "Filter by deprecation status")
	cmd.Flags().BoolVar(&hasLifecycleRisk, "lifecycle-risk", false, "Filter by lifecycle risk (EOL or deprecated)")
	cmd.Flags().IntVar(&daysUntilEOLMax, "days-until-eol-max", 0, "Maximum days until EOL")
	cmd.Flags().IntVar(&daysUntilEOLMin, "days-until-eol-min", 0, "Minimum days until EOL")
	cmd.Flags().StringVar(&ecosystem, "ecosystem", "", "Filter by package ecosystem (npm, pypi, golang, maven, cargo)")
	cmd.Flags().IntVar(&softwareID, "software-id", 0, "Filter to packages used by this software ID")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort order (eol_date_asc, eol_date_desc, name_asc, name_desc, impact_desc, impact_asc)")
	cmd.Flags().IntVar(&page, "page", 0, "Page number for pagination")
	cmd.Flags().IntVar(&size, "size", 100, "Number of results per page (max 1000)")

	return cmd
}
