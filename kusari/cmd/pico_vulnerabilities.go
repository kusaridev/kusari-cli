// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/pico"
	"github.com/spf13/cobra"
)

func vulnerabilities() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vulnerabilities",
		Short: "Query vulnerabilities",
		Long:  "List and retrieve vulnerability information from the Pico API",
	}

	cmd.AddCommand(picoVulnerabilitiesList())
	cmd.AddCommand(picoVulnerabilitiesGet())

	return cmd
}

func picoVulnerabilitiesList() *cobra.Command {
	var search string
	var kusariScore string
	var page int
	var size int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List vulnerabilities",
		Long:  "List vulnerabilities with optional filters for search and severity",
		RunE: func(cmd *cobra.Command, args []string) error {
			if platformTenant == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenant)

			ctx := context.Background()
			result, err := client.GetVulnerabilities(ctx, search, kusariScore, page, size)
			if err != nil {
				return fmt.Errorf("failed to fetch vulnerabilities: %w", err)
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

	cmd.Flags().StringVar(&search, "search", "", "Search glob for affected/vulnerable package name")
	cmd.Flags().StringVar(&kusariScore, "kusari-score", "", "Minimum Kusari score (0-10)")
	cmd.Flags().IntVar(&page, "page", 0, "Page number for pagination")
	cmd.Flags().IntVar(&size, "size", 20, "Number of results per page (max 100)")

	return cmd
}

func picoVulnerabilitiesGet() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <external-id>",
		Short: "Get vulnerability by external ID",
		Long:  "Get detailed information about a specific vulnerability by its external ID (CVE, GHSA, GO-, etc.)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			externalID := args[0]

			if platformTenant == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenant)

			ctx := context.Background()
			result, err := client.GetVulnerabilityByExternalID(ctx, externalID)
			if err != nil {
				return fmt.Errorf("failed to fetch vulnerability: %w", err)
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

	return cmd
}
