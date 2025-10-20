// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/kusaridev/kusari-cli/pkg/repo"
	"github.com/spf13/cobra"
)

func init() {
	riskcheckcmd.Flags().StringVarP(&platformUrl, "platform-url", "", "https://platform.api.us.kusari.cloud/", "platform url")
	riskcheckcmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for results")
}

func riskcheck() *cobra.Command {
	riskcheckcmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		dir := args[0]

		return repo.RiskCheck(dir, platformUrl, consoleUrl, verbose, wait)
	}

	return riskcheckcmd
}

var riskcheckcmd = &cobra.Command{
	Use:   "risk-check <directory>",
	Short: "Risk-check a repo with Kusari Inspector",
	Long: `Submit the directory for summary analysis in Kusari Inspector.
    <directory>  A directory containing a git repository to analyze`,
	Args: cobra.ExactArgs(1),
}
