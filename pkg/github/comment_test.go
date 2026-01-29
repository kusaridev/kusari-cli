// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kusaridev/kusari-cli/api"
	"github.com/kusaridev/kusari-cli/pkg/comment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostComment(t *testing.T) {
	tests := []struct {
		name          string
		analysis      *api.SecurityAnalysis
		setupServer   func() *httptest.Server
		expectPosted  bool
		expectError   bool
		expectMessage string
		expectIssues  int
	}{
		{
			name:     "nil analysis returns early",
			analysis: nil,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Fatal("Server should not be called for nil analysis")
				}))
			},
			expectPosted:  false,
			expectError:   false,
			expectMessage: "No analysis results available - skipping comment",
			expectIssues:  0,
		},
		{
			name: "no issues - should proceed true",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  true,
				FailedAnalysis: false,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Fatal("Server should not be called when no issues")
				}))
			},
			expectPosted:  false,
			expectError:   false,
			expectMessage: "No issues found - skipping comment",
			expectIssues:  0,
		},
		{
			name: "creates new comment when no existing",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  false,
				FailedAnalysis: false,
				Justification:  "Security issues found",
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == "GET" && r.URL.Path == "/repos/owner/repo/issues/1/comments":
						// Return empty comments list
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode([]issueComment{})
					case r.Method == "POST" && r.URL.Path == "/repos/owner/repo/issues/1/comments":
						w.WriteHeader(http.StatusCreated)
					default:
						t.Fatalf("Unexpected request: %s %s", r.Method, r.URL.Path)
					}
				}))
			},
			expectPosted:  true,
			expectError:   false,
			expectMessage: "Posted comment with 1 issue(s) to PR #1",
			expectIssues:  1,
		},
		{
			name: "updates existing comment",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  false,
				FailedAnalysis: false,
				Justification:  "Security issues found",
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == "GET" && r.URL.Path == "/repos/owner/repo/issues/1/comments":
						// Return existing Kusari comment
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode([]issueComment{
							{ID: 123, Body: "Some other comment"},
							{ID: 456, Body: "Kusari Analysis Results\n<!-- IGNORE_KUSARI_COMMENT -->"},
						})
					case r.Method == "PATCH" && r.URL.Path == "/repos/owner/repo/issues/comments/456":
						w.WriteHeader(http.StatusOK)
					default:
						t.Fatalf("Unexpected request: %s %s", r.Method, r.URL.Path)
					}
				}))
			},
			expectPosted:  true,
			expectError:   false,
			expectMessage: "Updated comment with 1 issue(s) to PR #1",
			expectIssues:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			opts := CommentOptions{
				Owner:      "owner",
				Repo:       "repo",
				PRNumber:   1,
				GitHubURL:  server.URL,
				Token:      "test-token",
				ConsoleURL: "https://console.example.com",
				Verbose:    false,
			}

			result, err := PostComment(tt.analysis, opts)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectPosted, result.Posted)
			assert.Equal(t, tt.expectIssues, result.IssuesFound)
			assert.Contains(t, result.Message, tt.expectMessage)
		})
	}
}

