// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
)

// MCP returns the parent command for MCP server operations.
func MCP() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server operations",
		Long:  "Manage the Kusari Inspector MCP server for AI coding assistants",
	}

	cmd.AddCommand(serve())
	cmd.AddCommand(install())
	cmd.AddCommand(uninstall())
	cmd.AddCommand(list())

	return cmd
}
