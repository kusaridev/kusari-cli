// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package comment

import (
	"strings"
	"testing"

	"github.com/kusaridev/kusari-cli/api"
	"github.com/stretchr/testify/assert"
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
			name: "should not proceed with no mitigations",
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
			name: "should not proceed with code mitigations",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  false,
				FailedAnalysis: false,
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Content: "Fix SQL injection", Path: "main.go", LineNumber: 10},
					{Content: "Fix XSS", Path: "handler.go", LineNumber: 20},
				},
				RequiredDependencyMitigations: nil,
			},
			expectHasIssues: true,
			expectCount:     2,
		},
		{
			name: "should not proceed with dependency mitigations",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:           false,
				FailedAnalysis:          false,
				RequiredCodeMitigations: nil,
				RequiredDependencyMitigations: []api.DependencyMitigationItem{
					{Content: "Update vulnerable package"},
				},
			},
			expectHasIssues: true,
			expectCount:     1,
		},
		{
			name: "should not proceed with both mitigations",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  false,
				FailedAnalysis: false,
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Content: "Fix SQL injection", Path: "main.go", LineNumber: 10},
				},
				RequiredDependencyMitigations: []api.DependencyMitigationItem{
					{Content: "Update vulnerable package"},
					{Content: "Remove deprecated package"},
				},
			},
			expectHasIssues: true,
			expectCount:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasIssues, count := CheckForIssues(tt.analysis)
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
			name:     "path with leading ./",
			input:    "./src/main.go",
			expected: "src/main.go",
		},
		{
			name:     "path with leading /",
			input:    "/src/main.go",
			expected: "src/main.go",
		},
		{
			name:     "path with both ./ and /",
			input:    "./src/main.go",
			expected: "src/main.go",
		},
		{
			name:     "clean path",
			input:    "src/main.go",
			expected: "src/main.go",
		},
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "just ./",
			input:    "./",
			expected: "",
		},
		{
			name:     "just /",
			input:    "/",
			expected: "",
		},
		{
			name:     "nested path with leading ./",
			input:    "./pkg/comment/comment.go",
			expected: "pkg/comment/comment.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatComment(t *testing.T) {
	tests := []struct {
		name             string
		analysis         *api.SecurityAnalysis
		consoleURL       string
		expectContains   []string
		expectNotContain []string
	}{
		{
			name: "should proceed - no issues",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  true,
				Justification:  "All changes look safe",
				FailedAnalysis: false,
			},
			consoleURL: "https://console.example.com/results/123",
			expectContains: []string{
				"Kusari Analysis Results",
				"No Flagged Issues Detected",
				"All changes look safe",
				"https://console.example.com/results/123",
				"IGNORE_KUSARI_COMMENT",
			},
			expectNotContain: []string{
				"Required Code Mitigations",
				"Required Dependency Mitigations",
			},
		},
		{
			name: "should not proceed - with issues",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  false,
				Justification:  "Security issues detected",
				FailedAnalysis: false,
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Content: "Fix SQL injection vulnerability", Path: "main.go", LineNumber: 42, Code: "db.Query(ctx, query, args...)"},
				},
				RequiredDependencyMitigations: []api.DependencyMitigationItem{
					{Content: "Update lodash to version 4.17.21"},
				},
			},
			consoleURL: "https://console.example.com/results/456",
			expectContains: []string{
				"Kusari Analysis Results",
				"Security issues detected",
				"https://console.example.com/results/456",
				"IGNORE_KUSARI_COMMENT",
			},
		},
		{
			name: "no console URL",
			analysis: &api.SecurityAnalysis{
				ShouldProceed:  true,
				Justification:  "All good",
				FailedAnalysis: false,
			},
			consoleURL: "",
			expectContains: []string{
				"Kusari Analysis Results",
				"IGNORE_KUSARI_COMMENT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatComment(tt.analysis, tt.consoleURL)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected, "Expected result to contain: %s", expected)
			}

			for _, notExpected := range tt.expectNotContain {
				assert.NotContains(t, result, notExpected, "Expected result to NOT contain: %s", notExpected)
			}
		})
	}
}

