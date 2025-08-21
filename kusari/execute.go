// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/kusaridev/kusari-cli/kusari/cmd"
	"github.com/spf13/cobra"
)

var (
	consoleUrl string
	verbose    bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&consoleUrl, "console-url", "", "http://console.us.kusari.cloud/", "console url")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
}

var rootCmd = &cobra.Command{
	Use:   "kusari",
	Short: "Kusari - All signal, no noise. No chasing. No surprises. Just secure code, faster.",
	Long:  "Kusari - All signal, no noise. No chasing. No surprises. Just secure code, faster.",
}

func Execute() error {

	rootCmd.AddCommand(cmd.Auth(consoleUrl, verbose))
	rootCmd.AddCommand(cmd.Repo(consoleUrl, verbose))

	return rootCmd.Execute()
}
