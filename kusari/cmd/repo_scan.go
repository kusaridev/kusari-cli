// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/kusaridev/kusari-cli/pkg/repo"
	"github.com/spf13/cobra"
)

var (
	platformUrl   string
	wait          bool
	clientSecret  string
	tokenEndpoint string
)

func init() {
	scancmd.Flags().StringVarP(&platformUrl, "platform-url", "", "https://platform.api.us.kusari.cloud/", "platform url")
	scancmd.Flags().StringVarP(&clientId, "client-id", "c", "4lnk6jccl3hc4lkcudai5lt36u", "OAuth2 client ID")
	scancmd.Flags().StringVarP(&clientSecret, "client-secret", "s", "", "OAuth client secret ")
	scancmd.Flags().StringVarP(&tokenEndpoint, "token-endpoint", "k", "https://kusari.api.us.kusari.cloud", "Token endpoint URL")
	scancmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for results")
}

func scan() *cobra.Command {
	scancmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		dir := args[0]
		ref := args[1]

		return repo.Scan(cmd.Context(), dir, ref, platformUrl, consoleUrl, verbose, wait, clientId, clientSecret, tokenEndpoint)
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
}
