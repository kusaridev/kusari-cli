package cmd

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/configuration"
	"github.com/spf13/cobra"
)

var (
	forceWrite bool
)

func init() {
	generatecmd.Flags().BoolVarP(&forceWrite, "force", "f", false, "Force creation when file exists")
}

func generateConfig() *cobra.Command {
	generatecmd.RunE = func(cmd *cobra.Command, args []string) error {
		return configuration.GenerateConfig(forceWrite)
	}

	return generatecmd
}

var generatecmd = &cobra.Command{
	Use:   "generate-config",
	Short: fmt.Sprintf("Generate %s config file", configuration.ConfigFilename),
	Long: fmt.Sprintf("Generate a %s config file for kusari-cli "+
		"with default values.", configuration.ConfigFilename),
}
