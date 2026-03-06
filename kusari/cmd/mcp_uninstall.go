// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/kusaridev/kusari-cli/pkg/mcpinstall"
	"github.com/spf13/cobra"
)

func uninstall() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall [client]",
		Short: "Uninstall the MCP server from a coding agent",
		Long: `Remove the Kusari Inspector MCP server configuration from a specific coding agent.

If no client is specified, an interactive menu will let you select from supported clients.

Supported clients: claude, cursor, windsurf, cline, continue`,
		Example: `  kusari mcp uninstall           # Interactive selection
  kusari mcp uninstall claude    # Uninstall from Claude Code
  kusari mcp uninstall cursor    # Uninstall from Cursor`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var clientID string

			if len(args) > 0 {
				clientID = args[0]
			} else {
				// Interactive client selection
				selected, err := selectClientForUninstall()
				if err != nil {
					return err
				}
				clientID = selected
			}

			client, err := mcpinstall.GetClient(clientID)
			if err != nil {
				return fmt.Errorf("invalid client: %s\n\nSupported clients: claude, cursor, windsurf, cline, continue", clientID)
			}

			// Print header
			printUninstallHeader(client)

			// Perform uninstallation
			result, err := mcpinstall.Uninstall(client)
			if err != nil {
				return fmt.Errorf("uninstallation failed: %w", err)
			}

			if !result.Success {
				return fmt.Errorf("uninstallation failed: %s", result.Message)
			}

			// Print success message
			printUninstallSuccess(client, result)

			return nil
		},
	}

	return cmd
}

// selectClientForUninstall presents an interactive menu for selecting a coding agent.
func selectClientForUninstall() (string, error) {
	clients := mcpinstall.GetAllClients()

	options := make([]huh.Option[string], len(clients))
	for i, c := range clients {
		options[i] = huh.NewOption(c.Name, c.ID)
	}

	var selected string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a coding agent to uninstall from").
				Description("Use arrow keys to navigate, enter to select").
				Options(options...).
				Value(&selected),
		),
	)

	err := form.Run()
	if err != nil {
		return "", err
	}

	return selected, nil
}

func printUninstallHeader(client mcpinstall.ClientConfig) {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("Kusari Inspector MCP Server - Uninstallation")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Printf("Removing from: %s\n", client.Name)
	fmt.Printf("Platform: %s\n", mcpinstall.GetPlatform())
	fmt.Println()
}

func printUninstallSuccess(client mcpinstall.ClientConfig, result *mcpinstall.InstallationResult) {
	fmt.Printf("✓ %s\n", result.Message)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("Uninstallation Complete!")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Printf("Kusari Inspector has been removed from %s.\n", client.Name)
	fmt.Println()

	if result.NeedsRestart {
		fmt.Println("Note: You may need to restart your coding agent to apply the changes.")
		fmt.Println()
	}

	fmt.Println("To reinstall, run: kusari mcp install " + client.ID)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))

	if verbose {
		fmt.Printf("\nConfig file: %s\n", result.ConfigPath)
	}
}
