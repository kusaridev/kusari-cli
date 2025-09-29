// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
)

func Repo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Repository operations",
		Long:  "Handle repository scanning and packaging operations",
	}

	cmd.AddCommand(scan())
	cmd.AddCommand(fullscan())

	return cmd
}
