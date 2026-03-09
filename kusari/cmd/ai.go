// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
)

// AI returns the parent command for AI integrations.
func AI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI coding assistant integrations",
		Long:  "Install and manage Kusari integrations for AI coding assistants (MCP servers and agent skills)",
	}

	cmd.AddCommand(serve())
	cmd.AddCommand(install())
	cmd.AddCommand(uninstall())
	cmd.AddCommand(list())

	return cmd
}
