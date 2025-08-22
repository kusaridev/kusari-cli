// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package api

// UserInspectorResult represents the structure for our DynamoDB table
type UserInspectorResult struct {
	User     string   `docstore:"user" json:"user"` // Primary key
	Sort     string   `docstore:"sort" json:"sort"` // Sort key (epoch timestamp)
	TTL      int64    `docstore:"ttl" json:"ttl"`   // TTL (epoch expiration timestamp)
	Analysis Analysis `docstore:"analysis" json:"analysis"`
	Meta     Meta     `docstore:"meta" json:"meta"`
}

// WorkspaceApp represents the workspace-gh-app table structure
type WorkspaceApp struct {
	Workspace string `docstore:"workspace" json:"workspace"` // Primary key
	UserOrg   string `docstore:"user_org" json:"user_org"`   // Sort key
}

// WorkspaceInspectorResult represents the workspace-inspector-results table structure
type WorkspaceInspectorResult struct {
	Workspace string   `docstore:"workspace" json:"workspace"` // Primary key (fixed json tag)
	Sort      string   `docstore:"sort" json:"sort"`           // Sort key (epoch timestamp)
	TTL       int64    `docstore:"ttl" json:"ttl"`             // TTL (epoch expiration timestamp)
	Analysis  Analysis `docstore:"analysis" json:"analysis"`
	Meta      Meta     `docstore:"meta" json:"meta"`
}

type Analysis struct {
	Proceed bool   `docstore:"proceed" json:"proceed"`
	Results string `docstore:"results" json:"results"` // markdown content
	// Add other analysis fields as needed
}

type Meta struct {
	Type     string `docstore:"type" json:"type"` // pr, cli
	PR       int    `docstore:"pr" json:"pr"`
	Repo     string `docstore:"repo" json:"repo"`
	Org      string `docstore:"org" json:"org"`
	PRURL    string `docstore:"pr_url" json:"pr_url"`
	Commit   string `docstore:"commit" json:"commit"`
	Branch   string `docstore:"branch" json:"branch"`
	DirName  string `docstore:"dir_name" json:"dir_name"`
	DiffCmd  string `docstore:"diff_cmd" json:"diff_cmd"`
	Remote   string `docstore:"remote" json:"remote"`
	GitDirty bool   `docstore:"git_dirty" json:"git_dirty"`
}
