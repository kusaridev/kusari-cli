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

func install() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [client]",
		Short: "Install AI integrations for a coding agent",
		Long: `Install and configure Kusari integrations for a specific coding agent.

This includes MCP server configuration and agent skills (coming soon).

If no client is specified, an interactive menu will let you select from supported clients.

Supported clients: claude-code, claude-desktop, cline, continue, cursor, windsurf`,
		Example: `  kusari ai install           # Interactive selection
  kusari ai install claude-code    # Install for Claude Code
  kusari ai install cursor    # Install for Cursor`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			withCommitHook, err := cmd.Flags().GetBool("with-commit-hook")
			if err != nil {
				return err
			}

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
			result, err := mcpinstall.InstallWithOptions(client, mcpinstall.InstallOptions{
				WithCommitHook: withCommitHook,
			})
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

	cmd.Flags().Bool("with-commit-hook", false,
		"also install a Claude Code hook that scans with Kusari before Claude makes a git commit (Claude Code only)")

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
	fmt.Println("Kusari AI Integrations - Installation")
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
	printSkillsResult(client, result)
	printCommitHookResult(client, result)
	fmt.Println("Next steps:")

	// Client-specific instructions
	switch client.ID {
	case "claude-code":
		fmt.Println("1. Restart Claude Code so it picks up the new MCP server")
		fmt.Println("2. Run /mcp - you should see 'kusari-inspector' connected")
	case "claude-desktop":
		fmt.Println("1. Restart Claude Desktop to load the new MCP configuration")
		fmt.Println("2. Check that 'kusari-inspector' appears in the connectors list")
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
	if len(result.Skills) > 0 && result.SkillsError == nil {
		fmt.Println("   ...or run /kusari-scan to scan and fix in one step")
	}
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

// printSkillsResult reports what happened to agent skills. Skills are a Claude
// Code construct, so for every other client this says so plainly rather than
// leaving the user to wonder whether something was installed.
func printSkillsResult(client mcpinstall.ClientConfig, result *mcpinstall.InstallationResult) {
	if !result.SkillsSupported {
		fmt.Printf("Agent skills: not supported by %s (MCP server only).\n", client.Name)
		fmt.Println()
		return
	}

	if result.SkillsError != nil {
		fmt.Printf("! Agent skills could not be installed: %v\n", result.SkillsError)
		fmt.Println("  The MCP server is configured and usable; only the skills are missing.")
		fmt.Println()
		return
	}

	if len(result.Skills) == 0 {
		fmt.Println("Agent skills: none available in this build.")
		fmt.Println()
		return
	}

	fmt.Printf("✓ Installed %d agent skill(s) to %s:\n", len(result.Skills), result.SkillsPath)
	for _, name := range result.Skills {
		fmt.Printf("    /%s\n", name)
	}
	fmt.Println()
}

// printCommitHookResult states exactly what was written and where. A hook runs
// a shell command on every matching tool call, so the user is told the script
// path and the settings file rather than just "installed".
func printCommitHookResult(client mcpinstall.ClientConfig, result *mcpinstall.InstallationResult) {
	if !result.CommitHookRequested {
		return
	}

	if result.CommitHookError != nil {
		fmt.Printf("! Commit hook not installed: %v\n", result.CommitHookError)
		fmt.Println("  The MCP server and skills are unaffected.")
		fmt.Println()
		return
	}

	fmt.Println("✓ Installed the pre-commit scan hook:")
	fmt.Printf("    script:   %s\n", result.CommitHookScript)
	fmt.Printf("    settings: %s (hooks.PreToolUse)\n", result.CommitHookSettings)
	fmt.Printf("    binary:   %s\n", result.CommitHookBinary)
	if mcpinstall.TransientBinary(result.CommitHookBinary) {
		fmt.Println("  ! That binary is not what 'kusari' resolves to on your PATH, so it")
		fmt.Println("    looks like a local build. If it is rebuilt elsewhere or removed, the")
		fmt.Println("    hook stops scanning (commits are still allowed through, silently).")
		fmt.Println("    Run 'go install ./kusari' and reinstall to pin the installed binary.")
	}
	fmt.Println("  It runs when Claude is about to make a git commit, and only when")
	fmt.Println("  Kusari reports findings does it stop the commit. Your own commits from")
	fmt.Println("  the terminal are never intercepted.")
	fmt.Printf("  Remove it with: kusari ai uninstall %s\n", client.ID)
	fmt.Println()
}
