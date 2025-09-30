package cmd

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/configuration"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	forceWrite bool
)

func init() {
	generatecmd.Flags().BoolVarP(&forceWrite, "force", "f", false, "Force creation when file exists")
	// Bind flags to viper
	mustBindPFlag("force", generatecmd.Flags().Lookup("force"))
}

func generateConfig() *cobra.Command {
	generatecmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		return configuration.GenerateConfig(forceWrite)
	}

	return generatecmd
}

var generatecmd = &cobra.Command{
	Use:   "generate-config",
	Short: fmt.Sprintf("Generate %s config file", configuration.ConfigFilename),
	Long: fmt.Sprintf("Generate a %s config file for kusari-cli "+
		"with default values.", configuration.ConfigFilename),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Update from viper (this gets env vars + config + flags)
		forceWrite = viper.GetBool("force")
	},
}
