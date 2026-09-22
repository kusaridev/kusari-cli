// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
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
	cmd.AddCommand(picoSbomIDsByRepo())
	cmd.AddCommand(picoSbomListVersions())
	cmd.AddCommand(picoSbomVersionListTags())
	cmd.AddCommand(picoSbomVersionCreateTag())
	cmd.AddCommand(picoSbomVersionGetTag())
	cmd.AddCommand(picoSbomVersionUpdateTag())
	cmd.AddCommand(picoSbomVersionDeleteTag())
	cmd.AddCommand(picoSbomVersionMoveTag())

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
			sbomID, err := parseIDArg(args[0], "SBOM")
			if err != nil {
				return err
			}

			client, err := newPicoClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			result, err := client.GetSbomVersions(ctx, sbomID, page, size, sort, tagLabel, tagValue, asOf)
			if err != nil {
				return fmt.Errorf("failed to fetch SBOM #%d versions: %w", sbomID, err)
			}

			return printJSON(result)
		},
	}

	addPaginationFlags(cmd, &page, &size, 1000, 1000)
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
			sbomID, err := parseIDArg(args[0], "SBOM")
			if err != nil {
				return err
			}

			versionID, err := parseIDArg(args[1], "version")
			if err != nil {
				return err
			}

			client, err := newPicoClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			result, err := client.ListSbomVersionTags(ctx, sbomID, versionID, page, size, label, active)
			if err != nil {
				return fmt.Errorf("failed to fetch tags for SBOM #%d version #%d: %w", sbomID, versionID, err)
			}

			return printJSON(result)
		},
	}

	addPaginationFlags(cmd, &page, &size, 1000, 1000)
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
			sbomID, err := parseIDArg(args[0], "SBOM")
			if err != nil {
				return err
			}

			versionID, err := parseIDArg(args[1], "version")
			if err != nil {
				return err
			}

			tagLabel := args[2]
			tagValue := args[3]
			if tagLabel == "" || tagValue == "" {
				return fmt.Errorf("label and value must not be empty")
			}

			client, err := newPicoClient()
			if err != nil {
				return err
			}

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
	if sbomID, err = parseIDArg(args[0], "SBOM"); err != nil {
		return 0, 0, 0, err
	}
	if versionID, err = parseIDArg(args[1], "version"); err != nil {
		return 0, 0, 0, err
	}
	if tagID, err = parseIDArg(args[2], "tag"); err != nil {
		return 0, 0, 0, err
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

			client, err := newPicoClient()
			if err != nil {
				return err
			}

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

			client, err := newPicoClient()
			if err != nil {
				return err
			}

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

			client, err := newPicoClient()
			if err != nil {
				return err
			}

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

			client, err := newPicoClient()
			if err != nil {
				return err
			}

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

func picoSbomIDsByRepo() *cobra.Command {
	var (
		forge       string
		org         string
		repo        string
		subrepoPath string
		visibility  string
	)

	cmd := &cobra.Command{
		Use:   "id-by-repo",
		Short: "Find SBOM IDs by repository",
		Long: `Find every SBOM whose upload metadata matches the given forge, org, repo, and (optional) subrepo path.

One repo can hold many SBOMs, so this returns only visible SBOMs by default; use --visibility hidden to return
only hidden ones instead. Returns a 404 error if no SBOM matches.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if forge == "" || org == "" || repo == "" {
				return fmt.Errorf("--forge, --org, and --repo are required")
			}

			client, err := newPicoClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			result, err := client.FindSbomIDsByRepo(ctx, forge, org, repo, subrepoPath, visibility)
			if err != nil {
				return fmt.Errorf("failed to find SBOM IDs for %s/%s/%s: %w", forge, org, repo, err)
			}

			return printJSON(result)
		},
	}

	cmd.Flags().StringVar(&forge, "forge", "", "Forge recorded in the SBOM's upload metadata (required, ex: 'github.com')")
	cmd.Flags().StringVar(&org, "org", "", "Organization recorded in the SBOM's upload metadata (required, ex: 'kusaridev')")
	cmd.Flags().StringVar(&repo, "repo", "", "Repo recorded in the SBOM's upload metadata (required, ex: 'iac')")
	cmd.Flags().StringVar(&subrepoPath, "subrepo-path", "", "Subrepo path recorded in the SBOM's upload metadata (default: none, ex: 'app-code/frontend-console')")
	cmd.Flags().StringVar(&visibility, "visibility", "", "Visibility filter (active|hidden, default: active)")

	return cmd
}

func picoSbomVersionMoveTag() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move-tag <sbom-id> <version-id> <label> <value>",
		Short: "Make a version the only one carrying a tag",
		Long: `Tag the given version with label=value and end that tag on every other version of the SBOM,
in one command. This is the deploy step for a CI pipeline: after a version is deployed to an
environment, move the environment tag to it.

The tags on the other versions are ended at the instant the tag on the given version started, so
the intervals meet with no gap or overlap. That timestamp comes from the server, not this machine.

Safe to rerun: if the given version already carries the tag it is left as is, and any other
versions still carrying it are ended.`,
		Example: `  # After deploying SBOM 42 version 99 to prod:
  kusari platform sboms move-tag 42 99 environment prod`,
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbomID, err := parseIDArg(args[0], "SBOM")
			if err != nil {
				return err
			}

			versionID, err := parseIDArg(args[1], "version")
			if err != nil {
				return err
			}

			tagLabel := args[2]
			tagValue := args[3]
			if tagLabel == "" || tagValue == "" {
				return fmt.Errorf("label and value must not be empty")
			}

			client, err := newPicoClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			res, moveErr := client.MoveSbomVersionTag(ctx, sbomID, versionID, tagLabel, tagValue)
			if res != nil {
				printMoveTagResult(res, sbomID)
			}
			if moveErr != nil {
				return fmt.Errorf("failed to move tag %s=%s on SBOM #%d: %w", tagLabel, tagValue, sbomID, moveErr)
			}
			return nil
		},
	}

	return cmd
}

// printMoveTagResult reports each step MoveSbomVersionTag completed, one line per tag.
func printMoveTagResult(res *pico.MoveTagResult, sbomID int) {
	switch {
	case res.Created != nil:
		t := res.Created
		fmt.Printf("Created tag %d (%s=%s) on SBOM %d version %d, in effect from %s\n", t.ID, t.TagLabel, t.TagValue, sbomID, t.VersionID, t.StartTimestamp)
	case res.Existing != nil:
		t := res.Existing
		fmt.Printf("SBOM %d version %d already carries tag %d (%s=%s) since %s; left as is\n", sbomID, t.VersionID, t.ID, t.TagLabel, t.TagValue, t.StartTimestamp)
	}
	for _, t := range res.Ended {
		fmt.Printf("Ended tag %d (%s=%s) on SBOM %d version %d at %s\n", t.ID, t.TagLabel, t.TagValue, sbomID, t.VersionID, res.EndTimestamp)
	}
	if len(res.Ended) == 0 && (res.Created != nil || res.Existing != nil) {
		fmt.Println("No other versions carried that tag")
	}
}
