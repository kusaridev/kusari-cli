package configuration

import (
	"fmt"
	"os"

	"github.com/kusaridev/kusari-cli/api/configuration"
	"gopkg.in/yaml.v3"
)

const ConfigFilename = "kusari.yaml"

func GenerateConfig() error {
	DefaultConfig := configuration.Config{
		GitHubActionVersionPinningCheckEnabled: true,
		ContainerVersionPinningCheckEnabled:    true,
	}

	cfgYaml, err := yaml.Marshal(DefaultConfig)
	if err != nil {
		return fmt.Errorf("error marshing default config yaml: %w", err)
	}

	return os.WriteFile(ConfigFilename, []byte(cfgYaml), 0600)
}
