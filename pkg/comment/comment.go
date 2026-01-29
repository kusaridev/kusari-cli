// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package comment

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/kusaridev/kusari-cli/api"
)

//go:embed templates/analysisComment.tmpl
var templateFS embed.FS

// AnalysisCommentData holds the data for the analysis comment template
type AnalysisCommentData struct {
	FinalAnalysis *api.SecurityAnalysis
	ConsoleURL    string
}

// CommentResult holds the result of posting a comment
type CommentResult struct {
	Posted               bool
	IssuesFound          int
	InlineCommentsPosted int
	Message              string
}

// CheckForIssues determines if there are issues to report
func CheckForIssues(analysis *api.SecurityAnalysis) (bool, int) {
	// If analysis failed, that's an issue
	if analysis.FailedAnalysis {
		return true, 1
	}

	issueCount := 0
	// If should not proceed, there are issues
	if !analysis.ShouldProceed {
		issueCount = len(analysis.RequiredCodeMitigations) + len(analysis.RequiredDependencyMitigations)
		if issueCount == 0 {
			issueCount = 1 // At least one issue if ShouldProceed is false
		}
		return true, issueCount
	}
	return issueCount > 0, issueCount
}

// FormatComment creates a markdown comment from analysis results using the shared template
func FormatComment(analysis *api.SecurityAnalysis, consoleURL string) string {
	tmplContent, err := templateFS.ReadFile("templates/analysisComment.tmpl")
	if err != nil {
		// Fallback to basic format if template fails
		return FormatCommentFallback(analysis, consoleURL)
	}

	tmpl, err := template.New("analysisComment").Parse(string(tmplContent))
	if err != nil {
		return FormatCommentFallback(analysis, consoleURL)
	}

	data := AnalysisCommentData{
		FinalAnalysis: analysis,
		ConsoleURL:    consoleURL,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return FormatCommentFallback(analysis, consoleURL)
	}

	return buf.String()
}

// FormatCommentFallback provides a basic format if template rendering fails
func FormatCommentFallback(analysis *api.SecurityAnalysis, consoleURL string) string {
	var sb strings.Builder

	sb.WriteString("#### Kusari Analysis Results:\n\n")

	if analysis.ShouldProceed {
		sb.WriteString("**:white_check_mark: No Flagged Issues Detected**\n")
		sb.WriteString("_All values appear to be within acceptable risk parameters._\n\n")
	} else {
		sb.WriteString("**:warning: Flagged Issues Detected**\n")
		sb.WriteString("_These changes contain flagged issues that may introduce security risks._\n\n")
	}

	if analysis.Justification != "" {
		sb.WriteString(analysis.Justification + "\n\n")
	}

	// Code mitigations
	if len(analysis.RequiredCodeMitigations) > 0 && !analysis.ShouldProceed {
		sb.WriteString("## Required Code Mitigations\n\n")
		for _, m := range analysis.RequiredCodeMitigations {
			sb.WriteString(fmt.Sprintf("### %s\n", m.Content))
			if m.LineNumber > 0 {
				sb.WriteString(fmt.Sprintf("- **Location:** %s:%d\n", m.Path, m.LineNumber))
			}
			if m.Code != "" {
				sb.WriteString("- **Potential Code Fix:**\n```\n")
				sb.WriteString(m.Code)
				sb.WriteString("\n```\n")
			}
			sb.WriteString("\n")
		}
	}

	// Dependency mitigations
	if len(analysis.RequiredDependencyMitigations) > 0 && !analysis.ShouldProceed {
		sb.WriteString("## Required Dependency Mitigations\n\n")
		for _, m := range analysis.RequiredDependencyMitigations {
			sb.WriteString(fmt.Sprintf("- %s\n", m.Content))
		}
		sb.WriteString("\n")
	}

	// Link to full results
	if consoleURL != "" {
		sb.WriteString(fmt.Sprintf("> **Note:** [View full detailed analysis result](%s) for more information.\n\n", consoleURL))
	}

	sb.WriteString("--------\n\n")
	sb.WriteString("<!-- IGNORE_KUSARI_COMMENT -->\n")

	return sb.String()
}

// FormatInlineComment creates the message for an inline code comment
// Includes a hidden marker for duplicate detection
func FormatInlineComment(issue api.CodeMitigationItem) string {
	var sb strings.Builder

	sb.WriteString("\U0001F512 **Kusari Security Issue**\n\n")
	sb.WriteString(issue.Content)

	if issue.Code != "" {
		sb.WriteString("\n\n**Recommended Code Changes:**\n```\n")
		sb.WriteString(issue.Code)
		sb.WriteString("\n```")
	}

	// Add hidden marker for duplicate detection by path:line
	// Format: <!-- KUSARI_INLINE:path:line -->
	sb.WriteString(fmt.Sprintf("\n\n<!-- KUSARI_INLINE:%s:%d -->", issue.Path, issue.LineNumber))

	return sb.String()
}

// SanitizePath ensures the path is in the correct format for APIs
func SanitizePath(path string) string {
	// Remove leading ./ if present
	path = strings.TrimPrefix(path, "./")
	// Remove leading / if present
	path = strings.TrimPrefix(path, "/")
	return path
}
