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

func components() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "components",
		Short: "Manage components",
		Long:  "Manage components in the Kusari platform",
	}

	cmd.AddCommand(picoComponentsList())
	cmd.AddCommand(picoComponentsGet())
	cmd.AddCommand(picoComponentsCreate())
	cmd.AddCommand(picoComponentsUpdate())
	cmd.AddCommand(picoComponentsDelete())
	cmd.AddCommand(picoComponentsAssignSoftware())
	cmd.AddCommand(picoComponentsRemoveSoftware())

	return cmd
}

func picoComponentsList() *cobra.Command {
	var (
		search       string
		statusFilter string
		filter       string
		sort         string
		tags         string
		excludeTags  string
		hasTags      bool
		page         int
		size         int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List components",
		Long:  "List components with optional filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			params := make(map[string]string)
			if search != "" {
				params["search"] = search
			}
			if statusFilter != "" {
				params["status_filter"] = statusFilter
			}
			if filter != "" {
				params["filter"] = filter
			}
			if sort != "" {
				params["sort"] = sort
			}
			if tags != "" {
				params["tags"] = tags
			}
			if excludeTags != "" {
				params["exclude_tags"] = excludeTags
			}
			if cmd.Flags().Changed("has-tags") {
				params["has_tags"] = strconv.FormatBool(hasTags)
			}
			if page >= 0 {
				params["page"] = strconv.Itoa(page)
			}
			if size > 0 {
				params["size"] = strconv.Itoa(size)
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.ListComponents(ctx, params)
			if err != nil {
				return fmt.Errorf("failed to fetch components: %w", err)
			}

			return printJSON(result)
		},
	}

	cmd.Flags().StringVar(&search, "search", "", "Search glob for component display name (case-insensitive)")
	cmd.Flags().StringVar(&statusFilter, "status-filter", "", "Filter by EOL/deprecated status (all|eol|deprecated)")
	cmd.Flags().StringVar(&filter, "filter", "", "Visibility filter (active|hidden)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort order (e.g. display_name_asc, vuln_count_desc)")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tag IDs to include (OR semantics)")
	cmd.Flags().StringVar(&excludeTags, "exclude-tags", "", "Comma-separated tag IDs to exclude")
	cmd.Flags().BoolVar(&hasTags, "has-tags", false, "Only tagged (true) or only untagged (false) components; omit the flag to disable filter")
	cmd.Flags().IntVar(&page, "page", 0, "Page number for pagination")
	cmd.Flags().IntVar(&size, "size", 1000, "Number of results per page (max 1000)")

	return cmd
}

func picoComponentsGet() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <component-id>",
		Short: "Get a component by ID",
		Long:  "Get detailed information about a specific component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			compID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid component ID: %w", err)
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.GetComponentByID(ctx, compID)
			if err != nil {
				return fmt.Errorf("failed to fetch component: %w", err)
			}

			return printJSON(result)
		},
	}

	return cmd
}

func picoComponentsCreate() *cobra.Command {
	var (
		displayName string
		metaJSON    string
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a component",
		Long:  "Create a new component with the given name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			meta, err := parseMetaFlag(cmd.Flags().Changed("meta"), metaJSON)
			if err != nil {
				return err
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.CreateComponent(ctx, name, displayName, meta)
			if err != nil {
				return fmt.Errorf("failed to create component: %w", err)
			}

			return printJSON(result)
		},
	}

	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name (defaults to <name>)")
	cmd.Flags().StringVar(&metaJSON, "meta", "", "Arbitrary metadata as a JSON object string")

	return cmd
}

func picoComponentsUpdate() *cobra.Command {
	var (
		displayName string
		metaJSON    string
	)

	cmd := &cobra.Command{
		Use:   "update <component-id>",
		Short: "Update a component",
		Long:  "Update the display_name and/or meta fields of a component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			compID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid component ID: %w", err)
			}

			displayNameSet := cmd.Flags().Changed("display-name")
			metaSet := cmd.Flags().Changed("meta")
			if !displayNameSet && !metaSet {
				return fmt.Errorf("at least one of --display-name or --meta must be provided")
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			var displayNamePtr *string
			if displayNameSet {
				displayNamePtr = &displayName
			}

			meta, err := parseMetaFlag(metaSet, metaJSON)
			if err != nil {
				return err
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			if err := client.UpdateComponent(ctx, compID, displayNamePtr, meta); err != nil {
				return fmt.Errorf("failed to update component: %w", err)
			}

			fmt.Printf("Component %d updated\n", compID)
			return nil
		},
	}

	cmd.Flags().StringVar(&displayName, "display-name", "", "New display name")
	cmd.Flags().StringVar(&metaJSON, "meta", "", "Replacement metadata as a JSON object string")

	return cmd
}

func picoComponentsDelete() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <component-id>",
		Short: "Delete a component",
		Long:  "Unassign all software from the component and delete it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			compID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid component ID: %w", err)
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			if err := client.DeleteComponent(ctx, compID); err != nil {
				return fmt.Errorf("failed to delete component: %w", err)
			}

			fmt.Printf("Component %d deleted\n", compID)
			return nil
		},
	}

	return cmd
}

func picoComponentsAssignSoftware() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign-software <component-id> <software-id> [<software-id>...]",
		Short: "Assign software to a component",
		Long:  "Bulk-assign one or more software entries to a component. Each software is moved from any prior component into the target component. Atomic — if any software ID does not exist, no changes are made. Maximum 100 software IDs per call.",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			compID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid component ID: %w", err)
			}

			softwareIDs := make([]int, 0, len(args)-1)
			for _, a := range args[1:] {
				id, err := strconv.Atoi(a)
				if err != nil {
					return fmt.Errorf("invalid software ID %q: %w", a, err)
				}
				softwareIDs = append(softwareIDs, id)
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			if err := client.AssignSoftwareToComponent(ctx, compID, softwareIDs); err != nil {
				return fmt.Errorf("failed to assign software to component: %w", err)
			}

			fmt.Printf("Assigned %d software ID(s) to component %d\n", len(softwareIDs), compID)
			return nil
		},
	}

	return cmd
}

func picoComponentsRemoveSoftware() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-software <component-id> <software-id>",
		Short: "Remove a software entry from a component",
		Long:  "Remove the link between a component and a single software entry. Returns an error if no such link exists.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			compID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid component ID: %w", err)
			}

			softwareID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid software ID: %w", err)
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			if err := client.RemoveSoftwareFromComponent(ctx, compID, softwareID); err != nil {
				return fmt.Errorf("failed to remove software from component: %w", err)
			}

			fmt.Printf("Software %d removed from component %d\n", softwareID, compID)
			return nil
		},
	}

	return cmd
}

// parseMetaFlag returns the parsed --meta JSON object, or nil if the flag was not set.
// An explicit empty string is rejected with a clear message rather than surfacing a raw
// "unexpected end of JSON input" error.
func parseMetaFlag(set bool, metaJSON string) (map[string]any, error) {
	if !set {
		return nil, nil
	}
	if metaJSON == "" {
		return nil, fmt.Errorf("--meta cannot be empty; use '{}' to set an empty object, or omit the flag")
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		return nil, fmt.Errorf("invalid --meta JSON: %w", err)
	}
	return meta, nil
}

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
