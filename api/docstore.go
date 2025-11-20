// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package api

// UserInspectorResult represents the structure for our DynamoDB table
type UserInspectorResult struct {
	User       string     `docstore:"user" json:"user"` // Primary key
	Sort       string     `docstore:"sort" json:"sort"` // Sort key (epoch timestamp)
	TTL        int64      `docstore:"ttl" json:"ttl"`   // TTL (epoch expiration timestamp)
	Analysis   *Analysis  `docstore:"analysis" json:"analysis,omitempty"`
	Meta       Meta       `docstore:"meta" json:"meta"`
	StatusMeta StatusMeta `docstore:"status_meta" json:"statusMeta"` // Status and metadata
}

type StatusMeta struct {
	SortEpoch string `json:"sort"`              // the sort key will be "epoch|status|value", this is the epoch
	Status    string `json:"status"`            // processing, uploaded, etc
	Details   string `json:"details,omitempty"` // detailed message
	UpdatedAt string `json:"updatedAt"`         // Insertion time
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
	Proceed        bool              `docstore:"proceed" json:"proceed"`
	Results        string            `docstore:"results" json:"results"` // markdown content
	RawLLMAnalysis *SecurityAnalysis `docstore:"rawLLMAnalysis" json:"rawLLMAnalysis"`
	Score          int               `dynamodbav:"score" docstore:"score" json:"score"`
	Health         Health            `dynamodbav:"health" docstore:"health" json:"health"`
	// Add other analysis fields as needed
}

type SubScan struct {
	Score   int     `dynamodbav:"score" docstore:"score" json:"score"`
	Summary Summary `dynamodbav:"summary" docstore:"summary" json:"summary"`
	Checks  []Check `dynamodbav:"checks" docstore:"checks" json:"checks"`
}

type Summary struct {
	Data []LabelWithValues `dynamodbav:"data" docstore:"data" json:"data"`
}

type Check struct {
	Name string          `dynamodbav:"name" docstore:"name" json:"name"`
	Pass bool            `dynamodbav:"pass" docstore:"pass" json:"pass"`
	Data LabelWithValues `dynamodbav:"data" docstore:"data" json:"data"`
}

type LabelWithValues struct {
	Label  string   `dynamodbav:"label" docstore:"label" json:"label"`
	Values []string `dynamodbav:"values" docstore:"values" json:"values"`
}

type Health map[string]SubScan

// Structured output types
type CodeMitigationItem struct {
	LineNumber int    `docstore:"line_number" json:"line_number"`
	Path       string `docstore:"path" json:"path"`
	Content    string `docstore:"content" json:"content"`
	Code       string `docstore:"code" json:"code,omitempty"`
}

type DependencyMitigationItem struct {
	Content string `docstore:"content" json:"content"`
}

type SecurityAnalysis struct {
	Recommendation                string                     `docstore:"recommendation" json:"recommendation"`
	Justification                 string                     `docstore:"justification" json:"justification"`
	RequiredCodeMitigations       []CodeMitigationItem       `docstore:"code_mitigations" json:"code_mitigations,omitempty"`
	RequiredDependencyMitigations []DependencyMitigationItem `docstore:"dependency_mitigations" json:"dependency_mitigations,omitempty"`
	ShouldProceed                 bool                       `docstore:"should_proceed" json:"should_proceed"`
	FailedAnalysis                bool                       `docstore:"failed_analysis" json:"failed_analysis"`
	HealthScore                   int                        `docstore:"health_score" json:"health_score"` // 0-5 scale for full repo scans
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
