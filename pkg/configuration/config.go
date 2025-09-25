package configuration

import (
	"fmt"
	"os"

	"github.com/kusaridev/kusari-cli/api/configuration"
	"gopkg.in/yaml.v3"
)

const ConfigFilename = "kusari.yaml"

var ErrFileExists = fmt.Errorf("file %s exists, not overwriting (specify '--force' to overwrite)", ConfigFilename)

func GenerateConfig(forceWrite bool) error {
	DefaultConfig := configuration.Config{
		GitHubActionVersionPinningCheckEnabled: true,
		ContainerVersionPinningCheckEnabled:    true,
		StatusCheckName:                        "Kusari Inspector",
		PostCommentOnFailure:                   true,
		PostCommentOnSuccess:                   true,
	}

	// check to see if the config file already exists
	_, err := os.Stat(ConfigFilename)
	if (err == nil) && !forceWrite {
		return ErrFileExists
	}

	cfgYaml, err := yaml.Marshal(DefaultConfig)
	if err != nil {
		return fmt.Errorf("error marshing default config yaml: %w", err)
	}

	return os.WriteFile(ConfigFilename, []byte(cfgYaml), 0600)
}
