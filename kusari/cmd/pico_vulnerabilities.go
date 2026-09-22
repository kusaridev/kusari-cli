// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"

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
			client, err := newPicoClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			result, err := client.GetVulnerabilities(ctx, search, kusariScore, page, size)
			if err != nil {
				return fmt.Errorf("failed to fetch vulnerabilities: %w", err)
			}

			return printJSON(result)
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

			client, err := newPicoClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			result, err := client.GetVulnerabilityByExternalID(ctx, externalID)
			if err != nil {
				return fmt.Errorf("failed to fetch vulnerability: %w", err)
			}

			return printJSON(result)
		},
	}

	return cmd
}
