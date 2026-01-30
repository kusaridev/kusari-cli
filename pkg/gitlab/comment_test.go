// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package gitlab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kusaridev/kusari-cli/api"
	"github.com/kusaridev/kusari-cli/pkg/comment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckForIssues(t *testing.T) {
	tests := []struct {
		name            string
		analysis        *api.SecurityAnalysis
		expectHasIssues bool
		expectCount     int
	}{
		{
			name: "no issues - should proceed true, no mitigations",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:                 true,
				FailedAnalysis:                false,
				RequiredCodeMitigations:       nil,
				RequiredDependencyMitigations: nil,
			},
			expectHasIssues: false,
			expectCount:     0,
		},
		{
			name: "failed analysis",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  true,
				FailedAnalysis: true,
			},
			expectHasIssues: true,
			expectCount:     1,
		},
		{
			name: "should not proceed with mitigations",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  false,
				FailedAnalysis: false,
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Path: "test.go", LineNumber: 10, Content: "Issue 1"},
					{Path: "test.go", LineNumber: 20, Content: "Issue 2"},
				},
				RequiredDependencyMitigations: []api.DependencyMitigationItem{
					{Content: "Dep issue"},
				},
			},
			expectHasIssues: true,
			expectCount:     3,
		},
		{
			name: "should not proceed without mitigations",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:                 false,
				FailedAnalysis:                false,
				RequiredCodeMitigations:       nil,
				RequiredDependencyMitigations: nil,
			},
			expectHasIssues: true,
			expectCount:     1,
		},
		{
			name: "should proceed but has mitigations",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  true,
				FailedAnalysis: false,
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Path: "test.go", LineNumber: 10, Content: "Warning"},
				},
			},
			expectHasIssues: false,
			expectCount:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasIssues, count := comment.CheckForIssues(tt.analysis)
			assert.Equal(t, tt.expectHasIssues, hasIssues)
			assert.Equal(t, tt.expectCount, count)
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "leading dot slash",
			input:    "./src/main.go",
			expected: "src/main.go",
		},
		{
			name:     "leading slash",
			input:    "/src/main.go",
			expected: "src/main.go",
		},
		{
			name:     "no prefix",
			input:    "src/main.go",
			expected: "src/main.go",
		},
		{
			name:     "double dot slash",
			input:    "././test.go",
			expected: "./test.go",
		},
		{
			name:     "simple filename",
			input:    "main.go",
			expected: "main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comment.SanitizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatComment(t *testing.T) {
	tests := []struct {
		name             string
		analysis         *api.SecurityAnalysis
		consoleURL       string
		expectedContains []string
		expectedMissing  []string
	}{
		{
			name: "full analysis with issues - not proceeding",
			analysis: &api.SecurityAnalysis{
				ShouldProceed: false,
				Justification: "These issues pose a risk",
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Path: "src/main.go", LineNumber: 42, Content: "SQL injection vulnerability"},
					{Path: "src/auth.go", LineNumber: 0, Content: "Weak password validation"},
				},
				RequiredDependencyMitigations: []api.DependencyMitigationItem{
					{Content: "Upgrade lodash to 4.17.21"},
				},
			},
			consoleURL: "https://console.kusari.dev/results/123",
			expectedContains: []string{
				"Kusari Analysis Results",
				"Flagged Issues Detected",
				"These issues pose a risk",
				"## Required Code Mitigations",
				"SQL injection vulnerability",
				"src/main.go:42",
				"Weak password validation",
				"## Required Dependency Mitigations",
				"Upgrade lodash to 4.17.21",
				"View full detailed analysis result",
				"https://console.kusari.dev/results/123",
				"IGNORE_KUSARI_COMMENT",
			},
			expectedMissing: nil,
		},
		{
			name: "analysis with should proceed - no issues",
			analysis: &api.SecurityAnalysis{
				ShouldProceed: true,
				Justification: "All values appear safe",
			},
			consoleURL: "",
			expectedContains: []string{
				"Kusari Analysis Results",
				"No Flagged Issues Detected",
				"All values appear safe",
			},
			expectedMissing: []string{
				"## Required Code Mitigations",
				"## Required Dependency Mitigations",
				"View full detailed analysis result",
			},
		},
		{
			name: "analysis with console URL",
			analysis: &api.SecurityAnalysis{
				ShouldProceed: true,
				Justification: "Test",
			},
			consoleURL: "https://console.kusari.dev/test",
			expectedContains: []string{
				"View full detailed analysis result",
				"https://console.kusari.dev/test",
			},
			expectedMissing: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comment.FormatComment(tt.analysis, tt.consoleURL)

			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}

			for _, missing := range tt.expectedMissing {
				assert.NotContains(t, result, missing)
			}
		})
	}
}

