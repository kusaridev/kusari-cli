package cmd

import "github.com/spf13/cobra"

func Inspector() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspector",
		Short: "Inspector operations",
		Long:  "Run operations relating to Kusari Inspector",
	}

	cmd.AddCommand(generateConfig())

	return cmd
}
