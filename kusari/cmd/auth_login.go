// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	l "github.com/kusaridev/kusari-cli/pkg/login"
	"github.com/kusaridev/kusari-cli/pkg/port"
	"github.com/spf13/cobra"
)

var (
	clientId     string
	authEndpoint string
)

func init() {
	logincmd.Flags().StringVarP(&authEndpoint, "auth-endpoint", "p", "https://auth.us.kusari.cloud/", "authentication endpoint URL")
	logincmd.Flags().StringVarP(&clientId, "client-id", "c", "4lnk6jccl3hc4lkcudai5lt36u", "OAuth2 client ID")
}

var logincmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Kusari Platform",
	Long:  `Login to Kusari Platform`,
}

func login() *cobra.Command {
	logincmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		redirectPort := port.GenerateRandomPortOrDefault()
		redirectUrl := fmt.Sprintf("http://localhost:%s/callback", redirectPort)

		return l.Login(cmd.Context(), clientId, redirectUrl, authEndpoint, redirectPort, consoleUrl, verbose)
	}

	return logincmd
}
