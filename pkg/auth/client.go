// =============================================================================
// pkg/auth/client.go
// =============================================================================
package auth

import (
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Client struct {
	oauth2Config *oauth2.Config
}

func NewClient(authendpoint, clientID, clientSecret, redirectURL string) *Client {
	return &Client{
		oauth2Config: &oauth2.Config{
			ClientID:    clientID,
			RedirectURL: redirectURL,
			Scopes:      []string{oidc.ScopeOpenID, "profile", "email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  authendpoint + "oauth2/authorize",
				TokenURL: authendpoint + "oauth2/token",
			},
		},
	}
}
