// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/kusaridev/kusari-cli/pkg/repo"
	"github.com/spf13/cobra"
)

var scanSubprojects bool

func init() {
	riskcheckcmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for results")
	riskcheckcmd.Flags().BoolVar(&scanSubprojects, "scan-subprojects", false, "automatically scan each detected subproject in a monorepo")
}

func riskcheck() *cobra.Command {
	riskcheckcmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		dir := args[0]

		return repo.RiskCheck(dir, platformUrl, consoleUrl, verbose, wait, scanSubprojects)
	}

	return riskcheckcmd
}

var riskcheckcmd = &cobra.Command{
	Use:   "risk-check <directory>",
	Short: "Risk-check a repo with Kusari Inspector",
	Long: `Submit the directory for summary analysis in Kusari Inspector.
    <directory>  A directory containing a git repository to analyze.

For monorepos, use --scan-subprojects to automatically scan all detected subprojects,
or specify a subproject directory directly.`,
	Args: cobra.ExactArgs(1),
}
