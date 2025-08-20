package auth

import "time"
type AuthResult struct {
	Token *Token
	Error error
}
type callbackResult struct {
	Code  string
	Error error
}
type Token struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	IDToken      string    `json:"id_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	ExpiresIn    int64     `json:"expires_in,omitempty"`
	Scopes       []string  `json:"scopes"`
	Provider     string    `json:"provider"`
}
