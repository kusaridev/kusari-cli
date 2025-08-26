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
	wait        bool
)

func init() {
	scancmd.Flags().StringVarP(&platformUrl, "platform-url", "", "https://platform.api.us.kusari.cloud/", "platform url")
	scancmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for results")
}

func scan() *cobra.Command {
	scancmd.RunE = func(cmd *cobra.Command, args []string) error {
		dir := args[0]
		var diff []string

		// Handle the -- separator case
		if len(args) >= 3 && args[1] == "--" {
			diff = args[2:]
		} else if len(args) >= 2 {
			diff = args[1:]
		} else {
			return fmt.Errorf("not enough arguments")
		}

		if len(diff) == 0 {
			return fmt.Errorf("no git diff command provided")
		}

		return repo.Scan(dir, diff, platformUrl, consoleUrl, verbose, wait)
	}

	return scancmd
}

var scancmd = &cobra.Command{
	Use:   "scan <directory> <git-diff command>",
	Short: "Scan a git diff with Kusari Inspector",
	Long: `Run a git-diff command against a repository, then submit the directory and diff for analysis in Kusari Inspector.
    <directory>     	A directory containing a git repository to analyze
    <git-diff path> 	Git paths to analyze, using git-diff arguments

Use the separator "--" to pass git diff arguments that start with "--":
    kusari repo scan /path/to/repo -- --cached
    kusari repo scan /path/to/repo -- --staged --name-only
    kusari repo scan /path/to/repo -- HEAD^`,
	Args: cobra.MinimumNArgs(2),
}