func TestFormatCommentFallback(t *testing.T) {
	tests := []struct {
		name           string
		analysis       *api.SecurityAnalysis
		consoleURL     string
		expectContains []string
	}{
		{
			name: "should proceed",
			analysis: &api.SecurityAnalysis{
				ShouldProceed: true,
				Justification: "Everything looks good",
			},
			consoleURL: "https://console.example.com",
			expectContains: []string{
				"#### Kusari Analysis Results:",
				":white_check_mark: No Flagged Issues Detected",
				"Everything looks good",
				"View full detailed analysis result",
				"IGNORE_KUSARI_COMMENT",
			},
		},
		{
			name: "should not proceed with code mitigations",
			analysis: &api.SecurityAnalysis{
				ShouldProceed: false,
				Justification: "Issues found",
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{Content: "SQL injection fix", Path: "db.go", LineNumber: 15, Code: "use parameterized queries"},
				},
			},
			consoleURL: "",
			expectContains: []string{
				":warning: Flagged Issues Detected",
				"## Required Code Mitigations",
				"SQL injection fix",
				"db.go:15",
				"use parameterized queries",
			},
		},
		{
			name: "should not proceed with dependency mitigations",
			analysis: &api.SecurityAnalysis{
				ShouldProceed: false,
				RequiredDependencyMitigations: []api.DependencyMitigationItem{
					{Content: "Update axios to 1.0.0"},
					{Content: "Remove deprecated package"},
				},
			},
			consoleURL: "",
			expectContains: []string{
				"## Required Dependency Mitigations",
				"Update axios to 1.0.0",
				"Remove deprecated package",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCommentFallback(tt.analysis, tt.consoleURL)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestFormatInlineComment(t *testing.T) {
	tests := []struct {
		name           string
		issue          api.CodeMitigationItem
		expectContains []string
	}{
		{
			name: "with code suggestion",
			issue: api.CodeMitigationItem{
				Content:    "SQL injection vulnerability detected",
				Path:       "main.go",
				LineNumber: 42,
				Code:       "db.Query(ctx, query, args...)",
			},
			expectContains: []string{
				"Kusari Security Issue",
				"SQL injection vulnerability detected",
				"Recommended Code Changes:",
				"db.Query(ctx, query, args...)",
				"<!-- KUSARI_INLINE:main.go:42 -->",
			},
		},
		{
			name: "without code suggestion",
			issue: api.CodeMitigationItem{
				Content:    "Hardcoded credentials detected",
				Path:       "config.go",
				LineNumber: 10,
				Code:       "",
			},
			expectContains: []string{
				"Kusari Security Issue",
				"Hardcoded credentials detected",
				"<!-- KUSARI_INLINE:config.go:10 -->",
			},
		},
		{
			name: "with special characters in path",
			issue: api.CodeMitigationItem{
				Content:    "Issue found",
				Path:       "src/components/auth/login.tsx",
				LineNumber: 100,
				Code:       "",
			},
			expectContains: []string{
				"<!-- KUSARI_INLINE:src/components/auth/login.tsx:100 -->",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatInlineComment(tt.issue)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}

			// Verify it doesn't contain code block if no code provided
			if tt.issue.Code == "" {
				assert.NotContains(t, result, "Recommended Code Changes:")
			}
		})
	}
}

func TestFormatInlineCommentMarkerFormat(t *testing.T) {
	// Test that the marker is properly formatted for parsing
	issue := api.CodeMitigationItem{
		Content:    "Test issue",
		Path:       "test/file.go",
		LineNumber: 123,
	}

	result := FormatInlineComment(issue)

	// The marker should be parseable
	assert.True(t, strings.Contains(result, "<!-- KUSARI_INLINE:test/file.go:123 -->"))
}
