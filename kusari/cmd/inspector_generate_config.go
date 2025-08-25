package cmd

import (
	"github.com/kusaridev/kusari-cli/pkg/inspector"
	"github.com/spf13/cobra"
)

func generateConfig() *cobra.Command {
	generatecmd.RunE = func(cmd *cobra.Command, args []string) error {
		return inspector.GenerateConfig()
	}

	return generatecmd
}

var generatecmd = &cobra.Command{
	Use:   "generate-config",
	Short: "Generate kusari.yaml config file",
	Long:  "Generate a kusari.yaml config file suitable for use with the Inspector and repo scan, with default values.",
}