func TestFindExistingInlineComment(t *testing.T) {
	tests := []struct {
		name     string
		comments []prComment
		path     string
		line     int
		expected int64
	}{
		{
			name:     "empty comments list",
			comments: []prComment{},
			path:     "main.go",
			line:     10,
			expected: 0,
		},
		{
			name: "no matching comment",
			comments: []prComment{
				{ID: 1, Body: "Regular comment"},
				{ID: 2, Body: "Another comment"},
			},
			path:     "main.go",
			line:     10,
			expected: 0,
		},
		{
			name: "finds matching comment by marker",
			comments: []prComment{
				{ID: 1, Body: "Regular comment"},
				{ID: 2, Body: "Kusari issue\n<!-- KUSARI_INLINE:main.go:10 -->"},
			},
			path:     "main.go",
			line:     10,
			expected: 2,
		},
		{
			name: "does not match different path",
			comments: []prComment{
				{ID: 1, Body: "<!-- KUSARI_INLINE:other.go:10 -->"},
			},
			path:     "main.go",
			line:     10,
			expected: 0,
		},
		{
			name: "does not match different line",
			comments: []prComment{
				{ID: 1, Body: "<!-- KUSARI_INLINE:main.go:20 -->"},
			},
			path:     "main.go",
			line:     10,
			expected: 0,
		},
		{
			name: "handles complex path",
			comments: []prComment{
				{ID: 42, Body: "Security issue\n<!-- KUSARI_INLINE:src/components/auth/login.tsx:100 -->"},
			},
			path:     "src/components/auth/login.tsx",
			line:     100,
			expected: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findExistingInlineComment(tt.comments, tt.path, tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTokenFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "GITHUB_TOKEN set",
			envVars:  map[string]string{"GITHUB_TOKEN": "gh-token-123"},
			expected: "gh-token-123",
		},
		{
			name:     "GH_TOKEN set",
			envVars:  map[string]string{"GH_TOKEN": "gh-token-456"},
			expected: "gh-token-456",
		},
		{
			name:     "GITHUB_TOKEN takes precedence",
			envVars:  map[string]string{"GITHUB_TOKEN": "primary", "GH_TOKEN": "fallback"},
			expected: "primary",
		},
		{
			name:     "no token set",
			envVars:  map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			_ = os.Unsetenv("GITHUB_TOKEN")
			_ = os.Unsetenv("GH_TOKEN")

			// Set test env vars
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
			}

			result := GetTokenFromEnv()
			assert.Equal(t, tt.expected, result)

			// Cleanup
			for k := range tt.envVars {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestGetGitHubAPIURLFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "GITHUB_API_URL set",
			envVars:  map[string]string{"GITHUB_API_URL": "https://api.github.enterprise.com"},
			expected: "https://api.github.enterprise.com",
		},
		{
			name:     "no URL set",
			envVars:  map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Unsetenv("GITHUB_API_URL")

			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
			}

			result := GetGitHubAPIURLFromEnv()
			assert.Equal(t, tt.expected, result)

			for k := range tt.envVars {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestGetPRInfoFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		envVars       map[string]string
		expectedOwner string
		expectedRepo  string
		expectedPRNum int
	}{
		{
			name: "full PR info from GITHUB_REF",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "myorg/myrepo",
				"GITHUB_REF":        "refs/pull/42/merge",
			},
			expectedOwner: "myorg",
			expectedRepo:  "myrepo",
			expectedPRNum: 42,
		},
		{
			name: "PR info from GITHUB_REF_NAME",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "owner/repo",
				"GITHUB_REF_NAME":   "123/merge",
			},
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedPRNum: 123,
		},
		{
			name: "only repository set",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "owner/repo",
			},
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedPRNum: 0,
		},
		{
			name:          "nothing set",
			envVars:       map[string]string{},
			expectedOwner: "",
			expectedRepo:  "",
			expectedPRNum: 0,
		},
		{
			name: "invalid repository format",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "invalid",
			},
			expectedOwner: "",
			expectedRepo:  "",
			expectedPRNum: 0,
		},
		{
			name: "GITHUB_REF_NAME takes precedence over GITHUB_REF",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "owner/repo",
				"GITHUB_REF_NAME":   "100/merge",
				"GITHUB_REF":        "refs/pull/200/merge",
			},
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedPRNum: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			_ = os.Unsetenv("GITHUB_REPOSITORY")
			_ = os.Unsetenv("GITHUB_REF")
			_ = os.Unsetenv("GITHUB_REF_NAME")

			// Set test env vars
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
			}

			owner, repo, prNum := GetPRInfoFromEnv()
			assert.Equal(t, tt.expectedOwner, owner)
			assert.Equal(t, tt.expectedRepo, repo)
			assert.Equal(t, tt.expectedPRNum, prNum)

			// Cleanup
			for k := range tt.envVars {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestListIssueComments(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		expectError bool
		expectCount int
	}{
		{
			name: "successful response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Contains(t, r.URL.Path, "/repos/owner/repo/issues/1/comments")
					assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode([]issueComment{
						{ID: 1, Body: "Comment 1"},
						{ID: 2, Body: "Comment 2"},
					})
				}))
			},
			expectError: false,
			expectCount: 2,
		},
		{
			name: "API error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
				}))
			},
			expectError: true,
			expectCount: 0,
		},
		{
			name: "empty response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode([]issueComment{})
				}))
			},
			expectError: false,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			comments, err := listIssueComments(server.URL, "owner", "repo", 1, "test-token")

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, comments, tt.expectCount)
		})
	}
}

