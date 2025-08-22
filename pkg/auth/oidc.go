// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/kusaridev/kusari-cli/pkg/url"
	"golang.org/x/oauth2"
)

func Authenticate(ctx context.Context, clientId string, redirectUrl string, authEndpoint string, redirectPort string, consoleUrl string) (*oauth2.Token, error) {

	consoleAnalysisUrl, err := url.Build(consoleUrl, "analysis")
	if err != nil {
		return nil, err
	}

	oauth2Config := oauthConfig(clientId, redirectUrl, authEndpoint)
	// Generate and use state to prevent CSRF attacks
	state, err := generateRandomString(32)
	if err != nil {
		return nil, NewAuthErrorWithCause(ErrAuthFlow, "failed to generate state", err)
	}

	// use PKCE to protect the auth code exchange
	codeVerifier := oauth2.GenerateVerifier()

	// Get code.
	var callbackRes = make(chan callbackResult)

	err = initializeListener(callbackRes, redirectPort, state, *consoleAnalysisUrl)
	if err != nil {
		return nil, err
	}

	challengeOption := oauth2.S256ChallengeOption(codeVerifier)
	authURL := oauth2Config.AuthCodeURL(state, challengeOption)

	fmt.Println("Attempting to automatically open the login page in your default browser.")
	fmt.Printf("If the browser does not open or you wish to use a different device to authorize this request, open the following URL:\n\n%s\n\n", authURL)
	fmt.Printf("Waiting for authentication...\n\n")

	if err := OpenBrowser(authURL); err != nil {
		fmt.Printf("Failed to open browser automatically. Please visit the login page manually.")
	}

	cs := <-callbackRes
	if cs.Error != nil {
		return nil, cs.Error
	}

	code := cs.Code

	authUrlOption := oauth2.SetAuthURLParam("code_verifier", codeVerifier)
	token, err := oauth2Config.Exchange(ctx, code, authUrlOption)
	if err != nil {
		return nil, fmt.Errorf("Failed to exchange token: %v", err)
	}

	provider := oauth2Config.Endpoint.TokenURL
	if err := SaveToken(token, provider); err != nil {
		return nil, err
	}

	return token, nil
}

func initializeListener(callback chan callbackResult, redirectPort string, state string, consoleAnalysisUrl string) error {
	l, err := net.Listen("tcp", fmt.Sprintf("localhost:%s", redirectPort))
	if err != nil {
		return fmt.Errorf("Failed to initialize listener: %v", err)
	}
	go func() {
		defer func() {
			_ = l.Close()
		}()
		err := http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handleCallbackv2(w, r, state, callback, consoleAnalysisUrl)
		}))
		if err != nil {
			log.Printf("Error listening for auth callback: %v", err)
		}
	}()
	return nil
}

func oauthConfig(clientID string, redirectURL string, authendpoint string) *oauth2.Config {
	// in here probably do the url concat logic.
	return &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: redirectURL,
		Scopes:      []string{oidc.ScopeOpenID, "profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  authendpoint + "oauth2/authorize",
			TokenURL: authendpoint + "oauth2/token",
		},
	}
}
