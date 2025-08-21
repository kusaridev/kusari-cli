// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
)

func Repo(consoleUrl string, verbose bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Repository operations",
		Long:  "Handle repository scanning and packaging operations",
	}

	cmd.AddCommand(scan(consoleUrl, verbose))

	return cmd
}
