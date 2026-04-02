// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	cmd.AddCommand(picoSoftwareCurrent())
	cmd.AddCommand(picoSoftwareVulnerabilities())
	cmd.AddCommand(picoSoftwareVulnerabilityByID())

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
			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

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

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

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

func picoSoftwareCurrent() *cobra.Command {
	var repoPath string

	cmd := &cobra.Command{
		Use:   "current",
		Short: "Find software IDs for the current repository",
		Long:  "Find software IDs by extracting repository information from git remote (forge, org, repo, subrepo_path)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if platformTenant == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			// Extract git remote info
			repoInfo, err := pico.ExtractGitRemoteInfo(repoPath)
			if err != nil {
				return fmt.Errorf("failed to extract git remote info: %w", err)
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "Querying software for:\n")
				fmt.Fprintf(os.Stderr, "  Forge: %s\n", repoInfo.Forge)
				fmt.Fprintf(os.Stderr, "  Org: %s\n", repoInfo.Org)
				fmt.Fprintf(os.Stderr, "  Repo: %s\n", repoInfo.Repo)
				fmt.Fprintf(os.Stderr, "  Subrepo Path: %s\n", repoInfo.SubrepoPath)
			}

			client := pico.NewClient(platformTenant)

			ctx := context.Background()
			result, err := client.GetSoftwareIDsByRepo(ctx, repoInfo.Forge, repoInfo.Org, repoInfo.Repo, repoInfo.SubrepoPath)
			if err != nil {
				return fmt.Errorf("failed to fetch software IDs: %w", err)
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

	cmd.Flags().StringVar(&repoPath, "repo-path", "", "Path to git repository (defaults to current directory)")

	return cmd
}

func picoSoftwareVulnerabilities() *cobra.Command {
	var page int
	var size int

	cmd := &cobra.Command{
		Use:   "vulnerabilities <software-id>",
		Short: "Get vulnerabilities for a software",
		Long:  "Get paginated list of vulnerabilities affecting a specific software/application by its ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			softwareID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid software ID: %w", err)
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.GetSoftwareVulnerabilities(ctx, softwareID, page, size)
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

	return cmd
}

func picoSoftwareVulnerabilityByID() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vulnerability <software-id> <vuln-id>",
		Short: "Get detailed vulnerability information for a software",
		Long:  "Get detailed information about how a specific vulnerability affects a specific software, including remediation plans",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			softwareID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid software ID: %w", err)
			}

			vulnID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid vulnerability ID: %w", err)
			}

			if platformTenantEndpoint == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenantEndpoint)

			ctx := context.Background()
			result, err := client.GetSoftwareVulnerabilityByID(ctx, softwareID, vulnID)
			if err != nil {
				return fmt.Errorf("failed to fetch software vulnerability details: %w", err)
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
