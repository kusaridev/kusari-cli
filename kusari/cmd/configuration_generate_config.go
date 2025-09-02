package cmd

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/configuration"
	"github.com/spf13/cobra"
)

func generateConfig() *cobra.Command {
	generatecmd.RunE = func(cmd *cobra.Command, args []string) error {
		return configuration.GenerateConfig()
	}

	return generatecmd
}

var generatecmd = &cobra.Command{
	Use:   "generate-config",
	Short: fmt.Sprintf("Generate %s config file", configuration.ConfigFilename),
	Long: fmt.Sprintf("Generate a %s config file for kusari-cli "+
		"with default values.", configuration.ConfigFilename),
}
