package inspector

type Config struct {
	GitHubActionVersionPinningCheckEnabled bool `yaml:"github_action_version_pinning_check_enabled"`
	ContainerVersionPinningCheckEnabled    bool `yaml:"container_version_pinning_check_enabled"`
}
