// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	consoleUrl string
	verbose    bool
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&consoleUrl, "console-url", "", "https://console.us.kusari.cloud/", "console url")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Set environment variable prefix (optional)
	viper.SetEnvPrefix("CLI") // Will look for CLI_CONSOLE_URL, CLI_VERBOSE, etc.
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Bind flags to viper
	viper.BindPFlag("console-url", rootCmd.PersistentFlags().Lookup("console-url"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

func initConfig() {
	// Search for .env file in current directory
	viper.AddConfigPath(".")
	viper.SetConfigType("env")
	viper.SetConfigName(".env")

	// Read config file (not fatal if it doesn't exist)
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}

var rootCmd = &cobra.Command{
	Use:   "kusari",
	Short: "Kusari CLI",
	Long:  "Kusari CLI - Interact with Kusari products",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Update from viper (this gets env vars + config + flags)
		consoleUrl = viper.GetString("console-url")
		verbose = viper.GetBool("verbose")
	},
}

func Execute() error {

	rootCmd.AddCommand(Auth())
	rootCmd.AddCommand(Repo())
	rootCmd.AddCommand(KusariConfiguration())

	return rootCmd.Execute()
}
