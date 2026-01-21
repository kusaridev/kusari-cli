// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
)

func Auth() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication operations",
		Long:  "Authenticate to Kusari and select Kusari workspace/tenant",
	}

	cmd.AddCommand(login())
	cmd.AddCommand(selectWorkspace())
	cmd.AddCommand(selectTenant())

	return cmd
}
