package cmd

import "github.com/spf13/cobra"

func KusariConfiguration() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration actions",
		// Keep it simple for now and expand as more commands get added
		Long:    "Generate kusari-cli configuration file",
		Aliases: []string{"configuration"}, // alias to help existing users. Drop for 1.0
	}

	cmd.AddCommand(generateConfig())
	cmd.AddCommand(updateConfig())

	return cmd
}
