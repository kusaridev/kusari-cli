package configuration

type Config struct {
	GitHubActionVersionPinningCheckEnabled bool   `yaml:"github_action_version_pinning_check_enabled"`
	ContainerVersionPinningCheckEnabled    bool   `yaml:"container_version_pinning_check_enabled"`
	StatusCheckName                        string `yaml:"status_check_name"`       // Name of the status check (default: "Kusari Inspector")
	PostCommentOnFailure                   bool   `yaml:"post_comment_on_failure"` // Also post comment when status check fails
}
