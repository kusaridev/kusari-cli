// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/repo"
	"github.com/spf13/cobra"
)

var (
	platformUrl string
)

func init() {
	scancmd.Flags().StringVarP(&platformUrl, "platform-url", "", "https://platform.api.us.kusari.cloud/", "platform url")
}

func scan() *cobra.Command {
	scancmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("not enough arguments")
		}
		dir := args[0]
		diff := args[1:]

		return repo.Scan(dir, diff, platformUrl, consoleUrl, verbose)
	}

	return scancmd
}

var scancmd = &cobra.Command{
	Use:   "scan <directory> <git-diff command>",
	Short: "Scan a git diff with Kusari Inspector",
	Long: `Run a git-diff command against a repository, then submit the directory and diff for analysis in Kusai Inspector.
    <directory>        A directory containing a git repository to analyze
    <git-diff path> Git paths to analyze, using git-difff arguments`,
	Args: cobra.MinimumNArgs(2),
}
