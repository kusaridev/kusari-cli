// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package api

type BundleMeta struct {
	PatchName     string `json:"patch_name"`
	CurrentBranch string `json:"current_branch"`
	DirName       string `json:"dir_name"`
	DiffCmd       string `json:"diff_cmd"`
	Remote        string `json:"remote"`
	GitDirty      bool   `json:"git_dirty"`
	ScanType      string `json:"scan_type,omitempty"`
	// Incremental scanning fields
	CommitSHA    string   `json:"commit_sha,omitempty"`    // Current HEAD commit SHA
	ChangedFiles []string `json:"changed_files,omitempty"` // Files changed in this scan
}
