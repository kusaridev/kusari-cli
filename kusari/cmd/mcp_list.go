// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"

	"github.com/kusaridev/kusari-cli/pkg/mcpinstall"
	"github.com/spf13/cobra"
)

func list() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List supported coding agents and installation status",
		Long: `Display all supported coding agents and whether Kusari Inspector
is currently installed for each one.`,
		Example: `  kusari mcp list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clients := mcpinstall.GetAllClients()
			statuses := mcpinstall.ListClients(clients)

			printListHeader()
			printClientStatuses(statuses)

			return nil
		},
	}

	return cmd
}

func printListHeader() {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("Kusari Inspector MCP Server - Supported Clients")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
}

func printClientStatuses(statuses []mcpinstall.ClientStatus) {
	// Find the longest client name for alignment
	maxLen := 0
	for _, s := range statuses {
		if len(s.ClientName) > maxLen {
			maxLen = len(s.ClientName)
		}
	}

	installedCount := 0
	for _, status := range statuses {
		icon := "○"
		state := "Not installed"
		if status.Installed {
			icon = "●"
			state = "Installed"
			installedCount++
		}

		// Pad the client name for alignment
		padding := strings.Repeat(" ", maxLen-len(status.ClientName))
		fmt.Printf("  %s %s%s  [%s]\n", icon, status.ClientName, padding, state)

		if verbose && status.ConfigPath != "" {
			fmt.Printf("      Config: %s\n", status.ConfigPath)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("Total: %d clients | Installed: %d | Not installed: %d\n",
		len(statuses), installedCount, len(statuses)-installedCount)
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  kusari mcp install <client>    Install for a specific client")
	fmt.Println("  kusari mcp uninstall <client>  Remove from a specific client")
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
}
