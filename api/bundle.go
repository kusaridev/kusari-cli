package api

type BundleMeta struct {
	PatchName     string `json:"patch_name"`
	CurrentBranch string `json:"current_branch"`
	DirName       string `json:"dir_name"`
	DiffCmd       string `json:"diff_cmd"`
	Remote        string `json:"remote"`
	GitDirty      bool   `json:"git_dirty"`
}
