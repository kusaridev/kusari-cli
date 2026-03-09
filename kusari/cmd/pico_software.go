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

func software() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "software",
		Short: "Query software/applications",
		Long:  "List and retrieve information about internal software/applications",
	}

	cmd.AddCommand(picoSoftwareList())
	cmd.AddCommand(picoSoftwareGet())

	return cmd
}

func picoSoftwareList() *cobra.Command {
	var search string
	var page int
	var size int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List software/applications",
		Long:  "List internal software/applications being tracked",
		RunE: func(cmd *cobra.Command, args []string) error {
			if platformTenant == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenant)

			ctx := context.Background()
			result, err := client.GetSoftwareList(ctx, search, page, size)
			if err != nil {
				return fmt.Errorf("failed to fetch software: %w", err)
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

	cmd.Flags().StringVar(&search, "search", "", "Search term to filter software by name")
	cmd.Flags().IntVar(&page, "page", 0, "Page number for pagination")
	cmd.Flags().IntVar(&size, "size", 20, "Number of results per page (max 100)")

	return cmd
}

func picoSoftwareGet() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <software-id>",
		Short: "Get software by ID",
		Long:  "Get detailed information about a specific software/application including its vulnerabilities and dependencies",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			softwareID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid software ID: %w", err)
			}

			if platformTenant == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenant)

			ctx := context.Background()
			result, err := client.GetSoftwareByID(ctx, softwareID)
			if err != nil {
				return fmt.Errorf("failed to fetch software: %w", err)
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
