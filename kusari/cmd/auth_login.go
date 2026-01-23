// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	l "github.com/kusaridev/kusari-cli/pkg/login"
	"github.com/kusaridev/kusari-cli/pkg/port"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	clientId     string
	authEndpoint string
	clientSecret string
	useSso       bool
)

func init() {
	logincmd.Flags().StringVarP(&authEndpoint, "auth-endpoint", "p", "https://auth.us.kusari.cloud/", "authentication endpoint URL")
	logincmd.Flags().StringVarP(&clientId, "client-id", "c", "4lnk6jccl3hc4lkcudai5lt36u", "OAuth2 client ID")
	logincmd.Flags().StringVarP(&clientSecret, "client-secret", "s", "", "OAuth client secret ")
	logincmd.Flags().BoolVar(&useSso, "use-sso", false, "Use SSO (SAML) authentication")

	// Bind flags to viper
	mustBindPFlag("auth-endpoint", logincmd.Flags().Lookup("auth-endpoint"))
	mustBindPFlag("client-id", logincmd.Flags().Lookup("client-id"))
	mustBindPFlag("client-secret", logincmd.Flags().Lookup("client-secret"))
	mustBindPFlag("use-sso", logincmd.Flags().Lookup("use-sso"))
}

var logincmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Kusari Platform",
	Long:  `Login to Kusari Platform`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Update from viper (this gets env vars + config + flags)
		authEndpoint = viper.GetString("auth-endpoint")
		clientId = viper.GetString("client-id")
		clientSecret = viper.GetString("client-secret")
		useSso = viper.GetBool("use-sso")
	},
}

func login() *cobra.Command {
	logincmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// Override client ID for SSO authentication
		effectiveClientId := clientId
		if useSso {
			effectiveClientId = "7ippro0e5e8qd3oragd4k1h39i"
		}

		redirectPort := port.GenerateRandomPortOrDefault()
		redirectUrl := fmt.Sprintf("http://localhost:%s/callback", redirectPort)

		return l.Login(cmd.Context(), effectiveClientId, clientSecret, redirectUrl, authEndpoint, redirectPort, consoleUrl, platformUrl, verbose, useSso)
	}

	return logincmd
}
