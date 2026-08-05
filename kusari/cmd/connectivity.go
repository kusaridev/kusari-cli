// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
)

func Connectivity() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connectivity",
		Aliases: []string{"conn"},
		Short:   "Diagnose network connectivity to Kusari endpoints",
		Long:    "Diagnose DNS, TCP, TLS and HTTP proxy connectivity between this machine and the Kusari services the CLI uses",
	}

	cmd.AddCommand(connectivityCheck())

	return cmd
}
