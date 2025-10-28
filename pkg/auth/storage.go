// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
)

const (
	configDirName = ".kusari"
	tokenFileName = "tokens.json"
)

// getConfigDir returns the configuration directory path
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", NewAuthErrorWithCause(ErrTokenStorage, "failed to get user home directory", err)
	}
	return filepath.Join(homeDir, configDirName), nil
}

// getTokenFilePath returns the full path to the token file
func getTokenFilePath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, tokenFileName), nil
}

// SaveToken saves the token information to disk
func SaveToken(token *oauth2.Token, provider string) error {
	// func SaveToken(token *TokenInfo) error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return NewAuthErrorWithCause(ErrTokenStorage, "failed to create config directory", err)
	}

	tokenPath, err := getTokenFilePath()
	if err != nil {
		return err
	}

	// Load existing tokens
	var tokens map[string]*oauth2.Token
	if data, err := os.ReadFile(tokenPath); err == nil {
		if err := json.Unmarshal(data, &tokens); err != nil {
			return fmt.Errorf("error, found token file, but did not unmarshal: %w", err)
		}
	}
	if tokens == nil {
		// tokens = make(map[string]*TokenInfo)
		tokens = make(map[string]*oauth2.Token)
	}

	// Store the new token
	tokens["kusari"] = token
	// tokens[provider] = token

	// Write back to file
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return NewAuthErrorWithCause(ErrTokenStorage, "failed to marshal tokens", err)
	}

	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		return NewAuthErrorWithCause(ErrTokenStorage, "failed to write token file", err)
	}

	return nil
}

// LoadToken loads token information from disk
func LoadToken(provider string) (*oauth2.Token, error) {
	// func LoadToken(provider string) (*TokenInfo, error) {
	tokenPath, err := getTokenFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewAuthError(ErrInvalidToken, "no stored tokens found. Run `kusari auth login`.")
		}
		return nil, NewAuthErrorWithCause(ErrTokenStorage, "failed to read token file", err)
	}

	var tokens map[string]*oauth2.Token
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, NewAuthErrorWithCause(ErrTokenStorage, "failed to unmarshal tokens", err)
	}

	token, exists := tokens[provider]
	if !exists {
		return nil, NewAuthError(ErrInvalidToken, fmt.Sprintf("no token found for provider: %s", provider))
	}

	return token, nil
}

// ClearTokens removes all stored tokens
func ClearTokens() error {
	tokenPath, err := getTokenFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return NewAuthErrorWithCause(ErrTokenStorage, "failed to remove token file", err)
	}

	return nil
}

func CheckTokenExpiry(token *oauth2.Token) error {
	if token.Expiry.Before(time.Now()) {
		return NewAuthError(ErrTokenExpired, "Token is expired. Re-run `kusari auth login`")
	}
	return nil
}
