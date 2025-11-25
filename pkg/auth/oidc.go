// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func Authenticate(ctx context.Context, clientId, clientSecret, redirectUrl, authEndpoint, redirectPort, consoleUrl, workspaceId string) (*oauth2.Token, error) {
	baseURL, err := url.Parse(consoleUrl)
	if err != nil {
		return nil, err
	}

	var consoleAnalysisUrl string
	// Only redirect from callback if we have a workspace
	// For new users, we'll redirect from CLI after workspace selection
	if workspaceId != "" {
		analysisURL := baseURL.JoinPath("analysis")
		query := analysisURL.Query()
		query.Set("workspaceId", workspaceId)
		analysisURL.RawQuery = query.Encode()
		consoleAnalysisUrl = analysisURL.String()
	} else {
		// Empty string means don't redirect from callback handler
		// We'll redirect from CLI after workspace selection
		consoleAnalysisUrl = ""
	}

	oauth2Config := oauthConfig(clientId, redirectUrl, authEndpoint)

	var token *oauth2.Token
	if clientSecret != "" {
		var tokenErr error
		config := &clientcredentials.Config{
			ClientID:     clientId,
			ClientSecret: clientSecret,
			TokenURL:     oauth2Config.Endpoint.TokenURL,
		}
		token, tokenErr = config.Token(ctx)
		if tokenErr != nil {
			log.Fatalf("Failed to exchange token: %v", err)
			return nil, NewAuthErrorWithCause(ErrAuthFlow, "failed to exchange token", err)
		}
	} else {
		// Generate and use state to prevent CSRF attacks
		state, err := generateRandomString(32)
		if err != nil {
			return nil, NewAuthErrorWithCause(ErrAuthFlow, "failed to generate state", err)
		}

		// use PKCE to protect the auth code exchange
		codeVerifier := oauth2.GenerateVerifier()

		// Get code.
		l, err := net.Listen("tcp", fmt.Sprintf("localhost:%s", redirectPort))
		if err != nil {
			return nil, NewAuthErrorWithCause(ErrNetworkError, "failed to listen", err)
		}
		var callbackRes = make(chan callbackResult)
		go func() {
			defer func() {
				_ = l.Close()
			}()
			err := http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleCallbackv2(w, r, state, callbackRes, consoleAnalysisUrl)
			}))
			if err != nil {
				log.Printf("Error listening for auth callback: %v", err)
			}
		}()

		challengeOption := oauth2.S256ChallengeOption(codeVerifier)
		authURL := oauth2Config.AuthCodeURL(state, challengeOption)

		fmt.Println("Attempting to automatically open the login page in your default browser.")
		fmt.Printf("If the browser does not open or you wish to use a different device to authorize this request, open the following URL:\n\n%s\n\n", authURL)
		fmt.Printf("Waiting for authentication...\n\n")

		if err := OpenBrowser(authURL); err != nil {
			fmt.Printf("Failed to open browser automatically. Please visit the login page manually.")
		}

		cs := <-callbackRes
		//get code done
		if cs.Error != nil {
			return nil, cs.Error
		}

		code := cs.Code

		authUrlOption := oauth2.SetAuthURLParam("code_verifier", codeVerifier)
		var tokenErr error

		token, tokenErr = oauth2Config.Exchange(ctx, code, authUrlOption)
		if tokenErr != nil {
			log.Fatalf("Failed to exchange token: %v", tokenErr)
			return nil, tokenErr
		}
	}

	provider := oauth2Config.Endpoint.TokenURL
	if err := SaveToken(token, provider); err != nil {
		return nil, err
	}

	return token, nil
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
