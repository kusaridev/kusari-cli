// Copyright Kusari, Inc. and contributors <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/configuration"
	"github.com/spf13/cobra"
)

func updateConfig() *cobra.Command {
	updatecmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		return configuration.UpdateConfig()
	}

	return updatecmd
}

var updatecmd = &cobra.Command{
	Use:   "update",
	Short: fmt.Sprintf("Update %s config file", configuration.ConfigFilename),
	Long: fmt.Sprintf("Update a %s config file for kusari-cli "+
		"with new values.", configuration.ConfigFilename),
	Aliases: []string{"update-config"}, // alias to help existing users. Drop for 1.0
}
