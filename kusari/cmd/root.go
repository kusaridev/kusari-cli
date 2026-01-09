// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"

	"github.com/kusaridev/kusari-cli/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	consoleUrl  string
	platformUrl string
	verbose     bool

	// Version information (injected at build time)
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// SetVersionInfo sets the version information for the CLI
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}

func init() {
	cobra.OnInitialize(initConfig)

	// Set version information for the root command
	// This enables the --version flag automatically
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built at: %s)", version, commit, date)

	rootCmd.PersistentFlags().StringVarP(&consoleUrl, "console-url", "", constants.DefaultConsoleURL, "console url")
	rootCmd.PersistentFlags().StringVarP(&platformUrl, "platform-url", "", constants.DefaultPlatformURL, "platform url")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Set environment variable prefix (optional)
	viper.SetEnvPrefix("KUSARI") // Will look for KUSARI_CONSOLE_URL, KUSARI_VERBOSE, etc.
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Bind flags to viper
	mustBindPFlag("console-url", rootCmd.PersistentFlags().Lookup("console-url"))
	mustBindPFlag("platform-url", rootCmd.PersistentFlags().Lookup("platform-url"))
	mustBindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
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
		platformUrl = viper.GetString("platform-url")
		verbose = viper.GetBool("verbose")
	},
}

func Execute() error {

	rootCmd.AddCommand(Auth())
	rootCmd.AddCommand(Repo())
	rootCmd.AddCommand(Platform())
	rootCmd.AddCommand(KusariConfiguration())

	return rootCmd.Execute()
}

func mustBindPFlag(key string, flag *pflag.Flag) {
	if err := viper.BindPFlag(key, flag); err != nil {
		panic(fmt.Sprintf("failed to bind flag %s: %v", key, err))
	}
}
