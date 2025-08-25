package inspector

import (
	"fmt"
	"os"

	"github.com/kusaridev/kusari-cli/api/inspector"
	"gopkg.in/yaml.v3"
)

const ConfigFilename = "kusari.yaml"

func GenerateConfig() error {
	DefaultConfig := inspector.Config{
		GitHubActionVersionPinningCheckEnabled: true,
		ContainerVersionPinningCheckEnabled:    true,
	}

	cfgYaml, err := yaml.Marshal(DefaultConfig)
	if err != nil {
		return fmt.Errorf("error marshing default config yaml: %w", err)
	}

	return os.WriteFile(ConfigFilename, []byte(cfgYaml), 0600)
}
