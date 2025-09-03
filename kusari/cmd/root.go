// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
)

var (
	consoleUrl string
	verbose    bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&consoleUrl, "console-url", "", "https://console.us.kusari.cloud/", "console url")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
}

var rootCmd = &cobra.Command{
	Use:   "kusari",
	Short: "Kusari CLI",
	Long:  "Kusari CLI - Interact with Kusari products",
}

func Execute() error {

	rootCmd.AddCommand(Auth())
	rootCmd.AddCommand(Repo())
	rootCmd.AddCommand(KusariConfiguration())

	return rootCmd.Execute()
}
