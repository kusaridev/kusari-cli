package cmd

import (
	"github.com/spf13/cobra"
)

func Auth() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "auth things",
		Long:  "do auth things",
	}

	cmd.AddCommand(login)

	return cmd
}
