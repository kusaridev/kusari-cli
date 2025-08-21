// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
)

var (
	consoleUrl string
	verbose    bool
)

func Auth(c string, v bool) *cobra.Command {
	consoleUrl = c
	verbose = v
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "auth things",
		Long:  "do auth things",
	}

	cmd.AddCommand(login)

	return cmd
}
