// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
)

func Auth(consoleUrl string, verbose bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "auth things",
		Long:  "do auth things",
	}

	cmd.AddCommand(login(consoleUrl, verbose))

	return cmd
}
