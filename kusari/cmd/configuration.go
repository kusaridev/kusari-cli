package cmd

import "github.com/spf13/cobra"

func KusariConfiguration() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configuration",
		Short: "Configuration actions",
		// Keep it simple for now and expand as more commands get added
		Long: "Generate kusari-cli configuration file",
	}

	cmd.AddCommand(generateConfig())

	return cmd
}
