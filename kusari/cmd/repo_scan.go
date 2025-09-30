// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/kusaridev/kusari-cli/pkg/repo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	platformUrl string
	wait        bool
	ci          bool
)

func init() {
	scancmd.Flags().StringVarP(&platformUrl, "platform-url", "", "https://platform.api.us.kusari.cloud/", "platform url")
	scancmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for results")
	scancmd.Flags().BoolVarP(&ci, "ci", "c", false, "kusari cli running in a pipeline")

	// Bind flags to viper
	viper.BindPFlag("platform-url", scancmd.PersistentFlags().Lookup("platform-url"))
	viper.BindPFlag("wait", scancmd.PersistentFlags().Lookup("wait"))
	viper.BindPFlag("ci", scancmd.PersistentFlags().Lookup("ci"))
}

func scan() *cobra.Command {
	scancmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		dir := args[0]
		ref := args[1]

		return repo.Scan(dir, ref, platformUrl, consoleUrl, verbose, wait, ci)
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
		platformUrl = viper.GetString("platform-url")
		wait = viper.GetBool("wait")
		ci = viper.GetBool("ci")
	},
}
