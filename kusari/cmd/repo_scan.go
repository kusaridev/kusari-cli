// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/kusaridev/iac/app-code/kusari-cli/pkg/repo"
	"github.com/spf13/cobra"
)

var (
	platformUrl string
)

func init() {
	scan.Flags().StringVarP(&platformUrl, "platform-url", "", "https://platform.api.us.kusari.cloud/", "platform url")
}

var scan = &cobra.Command{
	Use:   "scan <directory> <git-diff command>",
	Short: "Package directory and diff, then submit for analysis",
	Long: `Run a git-diff command against a repository, then package the directory and diff. Submit for diff analysis in Kusar Inspector.
    <directory>        Should be a Git directory of code to be analyzed
    <git-diff command> Should be the arguments to provide to git-diff to determine what diff to analyze`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("not enough arguments")
		}
		dir := args[0]
		diff := args[1:]

		return repo.Scan(dir, diff, platformUrl)
	},
}
