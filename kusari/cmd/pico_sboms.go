// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kusaridev/kusari-cli/v2/pkg/pico"
	"github.com/spf13/cobra"
)

func sboms() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sboms",
		Short: "Query SBOMs",
		Long:  "List and retrieve information about internal SBOMs, and their versions and tags",
	}

	cmd.AddCommand(picoSbomIDGetVersions())

	return cmd
}

func picoSbomIDGetVersions() *cobra.Command {
	var page int
	var size int
	var sort string
	var tag_label string
	var tag_value string
	var as_of string

	cmd := &cobra.Command{
		Use:   "versions <sbom-id>",
		Short: "Get versions of an SBOM",
		Long:  "Get paginated list of all historical versions of an SBOM by its ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbomID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid SBOM ID: %w", err)
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.GetSbomIDVersions(ctx, sbomID, page, size, sort, tag_label, tag_value, as_of)
			if err != nil {
				return fmt.Errorf("failed to fetch software vulnerabilities: %w", err)
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

	cmd.Flags().IntVar(&page, "page", 0, "Page number (default: 0)")
	cmd.Flags().IntVar(&size, "size", 1000, "Page size (default: 1000)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort (default: newest first)")
	cmd.Flags().StringVar(&tag_label, "sbom_tag_label", "", "SBOM tag label (default: none, ex: 'environment')")
	cmd.Flags().StringVar(&tag_value, "sbom_tag_value", "", "SBOM tag value (default: none, ex: 'dev')")
	cmd.Flags().StringVar(&as_of, "as_of", "", "As of date-time (default: none, ex: 'dev')")

	return cmd
}
