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

var ErrFileExists = fmt.Errorf("file %s exists, not overwriting (specify '--force' to overwrite)", ConfigFilename)

var DefaultConfig = configuration.Config{
	GitHubActionVersionPinningCheckEnabled: true,
	ContainerVersionPinningCheckEnabled:    true,
	StatusCheckName:                        "Kusari Inspector",
	PostCommentOnFailure:                   true,
	PostCommentOnSuccess:                   false,
	FullCodeReviewEnabled:                  false,
}

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
	} else if err != nil {
		// Handle other failure cases
		return fmt.Errorf("failed to check for config file %s: %w", ConfigFilename, err)
	}

	// Read the config file to get existing values
	configData, err := os.ReadFile(ConfigFilename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", ConfigFilename, err)
	}

	var existingConfig map[string]interface{}
	if err := yaml.Unmarshal(configData, &existingConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	updatedConfig, err := mergeConfigs(DefaultConfig, existingConfig)
	if err != nil {
		return fmt.Errorf("error merging configs: %w", err)
	}

	cfgYaml, err := yaml.Marshal(updatedConfig)
	if err != nil {
		return fmt.Errorf("error marshaling default config yaml: %w", err)
	}

	return os.WriteFile(ConfigFilename, []byte(cfgYaml), 0600)
}

// A function to compare the configs and merge them together
func mergeConfigs(defaultConfig configuration.Config, existingConfig map[string]interface{}) (configuration.Config, error) {
	result := defaultConfig

	// Use reflection to iterate over all struct fields
	resultValue := reflect.ValueOf(&result).Elem()
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultType.NumField(); i++ {
		field := resultType.Field(i)

		// Get the JSON tag name for this field
		yamlTag := field.Tag.Get("yaml")

		// Check if this field was present in the original YAML
		if val, exists := existingConfig[yamlTag]; exists {
			resultFieldValue := resultValue.Field(i)

			// Only set if the field is settable
			if resultFieldValue.CanSet() {
				// Convert and set the value based on field type
				switch resultFieldValue.Kind() {
				case reflect.Bool:
					if boolVal, ok := val.(bool); ok {
						resultFieldValue.SetBool(boolVal)
					} else {
						return defaultConfig, fmt.Errorf("could not parse %s as a boolean", yamlTag)
					}
				case reflect.String:
					if stringVal, ok := val.(string); ok {
						resultFieldValue.SetString(stringVal)
					} else {
						return defaultConfig, fmt.Errorf("could not parse %s as a string", yamlTag)
					}
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					// YAML can parse numbers as int, int64, or sometimes float64
					switch v := val.(type) {
					case int:
						resultFieldValue.SetInt(int64(v))
					case int64:
						resultFieldValue.SetInt(v)
					case float64:
						resultFieldValue.SetInt(int64(v))
					default: // We should never get here
						return defaultConfig, fmt.Errorf("could not parse %s as an integer", yamlTag)
					}
				case reflect.Float32, reflect.Float64:
					if floatVal, ok := val.(float64); ok {
						resultFieldValue.SetFloat(floatVal)
					}
				default: // We should never get here
					return defaultConfig, fmt.Errorf("could not parse %s as a %s", yamlTag, resultFieldValue.Kind())
				}
			}
		}
	}

	return result, nil
}
