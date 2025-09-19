package configuration

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/kusaridev/kusari-cli/api/configuration"
	"gopkg.in/yaml.v3"
)

const ConfigFilename = "kusari.yaml"

var DefaultConfig = configuration.Config{
	GitHubActionVersionPinningCheckEnabled: true,
	ContainerVersionPinningCheckEnabled:    true,
	StatusCheckName:                        "Kusari Inspector",
	PostCommentOnFailure:                   true,
}
var ErrFileExists = fmt.Errorf("file %s exists, not overwriting (specify '--force' to overwrite)", ConfigFilename)

func GenerateConfig(forceWrite bool) error {
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

func UpdateConfig() error {
	// Check to see if the config file already exists. If it does not,
	// just run the Generate instead.
	_, err := os.Stat(ConfigFilename)
	if errors.Is(err, os.ErrNotExist) {
		return GenerateConfig(false)
	}

	var existingConfig configuration.Config
	var newConfig = DefaultConfig

	// Read the config file to get new values
	existingConfigFile, err := os.ReadFile(ConfigFilename)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	if err := yaml.Unmarshal(existingConfigFile, &existingConfig); err != nil {
		return fmt.Errorf("error unmarshaling config file: %w", err)
	}

	existingValue := reflect.ValueOf(existingConfig)

	defaultSetting := reflect.TypeOf(DefaultConfig)
	defaultValue := reflect.ValueOf(DefaultConfig)

	newConfigValue := reflect.ValueOf(&newConfig).Elem()

	fmt.Fprintf(os.Stderr, "%s", reflect.TypeOf(newConfig.StatusCheckName))
	for i := 0; i < defaultSetting.NumField(); i++ {
		settingName := defaultSetting.Field(i).Name
		// Update changed configs
		existingFieldValue := reflect.Indirect(existingValue).FieldByName(settingName)
		defaultFieldValue := reflect.Indirect(defaultValue).FieldByName(settingName)
		if !reflect.DeepEqual(existingFieldValue.Interface(), defaultFieldValue.Interface()) {
			newConfigValue.FieldByName(settingName).Set(existingFieldValue)
		}
	}

	cfgYaml, err := yaml.Marshal(newConfig)
	if err != nil {
		return fmt.Errorf("error marshing default config yaml: %w", err)
	}

	return os.WriteFile(ConfigFilename, []byte(cfgYaml), 0600)
}
