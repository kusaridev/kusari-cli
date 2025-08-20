// =============================================================================
// pkg/cli/root.go
// =============================================================================
package main

import (
	"github.com/kusaridev/kusari-cli/kusari/cmd"
	"github.com/spf13/cobra"
)

// Execute runs the root command
func Execute() error {

	rootCmd := &cobra.Command{
		Use:   "kusari",
		Short: "Kusari - All signal, no noise. No chasing. No surprises. Just secure code, faster.",
		Long:  "Kusari - All signal, no noise. No chasing. No surprises. Just secure code, faster.",
	}

	rootCmd.AddCommand(cmd.Auth())
	rootCmd.AddCommand(cmd.Repo())

	return rootCmd.Execute()
}
