// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/repo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	wait          bool
	outputFormat  string
	gitlabComment bool
)

func init() {
	scancmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for results")
	scancmd.Flags().StringVarP(&outputFormat, "output-format", "", "markdown", "output format (markdown or sarif)")
	scancmd.Flags().BoolVar(&gitlabComment, "gitlab-comment", false, "post results as a comment to GitLab MR (requires GitLab CI environment)")

	// Bind flags to viper
	mustBindPFlag("wait", scancmd.Flags().Lookup("wait"))
	mustBindPFlag("output-format", scancmd.Flags().Lookup("output-format"))
	mustBindPFlag("gitlab-comment", scancmd.Flags().Lookup("gitlab-comment"))
}

func scan() *cobra.Command {
	scancmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// Validate output format
		if outputFormat != "markdown" && outputFormat != "sarif" {
			return fmt.Errorf("invalid output format: %s (must be 'markdown' or 'sarif')", outputFormat)
		}

		dir := args[0]
		ref := args[1]

		return repo.Scan(dir, ref, platformUrl, consoleUrl, verbose, wait, outputFormat, gitlabComment)
	}

	return scancmd
}

var scancmd = &cobra.Command{
	Use:   "scan <directory> <git-rev>",
	Short: "Scan a change with Kusari Inspector",
	Long: `Generate a change set against a repository, then submit the directory and diff for analysis in Kusari Inspector.
    <directory>  A directory containing a git repository to analyze
    <git-rev>    Git revision to compare to the working tree`,
	Args: cobra.ExactArgs(2),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Update from viper (this gets env vars + config + flags)
		wait = viper.GetBool("wait")
		outputFormat = viper.GetString("output-format")
		gitlabComment = viper.GetBool("gitlab-comment")
	},
}
