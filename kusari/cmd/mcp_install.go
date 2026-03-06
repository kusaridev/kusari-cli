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

func install() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [client]",
		Short: "Install the MCP server for a coding agent",
		Long: `Install and configure the Kusari Inspector MCP server for a specific coding agent.

If no client is specified, an interactive menu will let you select from supported clients.

Supported clients: claude, cursor, windsurf, cline, continue`,
		Example: `  kusari mcp install           # Interactive selection
  kusari mcp install claude    # Install for Claude Code
  kusari mcp install cursor    # Install for Cursor`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var clientID string

			if len(args) > 0 {
				clientID = args[0]
			} else {
				// Interactive client selection
				selected, err := selectClient()
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
			printInstallHeader(client)

			// Perform installation
			result, err := mcpinstall.Install(client)
			if err != nil {
				return fmt.Errorf("installation failed: %w", err)
			}

			if !result.Success {
				return fmt.Errorf("installation failed: %s", result.Message)
			}

			// Print success message
			printInstallSuccess(client, result)

			return nil
		},
	}

	return cmd
}

// selectClient presents an interactive menu for selecting a coding agent.
func selectClient() (string, error) {
	clients := mcpinstall.GetAllClients()

	options := make([]huh.Option[string], len(clients))
	for i, c := range clients {
		options[i] = huh.NewOption(c.Name, c.ID)
	}

	var selected string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a coding agent to configure").
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

func printInstallHeader(client mcpinstall.ClientConfig) {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("Kusari Inspector MCP Server - Installation")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Printf("Configuring for: %s\n", client.Name)
	fmt.Printf("Platform: %s\n", mcpinstall.GetPlatform())
	fmt.Println()
}

func printInstallSuccess(client mcpinstall.ClientConfig, result *mcpinstall.InstallationResult) {
	fmt.Printf("✓ %s\n", result.Message)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("Installation Complete!")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Printf("Kusari Inspector has been configured for %s.\n", client.Name)
	fmt.Println()
	fmt.Println("Next steps:")

	// Client-specific instructions
	switch client.ID {
	case "claude":
		fmt.Println("1. Reload VS Code: Cmd+Shift+P → 'Developer: Reload Window'")
		fmt.Println("2. Check MCP status - you should see 'kusari-inspector' running")
	case "cursor":
		fmt.Println("1. Restart Cursor to load the new MCP configuration")
		fmt.Println("2. The kusari-inspector server will be available in Cursor")
	case "windsurf":
		fmt.Println("1. Restart Windsurf to load the new MCP configuration")
		fmt.Println("2. The kusari-inspector server will be available in Windsurf")
	case "cline":
		fmt.Println("1. Reload VS Code: Cmd+Shift+P → 'Developer: Reload Window'")
		fmt.Println("2. Cline will now have access to kusari-inspector")
	case "continue":
		fmt.Println("1. Reload VS Code: Cmd+Shift+P → 'Developer: Reload Window'")
		fmt.Println("2. Continue will now have access to kusari-inspector")
	default:
		fmt.Println("1. Restart your coding agent to load the new configuration")
	}

	fmt.Println()
	fmt.Println("3. Ask your AI assistant: 'Scan my local changes for security issues'")
	fmt.Println()
	fmt.Println("For authentication:")
	fmt.Println("- On first use, your browser will open to authenticate with Kusari")
	fmt.Println("- Credentials are saved to ~/.kusari/tokens.json")
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))

	if verbose {
		fmt.Printf("\nConfig file: %s\n", result.ConfigPath)
	}
}
