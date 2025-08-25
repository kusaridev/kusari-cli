package inspector

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig(t *testing.T) {
	type test struct {
		name      string
		inYaml    string
		expErr    error
		expErrMsg string
		exp       Config
	}

	tests := []test{
		{
			name: "no input",
			exp: Config{
				GitHubActionVersionPinningCheckEnabled: true,
				ContainerVersionPinningCheckEnabled:    true,
			},
		},

		{
			name:      "invalid yaml",
			inYaml:    "<ASDASD>ASD",
			expErr:    ErrParsingConfig,
			expErrMsg: ErrParsingConfig.Error(),
		},

		{
			name: "set all the config vars explicitly",
			inYaml: `
github_action_version_pinning_check_enabled: false
container_version_pinning_check_enabled: false
`,
			exp: Config{
				GitHubActionVersionPinningCheckEnabled: false,
				ContainerVersionPinningCheckEnabled:    false,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			act, err := GetConfig(test.inYaml)
			if test.expErr != nil {
				assert.ErrorIs(t, err, test.expErr)
				assert.Error(t, err, test.expErrMsg)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, &test.exp, act)
		})
	}
}

func TestGenerateConfig(t *testing.T) {
	t.Chdir(t.TempDir())

	err := GenerateConfig()
	assert.NoError(t, err)

	cfgYaml, err := os.ReadFile(ConfigFilename)
	assert.NoError(t, err)

	cfg, err := GetConfig(string(cfgYaml))
	assert.NoError(t, err)

	// Assert that every config value is a bool with value true

	cfgType := reflect.TypeOf(*cfg)
	cfgValue := reflect.ValueOf(*cfg)

	for i := range cfgType.NumField() {
		name := cfgType.Field(i).Name
		val := cfgValue.FieldByName(name)
		assert.True(t, val.Bool(), name)
	}
}