func TestFormatInlineComment(t *testing.T) {
	tests := []struct {
		name             string
		issue            api.CodeMitigationItem
		expectedContains []string
	}{
		{
			name: "with code suggestion",
			issue: api.CodeMitigationItem{
				Content: "SQL injection vulnerability detected",
				Code:    "db.Query(ctx, query, args...)",
			},
			expectedContains: []string{
				"🔒 **Kusari Security Issue**",
				"SQL injection vulnerability detected",
				"**Recommended Code Changes:**",
				"```",
				"db.Query(ctx, query, args...)",
			},
		},
		{
			name: "without code suggestion",
			issue: api.CodeMitigationItem{
				Content: "Hardcoded credentials detected",
			},
			expectedContains: []string{
				"🔒 **Kusari Security Issue**",
				"Hardcoded credentials detected",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comment.FormatInlineComment(tt.issue)
			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestPostComment(t *testing.T) {
	tests := []struct {
		name          string
		analysis      *api.SecurityAnalysis
		expectPosted  bool
		expectMessage string
		expectIssues  int
	}{
		{
			name:          "nil analysis",
			analysis:      nil,
			expectPosted:  false,
			expectMessage: "No analysis results available",
			expectIssues:  0,
		},
		{
			name: "no issues",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  true,
				FailedAnalysis: false,
			},
			expectPosted:  false,
			expectMessage: "No issues found",
			expectIssues:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := CommentOptions{
				ProjectID:   "123",
				MergeReqIID: "1",
				Token:       "test-token",
			}

			result, err := PostComment(tt.analysis, opts)
			require.NoError(t, err)
			assert.Equal(t, tt.expectPosted, result.Posted)
			assert.Equal(t, tt.expectIssues, result.IssuesFound)
			assert.Contains(t, result.Message, tt.expectMessage)
		})
	}
}

func TestPostCommentWithServer(t *testing.T) {
	tests := []struct {
		name                  string
		analysis              *api.SecurityAnalysis
		existingNotes         string
		existingDiscussions   string
		expectNotesCreated    int
		expectNotesUpdated    int
		expectDiscCreated     int
		expectDiscUpdated     int
		expectInlineComments  int
		expectMessageContains string
	}{
		{
			name: "create new comments",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  false,
				Recommendation: "Fix issues",
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Path: "src/main.go", LineNumber: 10, Content: "Issue 1"},
					{Path: "src/db.go", LineNumber: 20, Content: "Issue 2"},
				},
			},
			existingNotes:         "[]",
			existingDiscussions:   "[]",
			expectNotesCreated:    1,
			expectNotesUpdated:    0,
			expectDiscCreated:     2,
			expectDiscUpdated:     0,
			expectInlineComments:  2,
			expectMessageContains: "Posted",
		},
		{
			name: "update existing summary",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  false,
				Recommendation: "Fix issues",
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Path: "src/new.go", LineNumber: 5, Content: "New issue"},
				},
			},
			existingNotes:         `[{"id": 999, "body": "#### Kusari Analysis Results:\n\nOld\n<!-- IGNORE_KUSARI_COMMENT -->"}]`,
			existingDiscussions:   "[]",
			expectNotesCreated:    0,
			expectNotesUpdated:    1,
			expectDiscCreated:     1,
			expectDiscUpdated:     0,
			expectInlineComments:  1,
			expectMessageContains: "Updated",
		},
		{
			name: "skip issues without line numbers",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  false,
				Recommendation: "Fix issues",
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Path: "src/main.go", LineNumber: 0, Content: "No line number"},
					{Path: "src/db.go", LineNumber: 20, Content: "Has line number"},
				},
			},
			existingNotes:         "[]",
			existingDiscussions:   "[]",
			expectNotesCreated:    1,
			expectNotesUpdated:    0,
			expectDiscCreated:     1,
			expectDiscUpdated:     0,
			expectInlineComments:  1,
			expectMessageContains: "Posted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notesCreated := 0
			notesUpdated := 0
			discCreated := 0
			discUpdated := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				switch {
				case r.URL.Path == "/api/v4/projects/123/merge_requests/1/notes" && r.Method == "GET":
					_, _ = w.Write([]byte(tt.existingNotes))

				case r.URL.Path == "/api/v4/projects/123/merge_requests/1/notes" && r.Method == "POST":
					notesCreated++
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"id": 1}`))

				case r.URL.Path == "/api/v4/projects/123/merge_requests/1/notes/999" && r.Method == "PUT":
					notesUpdated++
					_, _ = w.Write([]byte(`{"id": 999}`))

				case r.URL.Path == "/api/v4/projects/123/merge_requests/1" && r.Method == "GET":
					_, _ = w.Write([]byte(`{"diff_refs": {"base_sha": "abc", "head_sha": "def", "start_sha": "abc"}}`))

				case r.URL.Path == "/api/v4/projects/123/merge_requests/1/discussions" && r.Method == "GET":
					_, _ = w.Write([]byte(tt.existingDiscussions))

				case r.URL.Path == "/api/v4/projects/123/merge_requests/1/discussions" && r.Method == "POST":
					discCreated++
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"id": "disc1"}`))

				default:
					// Handle discussion note updates
					if r.Method == "PUT" {
						discUpdated++
						_, _ = w.Write([]byte(`{"id": 1}`))
					} else {
						w.WriteHeader(http.StatusNotFound)
					}
				}
			}))
			defer server.Close()

			opts := CommentOptions{
				ProjectID:   "123",
				MergeReqIID: "1",
				GitLabURL:   server.URL + "/api/v4",
				Token:       "test-token",
				Verbose:     false,
			}

			result, err := PostComment(tt.analysis, opts)
			require.NoError(t, err)

			assert.True(t, result.Posted)
			assert.Equal(t, tt.expectNotesCreated, notesCreated, "notes created")
			assert.Equal(t, tt.expectNotesUpdated, notesUpdated, "notes updated")
			assert.Equal(t, tt.expectDiscCreated, discCreated, "discussions created")
			assert.Equal(t, tt.expectDiscUpdated, discUpdated, "discussions updated")
			assert.Equal(t, tt.expectInlineComments, result.InlineCommentsPosted)
			assert.Contains(t, result.Message, tt.expectMessageContains)
		})
	}
}

