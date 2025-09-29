// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/kusaridev/kusari-cli/pkg/repo"
	"github.com/spf13/cobra"
)

func init() {
	fullscancmd.Flags().StringVarP(&platformUrl, "platform-url", "", "https://platform.api.us.kusari.cloud/", "platform url")
	fullscancmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for results")
}

func fullscan() *cobra.Command {
	fullscancmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		dir := args[0]

		return repo.FullScan(dir, platformUrl, consoleUrl, verbose, wait)
	}

	return fullscancmd
}

var fullscancmd = &cobra.Command{
	Use:   "full-scan <directory>",
	Short: "Scan a full repo with Kusari Inspector",
	Long: `Submit the directory for summary analysis in Kusari Inspector.
    <directory>  A directory containing a git repository to analyze`,
	Args: cobra.ExactArgs(1),
}
