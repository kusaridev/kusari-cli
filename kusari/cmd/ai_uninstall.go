// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/kusaridev/kusari-cli/v2/pkg/mcpinstall"
	"github.com/spf13/cobra"
)

func uninstall() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall [client]",
		Short: "Uninstall AI integrations from a coding agent",
		Long: `Remove Kusari integrations from a specific coding agent.

This includes MCP server configuration and agent skills.

If no client is specified, an interactive menu will let you select from supported clients.

Supported clients: claude-code, claude-desktop, cline, continue, cursor, windsurf`,
		Example: `  kusari ai uninstall           # Interactive selection
  kusari ai uninstall claude-code    # Uninstall from Claude Code
  kusari ai uninstall cursor    # Uninstall from Cursor`,
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
	fmt.Println("Kusari AI Integrations - Uninstallation")
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

	if result.SkillsSupported {
		switch {
		case result.SkillsError != nil:
			fmt.Printf("! Agent skills could not be removed: %v\n", result.SkillsError)
			fmt.Printf("  Remove them by hand from %s if needed.\n", result.SkillsPath)
		case len(result.Skills) > 0:
			fmt.Printf("✓ Removed %d agent skill(s) from %s.\n", len(result.Skills), result.SkillsPath)
		default:
			fmt.Println("Agent skills: none were installed.")
		}
		fmt.Println()
	}

	switch {
	case result.CommitHookError != nil:
		fmt.Printf("! Commit hook could not be removed: %v\n", result.CommitHookError)
		fmt.Println()
	case result.CommitHookInstalled:
		fmt.Println("✓ Removed the pre-commit scan hook and its settings entry.")
		fmt.Println()
	}

	if result.NeedsRestart {
		fmt.Println("Note: You may need to restart your coding agent to apply the changes.")
		fmt.Println()
	}

	fmt.Println("To reinstall, run: kusari ai install " + client.ID)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))

	if verbose {
		fmt.Printf("\nConfig file: %s\n", result.ConfigPath)
	}
}
