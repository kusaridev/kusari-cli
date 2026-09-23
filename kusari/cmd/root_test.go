// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A subcommand with its own PersistentPreRun (like `auth login` or `platform`) must still get the
// root-level values from viper (env vars and .env), not the flag defaults.
func TestRootPreRunRunsUnderSubcommandPreRun(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	origConsole, origPlatform, origVerbose := consoleUrl, platformUrl, verbose
	t.Cleanup(func() { consoleUrl, platformUrl, verbose = origConsole, origPlatform, origVerbose })

	viper.Set("console-url", "http://console.dev.kusari.cloud/")
	viper.Set("platform-url", "https://platform.api.dev.kusari.cloud/")
	viper.Set("verbose", true)

	var childPreRan bool
	var gotConsole, gotPlatform string
	var gotVerbose bool
	child := &cobra.Command{
		Use:              "pre-run-probe",
		PersistentPreRun: func(cmd *cobra.Command, args []string) { childPreRan = true },
		Run: func(cmd *cobra.Command, args []string) {
			gotConsole, gotPlatform, gotVerbose = consoleUrl, platformUrl, verbose
		},
	}
	rootCmd.AddCommand(child)
	t.Cleanup(func() { rootCmd.RemoveCommand(child) })

	rootCmd.SetArgs([]string{"pre-run-probe"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	require.NoError(t, rootCmd.Execute())

	assert.True(t, childPreRan, "subcommand's own PersistentPreRun should still run")
	assert.Equal(t, "http://console.dev.kusari.cloud/", gotConsole)
	assert.Equal(t, "https://platform.api.dev.kusari.cloud/", gotPlatform)
	assert.True(t, gotVerbose)
}
