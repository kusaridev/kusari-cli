package cmd

import "github.com/spf13/cobra"

func KusariConfiguration() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configuration",
		Short: "Configuration",
		Long:  "Configuration actions",
	}

	cmd.AddCommand(generateConfig())

	return cmd
}
