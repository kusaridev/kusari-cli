// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/kusaridev/kusari-cli/v2/pkg/pico"
	"github.com/spf13/cobra"
)

func sboms() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sboms",
		Short: "Query SBOMs",
		Long:  "List and retrieve information about internal SBOMs, and their versions and tags",
	}

	cmd.AddCommand(picoSbomIDsByIdentifier())
	cmd.AddCommand(picoSbomListVersions())
	cmd.AddCommand(picoSbomVersionListTags())
	cmd.AddCommand(picoSbomVersionCreateTag())
	cmd.AddCommand(picoSbomVersionGetTag())
	cmd.AddCommand(picoSbomVersionUpdateTag())
	cmd.AddCommand(picoSbomVersionDeleteTag())

	return cmd
}

func picoSbomListVersions() *cobra.Command {
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
			result, err := client.GetSbomVersions(ctx, sbomID, page, size, sort, tagLabel, tagValue, asOf)
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

// parseSbomTagIDs parses the <sbom-id> <version-id> <tag-id> positional args shared by the per-tag commands.
func parseSbomTagIDs(args []string) (sbomID, versionID, tagID int, err error) {
	sbomID, err = strconv.Atoi(args[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid SBOM ID: %w", err)
	}

	versionID, err = strconv.Atoi(args[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid version ID: %w", err)
	}

	tagID, err = strconv.Atoi(args[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid tag ID: %w", err)
	}

	return sbomID, versionID, tagID, nil
}

// parseTimestampFlag validates an RFC3339 timestamp flag value. "now" is replaced with the current UTC time.
func parseTimestampFlag(name, value string) (string, error) {
	if value == "now" {
		return time.Now().UTC().Format(time.RFC3339), nil
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return "", fmt.Errorf("invalid --%s: must be RFC3339 (ex: '2025-01-01T00:00:00Z') or 'now': %w", name, err)
	}
	return value, nil
}

func picoSbomVersionGetTag() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag <sbom-id> <version-id> <tag-id>",
		Short: "Get a specific tag on an SBOM version",
		Long:  "Get a single tag by ID. The tag must belong to the given SBOM version.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbomID, versionID, tagID, err := parseSbomTagIDs(args)
			if err != nil {
				return err
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.GetSbomVersionTag(ctx, sbomID, versionID, tagID)
			if err != nil {
				return fmt.Errorf("failed to fetch tag #%d on SBOM #%d version #%d: %w", tagID, sbomID, versionID, err)
			}

			return printJSON(result)
		},
	}

	return cmd
}

func picoSbomVersionUpdateTag() *cobra.Command {
	var (
		tagLabel string
		tagValue string
		start    string
		end      string
		reopen   bool
	)

	cmd := &cobra.Command{
		Use:   "update-tag <sbom-id> <version-id> <tag-id>",
		Short: "Update a tag on an SBOM version",
		Long: `Partially update a tag. Only the flags provided are written; everything else keeps its stored value.

To end a tag (stop it applying while keeping its history), set --end. Use '--end now' to end it at the current time.
To re-open a closed tag, pass --reopen. --end must be after the tag's start timestamp and must not be in the future.
Label and value are lowercased by the server. Fails with 409 if the change would leave the version carrying the same label and value open twice at once.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbomID, versionID, tagID, err := parseSbomTagIDs(args)
			if err != nil {
				return err
			}

			labelSet := cmd.Flags().Changed("label")
			valueSet := cmd.Flags().Changed("value")
			startSet := cmd.Flags().Changed("start")
			endSet := cmd.Flags().Changed("end")
			if !labelSet && !valueSet && !startSet && !endSet && !reopen {
				return fmt.Errorf("at least one of --label, --value, --start, --end, or --reopen must be provided")
			}
			if endSet && reopen {
				return fmt.Errorf("--end and --reopen are mutually exclusive")
			}

			body := map[string]any{}
			if labelSet {
				if tagLabel == "" {
					return fmt.Errorf("--label must not be empty")
				}
				body["tag_label"] = tagLabel
			}
			if valueSet {
				if tagValue == "" {
					return fmt.Errorf("--value must not be empty")
				}
				body["tag_value"] = tagValue
			}
			if startSet {
				ts, err := parseTimestampFlag("start", start)
				if err != nil {
					return err
				}
				body["start_timestamp"] = ts
			}
			if endSet {
				ts, err := parseTimestampFlag("end", end)
				if err != nil {
					return err
				}
				body["end_timestamp"] = ts
			}
			if reopen {
				body["end_timestamp"] = nil
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.UpdateSbomVersionTag(ctx, sbomID, versionID, tagID, body)
			if err != nil {
				return fmt.Errorf("failed to update tag #%d on SBOM #%d version #%d: %w", tagID, sbomID, versionID, err)
			}

			return printJSON(result)
		},
	}

	cmd.Flags().StringVar(&tagLabel, "label", "", "New tag label (ex: 'environment')")
	cmd.Flags().StringVar(&tagValue, "value", "", "New tag value (ex: 'production')")
	cmd.Flags().StringVar(&start, "start", "", "New start timestamp, RFC3339 or 'now' (ex: '2025-01-01T00:00:00Z')")
	cmd.Flags().StringVar(&end, "end", "", "End timestamp to close the tag, RFC3339 or 'now' (ex: '2025-01-01T00:00:00Z')")
	cmd.Flags().BoolVar(&reopen, "reopen", false, "Clear the end timestamp so a closed tag is in effect again")

	return cmd
}

func picoSbomVersionDeleteTag() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-tag <sbom-id> <version-id> <tag-id>",
		Short: "Delete a tag on an SBOM version",
		Long:  "Permanently remove a tag, including any record that it ever applied. To stop a tag applying while keeping its history, use update-tag --end instead.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbomID, versionID, tagID, err := parseSbomTagIDs(args)
			if err != nil {
				return err
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			if err := client.DeleteSbomVersionTag(ctx, sbomID, versionID, tagID); err != nil {
				return fmt.Errorf("failed to delete tag #%d on SBOM #%d version #%d: %w", tagID, sbomID, versionID, err)
			}

			fmt.Printf("Tag %d on SBOM %d version %d deleted\n", tagID, sbomID, versionID)
			return nil
		},
	}

	return cmd
}

func picoSbomIDsByIdentifier() *cobra.Command {
	var commitSha string

	cmd := &cobra.Command{
		Use:   "id-by-identifier",
		Short: "Find SBOM and version IDs by identifier",
		Long: `Find the SBOM ID and version ID of every SBOM version matching the given identifier.

Currently only --commit-sha is supported. This is an exact lookup, so it also returns versions of hidden SBOMs.
Returns a 404 error if no version matches.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if commitSha == "" {
				return fmt.Errorf("--commit-sha is required")
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.FindSbomIDsByIdentifier(ctx, commitSha)
			if err != nil {
				return fmt.Errorf("failed to find SBOM IDs for commit %s: %w", commitSha, err)
			}

			return printJSON(result)
		},
	}

	cmd.Flags().StringVar(&commitSha, "commit-sha", "", "Commit SHA recorded on the SBOM at ingestion time (required)")

	return cmd
}
