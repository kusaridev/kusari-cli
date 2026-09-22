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
	cmd.AddCommand(picoSbomVersionListTags())
	cmd.AddCommand(picoSbomVersionCreateTag())

	return cmd
}

func picoSbomIDGetVersions() *cobra.Command {
	var page int
	var size int
	var sort string
	var tagLabel string
	var tagValue string
	var asOf string

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
			result, err := client.GetSbomIDVersions(ctx, sbomID, page, size, sort, tagLabel, tagValue, asOf)
			if err != nil {
				return fmt.Errorf("failed to fetch SBOM #%d versions: %w", sbomID, err)
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
	cmd.Flags().StringVar(&sort, "sort", "", "Sort order (default: newest first). One of: sbom_time_desc, sbom_time_asc, sbom_type_desc, sbom_type_asc, first_ingested_desc, first_ingested_asc")
	cmd.Flags().StringVar(&tagLabel, "sbom-tag-label", "", "SBOM tag label (default: none, ex: 'environment')")
	cmd.Flags().StringVar(&tagValue, "sbom-tag-value", "", "SBOM tag value (default: none, ex: 'prod')")
	cmd.Flags().StringVar(&asOf, "as-of", "", "As of date-time (default: none, ex: '2025-01-01T00:00:00Z')")

	return cmd
}

func picoSbomVersionListTags() *cobra.Command {
	var page int
	var size int
	var label string
	var active bool

	cmd := &cobra.Command{
		Use:   "tags <sbom-id> <version-id>",
		Short: "Get tags on an SBOM version",
		Long:  "Get paginated list of tags applied to a specific SBOM version. A tag is an interval: it applies from start_timestamp until end_timestamp, and an absent end_timestamp means it is still in effect. Ended tags are included by default; use --active to return only tags still in effect.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbomID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid SBOM ID: %w", err)
			}

			versionID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid version ID: %w", err)
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.ListSbomVersionTags(ctx, sbomID, versionID, page, size, label, active)
			if err != nil {
				return fmt.Errorf("failed to fetch tags for SBOM #%d version #%d: %w", sbomID, versionID, err)
			}

			return printJSON(result)
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (default: 0)")
	cmd.Flags().IntVar(&size, "size", 1000, "Page size (default: 1000)")
	cmd.Flags().StringVar(&label, "label", "", "Only return tags with this exact label, case-insensitive (default: none, ex: 'environment')")
	cmd.Flags().BoolVar(&active, "active", false, "Only return tags still in effect (no end_timestamp)")

	return cmd
}

func picoSbomVersionCreateTag() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-tag <sbom-id> <version-id> <label> <value>",
		Short: "Add a tag to an SBOM version",
		Long:  "Create a tag (label/value pair) on a specific SBOM version. The server sets the start timestamp and lowercases the label and value. Fails with 409 if the version already has the same label and value still in effect.",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbomID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid SBOM ID: %w", err)
			}

			versionID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid version ID: %w", err)
			}

			tagLabel := args[2]
			tagValue := args[3]
			if tagLabel == "" || tagValue == "" {
				return fmt.Errorf("label and value must not be empty")
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.CreateSbomVersionTag(ctx, sbomID, versionID, tagLabel, tagValue)
			if err != nil {
				return fmt.Errorf("failed to create tag on SBOM #%d version #%d: %w", sbomID, versionID, err)
			}

			return printJSON(result)
		},
	}

	return cmd
}
