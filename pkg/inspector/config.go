package inspector

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const ConfigFilename = "kusari.yaml"

type Config struct {
	GitHubActionVersionPinningCheckEnabled bool
	ContainerVersionPinningCheckEnabled    bool
}

type configYAML struct {
	GitHubActionVersionPinningCheckEnabled *bool `yaml:"github_action_version_pinning_check_enabled,omitempty"`
	ContainerVersionPinningCheckEnabled    *bool `yaml:"container_version_pinning_check_enabled,omitempty"`
}

var (
	ErrParsingConfig = errors.New("error parsing config")
)

var DefaultConfig = Config{
	GitHubActionVersionPinningCheckEnabled: true,
	ContainerVersionPinningCheckEnabled:    true,
}

func GetConfig(yamlStr string) (*Config, error) {
	var cfgYaml configYAML
	if yamlStr != "" {
		dec := yaml.NewDecoder(strings.NewReader(yamlStr))
		dec.KnownFields(true)
		if err := dec.Decode(&cfgYaml); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrParsingConfig, err)
		}
	}

	cfg := DefaultConfig

	if cfgYaml.GitHubActionVersionPinningCheckEnabled != nil {
		cfg.GitHubActionVersionPinningCheckEnabled = *cfgYaml.GitHubActionVersionPinningCheckEnabled
	}

	if cfgYaml.ContainerVersionPinningCheckEnabled != nil {
		cfg.ContainerVersionPinningCheckEnabled = *cfgYaml.ContainerVersionPinningCheckEnabled
	}

	return &cfg, nil
}

func GenerateConfig() error {
	const cfgYaml = `github_action_version_pinning_check_enabled: true
container_version_pinning_check_enabled: true
`

	return os.WriteFile("kusari.yaml", []byte(cfgYaml), 0600)
}