func TestGetMRDiffRefs(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		expectError    bool
		expectBaseSHA  string
	}{
		{
			name:           "success",
			responseStatus: http.StatusOK,
			responseBody:   `{"diff_refs": {"base_sha": "base123", "head_sha": "head456", "start_sha": "start789"}}`,
			expectError:    false,
			expectBaseSHA:  "base123",
		},
		{
			name:           "unauthorized",
			responseStatus: http.StatusUnauthorized,
			responseBody:   `{"error": "unauthorized"}`,
			expectError:    true,
			expectBaseSHA:  "",
		},
		{
			name:           "not found",
			responseStatus: http.StatusNotFound,
			responseBody:   `{"error": "not found"}`,
			expectError:    true,
			expectBaseSHA:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			refs, err := getMRDiffRefs(server.URL+"/api/v4", "123", "1", "test-token")

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectBaseSHA, refs.BaseSHA)
			}
		})
	}
}

func TestFindExistingKusariNote(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		expectNoteID int
	}{
		{
			name: "found kusari note with IGNORE_KUSARI_COMMENT marker",
			responseBody: `[
				{"id": 1, "body": "Some other comment"},
				{"id": 2, "body": "#### Kusari Analysis Results:\n\nContent\n<!-- IGNORE_KUSARI_COMMENT -->"},
				{"id": 3, "body": "Another comment"}
			]`,
			expectNoteID: 2,
		},
		{
			name: "found kusari note with Analysis Results text",
			responseBody: `[
				{"id": 1, "body": "Some other comment"},
				{"id": 2, "body": "#### Kusari Analysis Results:\n\nContent"},
				{"id": 3, "body": "Another comment"}
			]`,
			expectNoteID: 2,
		},
		{
			name: "found kusari note with legacy format",
			responseBody: `[
				{"id": 1, "body": "Some other comment"},
				{"id": 2, "body": "## Kusari Security Scan Results\n\nContent"},
				{"id": 3, "body": "Another comment"}
			]`,
			expectNoteID: 2,
		},
		{
			name: "no kusari note",
			responseBody: `[
				{"id": 1, "body": "Some comment"},
				{"id": 3, "body": "Another comment"}
			]`,
			expectNoteID: 0,
		},
		{
			name:         "empty notes",
			responseBody: `[]`,
			expectNoteID: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			noteID, err := findExistingKusariNote(server.URL+"/api/v4", "123", "1", "test-token")
			require.NoError(t, err)
			assert.Equal(t, tt.expectNoteID, noteID)
		})
	}
}

func TestPostNote(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		expectError    bool
	}{
		{
			name:           "success",
			responseStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name:           "unauthorized",
			responseStatus: http.StatusUnauthorized,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "test-token", r.Header.Get("PRIVATE-TOKEN"))

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(`{"id": 1}`))
			}))
			defer server.Close()

			err := postNote(server.URL, "test-token", "Test body")

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdateNote(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		expectError    bool
	}{
		{
			name:           "success",
			responseStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "not found",
			responseStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "/api/v4/projects/123/merge_requests/1/notes/999", r.URL.Path)

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(`{"id": 999}`))
			}))
			defer server.Close()

			err := updateNote(server.URL+"/api/v4", "123", "1", 999, "test-token", "Updated body")

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPostInlineComment(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		expectError    bool
	}{
		{
			name:           "success",
			responseStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name:           "bad request",
			responseStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v4/projects/123/merge_requests/1/discussions", r.URL.Path)

				var req discussionRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)

				assert.Equal(t, "Test message", req.Body)
				assert.Equal(t, "base123", req.Position.BaseSHA)
				assert.Equal(t, "src/main.go", req.Position.NewPath)
				assert.Equal(t, 42, req.Position.NewLine)

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(`{"id": "disc1"}`))
			}))
			defer server.Close()

			diffRefs := &mrDiffRefs{
				BaseSHA:  "base123",
				HeadSHA:  "head456",
				StartSHA: "start789",
			}

			err := postInlineComment(server.URL+"/api/v4", "123", "1", "test-token", diffRefs, "src/main.go", 42, "Test message")

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
