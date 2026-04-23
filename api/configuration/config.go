package configuration

type Config struct {
	GitHubActionVersionPinningCheckEnabled bool   `yaml:"github_action_version_pinning_check_enabled"` // Check whether GH Action versions are pinned
	ContainerVersionPinningCheckEnabled    bool   `yaml:"container_version_pinning_check_enabled"`     // Check whether container versions are pinned
	PostCommentOnFailure                   bool   `yaml:"post_comment_on_failure"`  // Also post comment when status check fails
	PostCommentOnSuccess                   bool   `yaml:"post_comment_on_success"`  // Also post comment when status check succeeds

	// SBOM Generation Configuration (for merged PRs to main/master)
	SBOMGenerationEnabled      bool   `yaml:"sbom_generation_enabled"`                 // Enable SBOM generation on merged PRs (default: false)
	SBOMComponentName          string `yaml:"sbom_component_name,omitempty"`           // Custom component name for SBOM (default: GitHub repo name)
	SBOMSubjectNameOverride    string `yaml:"sbom_subject_name_override,omitempty"`    // Override SBOM subject name in Kusari Platform
	SBOMSubjectVersionOverride string `yaml:"sbom_subject_version_override,omitempty"` // Override SBOM subject version in Kusari Platform
}
