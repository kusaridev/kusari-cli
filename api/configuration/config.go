package configuration

type Config struct {
	GitHubActionVersionPinningCheckEnabled bool   `yaml:"github_action_version_pinning_check_enabled"` // Check whether GH Action versions are pinned
	ContainerVersionPinningCheckEnabled    bool   `yaml:"container_version_pinning_check_enabled"`     // Check whether container versions are pinned
	StatusCheckName                        string `yaml:"status_check_name"`                           // Name of the status check (default: "Kusari Inspector")
	PostCommentOnFailure                   bool   `yaml:"post_comment_on_failure"`                     // Also post comment when status check fails
	PostCommentOnSuccess                   bool   `yaml:"post_comment_on_success"`                     // Also post comment when status check succeeds
}
