package cmd

import (
	"fmt"

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
	Short: fmt.Sprintf("Generate %s config file", inspector.ConfigFilename),
	Long: fmt.Sprintf("Generate a %s config file suitable for use with the Inspector and repo scan, "+
		"with default values.", inspector.ConfigFilename),
}
