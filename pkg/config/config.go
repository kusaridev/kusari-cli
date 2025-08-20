// =============================================================================
// pkg/config/config.go
// =============================================================================
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Config represents the application configuration
type Config struct {
	ConsoleUrl   string    `json:"console_url,omitempty"`
	PlatformUrl  string    `json:"platform_url,omitempty"`
	AuthEndpoint string    `json:"oidc_provider,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
	ClientSecret string    `json:"client_secret,omitempty"`
	RedirectURL  string    `json:"redirect_url,omitempty"`
	RedirectPort string    `json:"redirect_port,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`
	Verbose      bool      `json:"verbose,omitempty"`
}

// Manager handles configuration persistence
type Manager struct {
	configDir  string
	configFile string
	config     *Config
}

// NewManager creates a new configuration manager
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".kusari")
	configFile := filepath.Join(configDir, "config.json")

	manager := &Manager{
		configDir:  configDir,
		configFile: configFile,
		config:     &Config{},
	}

	// Load existing config
	if err := manager.Load(); err != nil {
		return nil, err
	}

	return manager, nil
}

// Load reads configuration from disk
func (m *Manager) Load() error {
	if _, err := os.Stat(m.configFile); os.IsNotExist(err) {
		return nil // Config file doesn't exist yet
	}

	data, err := os.ReadFile(m.configFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, m.config)
}

// Save writes configuration to disk
func (m *Manager) Save() error {
	// Ensure config directory exists
	if err := os.MkdirAll(m.configDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configFile, data, 0600)
}

// Get returns the current configuration
func (m *Manager) Get() *Config {
	return m.config
}

// Set updates the configuration
func (m *Manager) Set(config *Config) {
	m.config = config
}