func TestCreateIssueComment(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		expectError bool
	}{
		{
			name: "successful creation",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					var body map[string]string
					_ = json.NewDecoder(r.Body).Decode(&body)
					assert.Equal(t, "Test comment body", body["body"])

					w.WriteHeader(http.StatusCreated)
				}))
			},
			expectError: false,
		},
		{
			name: "API error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Forbidden"}`))
				}))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			err := createIssueComment(server.URL, "owner", "repo", 1, "test-token", "Test comment body")

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestUpdateIssueComment(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		expectError bool
	}{
		{
			name: "successful update",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "PATCH", r.Method)
					assert.Contains(t, r.URL.Path, "/issues/comments/123")

					w.WriteHeader(http.StatusOK)
				}))
			},
			expectError: false,
		},
		{
			name: "comment not found",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			err := updateIssueComment(server.URL, "owner", "repo", 123, "test-token", "Updated body")

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestFindExistingKusariComment(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		expected    int64
		expectError bool
	}{
		{
			name: "finds comment with IGNORE_KUSARI_COMMENT marker",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode([]issueComment{
						{ID: 1, Body: "Random comment"},
						{ID: 42, Body: "Analysis\n<!-- IGNORE_KUSARI_COMMENT -->"},
					})
				}))
			},
			expected:    42,
			expectError: false,
		},
		{
			name: "finds comment with legacy Kusari Analysis Results marker",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode([]issueComment{
						{ID: 99, Body: "#### Kusari Analysis Results:\nSome content"},
					})
				}))
			},
			expected:    99,
			expectError: false,
		},
		{
			name: "no existing comment",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode([]issueComment{
						{ID: 1, Body: "Unrelated comment"},
					})
				}))
			},
			expected:    0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			result, err := findExistingKusariComment(server.URL, "owner", "repo", 1, "test-token")

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCommentOptionsDefaults(t *testing.T) {
	// Test that empty GitHubURL gets defaulted
	analysis := &api.SecurityAnalysis{
		ShouldProceed: false,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]issueComment{})
		case "POST":
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()

	opts := CommentOptions{
		Owner:     "owner",
		Repo:      "repo",
		PRNumber:  1,
		GitHubURL: server.URL, // Using test server URL
		Token:     "token",
	}

	result, err := PostComment(analysis, opts)
	require.NoError(t, err)
	assert.True(t, result.Posted)
}

func TestPostCommentWithCodeMitigations(t *testing.T) {
	analysis := &api.SecurityAnalysis{
		ShouldProceed: false,
		RequiredCodeMitigations: []api.CodeMitigationItem{
			{Content: "SQL injection", Path: "main.go", LineNumber: 10, Code: "fix code"},
		},
	}

	prInfoCalled := false
	reviewCommentsCalled := false
	createReviewCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/repos/owner/repo/issues/1/comments":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]issueComment{})
		case r.Method == "POST" && r.URL.Path == "/repos/owner/repo/issues/1/comments":
			w.WriteHeader(http.StatusCreated)
		case r.Method == "GET" && r.URL.Path == "/repos/owner/repo/pulls/1":
			prInfoCalled = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(pullRequest{Head: struct {
				SHA string `json:"sha"`
			}{SHA: "abc123"}})
		case r.Method == "GET" && r.URL.Path == "/repos/owner/repo/pulls/1/comments":
			reviewCommentsCalled = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]prComment{})
		case r.Method == "POST" && r.URL.Path == "/repos/owner/repo/pulls/1/comments":
			createReviewCalled = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	opts := CommentOptions{
		Owner:     "owner",
		Repo:      "repo",
		PRNumber:  1,
		GitHubURL: server.URL,
		Token:     "token",
	}

	result, err := PostComment(analysis, opts)
	require.NoError(t, err)

	assert.True(t, result.Posted)
	assert.Equal(t, 1, result.IssuesFound)
	assert.Equal(t, 1, result.InlineCommentsPosted)
	assert.True(t, prInfoCalled, "Should have fetched PR info")
	assert.True(t, reviewCommentsCalled, "Should have fetched review comments")
	assert.True(t, createReviewCalled, "Should have created review comment")
}

// Test that the comment package integration works correctly
func TestCommentPackageIntegration(t *testing.T) {
	analysis := &api.SecurityAnalysis{
		ShouldProceed: false,
		Justification: "Test justification",
	}

	// Verify CheckForIssues is called correctly
	hasIssues, count := comment.CheckForIssues(analysis)
	assert.True(t, hasIssues)
	assert.Equal(t, 1, count)

	// Verify FormatComment produces expected output
	formatted := comment.FormatComment(analysis, "https://example.com")
	assert.Contains(t, formatted, "Kusari Analysis Results")
	assert.Contains(t, formatted, "IGNORE_KUSARI_COMMENT")
}
