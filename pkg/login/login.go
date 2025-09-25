// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package login

import (
	"context"
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/auth"
)

func Login(ctx context.Context, clientId, clientSecret, redirectUrl, authEndpoint, redirectPort, consoleUrl string, verbose bool) error {
	if verbose {
		fmt.Printf(" AuthEndpoint: %s\n", authEndpoint)
		fmt.Printf(" ConsoleUrl: %s\n", consoleUrl)
		fmt.Printf(" ClientId: %s\n", clientId)
		fmt.Printf(" CallbackUrl: %s\n", redirectUrl)
		fmt.Println()
	}

	_, err := auth.Authenticate(ctx, clientId, clientSecret, redirectUrl, authEndpoint, redirectPort, consoleUrl)
	if err != nil {
		return err
	}

	fmt.Println("Successfully logged in!")

	// ANSI escape codes:
	// \033[1m = bold
	// \033[34m = blue
	// \033[0m = reset
	fmt.Println("\033[1m\033[34mFor more information, visit:\033[0m https://docs.kusari.cloud")
	return nil
}
