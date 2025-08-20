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
	consoleUrl   string
	verbose      bool
)

func init() {
	login.Flags().StringVarP(&consoleUrl, "console-url", "", "http://console.us.kusari.cloud/", "console url")
	login.Flags().StringVarP(&authEndpoint, "auth-endpoint", "p", "https://auth.us.kusari.cloud/", "authentication endpoint URL")
	login.Flags().StringVarP(&clientId, "client-id", "c", "4lnk6jccl3hc4lkcudai5lt36u", "OAuth2 client ID")
	login.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
}

var login = &cobra.Command{
	Use:   "login",
	Short: "Login to Kusari Platform",
	Long:  `Login to Kusari Platform`,
	RunE:  run,
}

func run(cmd *cobra.Command, args []string) error {
	redirectPort := port.GenerateRandomPortOrDefault()
	redirectUrl := fmt.Sprintf("http://localhost:%s/callback", redirectPort)

	return l.Login(cmd.Context(), clientId, redirectUrl, authEndpoint, redirectPort, consoleUrl, verbose)
}
