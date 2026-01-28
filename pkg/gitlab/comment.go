// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package gitlab

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/kusaridev/kusari-cli/api"
)

//go:embed templates/analysisComment.tmpl
var templateFS embed.FS

// analysisCommentData holds the data for the analysis comment template
type analysisCommentData struct {
	FinalAnalysis *api.SecurityAnalysis
	ConsoleURL    string
}

const (
	defaultGitLabAPIURL = "https://gitlab.com/api/v4"
)

// CommentOptions holds the configuration for posting a comment to GitLab
type CommentOptions struct {
	ProjectID   string
	MergeReqIID string
	GitLabURL   string
	Token       string
	ConsoleURL  string // Link to full results in Kusari console
	Verbose     bool
}

// CommentResult holds the result of posting a comment
type CommentResult struct {
	Posted               bool
	IssuesFound          int
	InlineCommentsPosted int
	Message              string
}

// mrDiffRefs holds the SHA references needed for inline comments
type mrDiffRefs struct {
	BaseSHA  string `json:"base_sha"`
	HeadSHA  string `json:"head_sha"`
	StartSHA string `json:"start_sha"`
}

// mrInfo holds merge request information from GitLab API
type mrInfo struct {
	DiffRefs mrDiffRefs `json:"diff_refs"`
}

// PostComment posts scan results as a comment to a GitLab merge request
// Returns without posting if no issues are found (ShouldProceed is true and no mitigations)
// If an existing Kusari comment exists, it will be updated instead of creating a new one
func PostComment(analysis *api.SecurityAnalysis, opts CommentOptions) (*CommentResult, error) {
	if analysis == nil {
		return &CommentResult{
			Posted:      false,
			IssuesFound: 0,
			Message:     "No analysis results available - skipping comment",
		}, nil
	}

	// Check if there are any issues to report
	hasIssues, issueCount := checkForIssues(analysis)
	if !hasIssues {
		return &CommentResult{
			Posted:      false,
			IssuesFound: 0,
			Message:     "No issues found - skipping comment",
		}, nil
	}

	// Determine API URL
	apiURL := opts.GitLabURL
	if apiURL == "" {
		apiURL = defaultGitLabAPIURL
	}
	apiURL = strings.TrimSuffix(apiURL, "/")

	// Format comment body from analysis results
	commentBody := formatComment(analysis, opts.ConsoleURL)

	// Check for existing Kusari summary comment and update if found
	existingNoteID, err := findExistingKusariNote(apiURL, opts.ProjectID, opts.MergeReqIID, opts.Token)
	if err != nil && opts.Verbose {
		fmt.Fprintf(os.Stderr, "Warning: Could not check for existing comments: %v\n", err)
	}

	if existingNoteID > 0 {
		// Update existing comment
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Updating existing summary comment (note ID: %d)\n", existingNoteID)
		}
		if err := updateNote(apiURL, opts.ProjectID, opts.MergeReqIID, existingNoteID, opts.Token, commentBody); err != nil {
			return nil, fmt.Errorf("failed to update comment on GitLab: %w", err)
		}
	} else {
		// Post new comment
		notesEndpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%s/notes",
			apiURL, opts.ProjectID, opts.MergeReqIID)

		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Posting new summary comment to: %s\n", notesEndpoint)
		}

		if err := postNote(notesEndpoint, opts.Token, commentBody); err != nil {
			return nil, fmt.Errorf("failed to post comment to GitLab: %w", err)
		}
	}

	// Post or update inline comments for code mitigations
	inlineCount := 0
	if len(analysis.RequiredCodeMitigations) > 0 && !analysis.ShouldProceed {
		posted, err := postCodeMitigationComments(analysis, opts, apiURL)
		if err != nil {
			// Log but don't fail - inline comments are best-effort
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to post some inline comments: %v\n", err)
			}
		}
		inlineCount = posted
	}

	action := "Posted"
	if existingNoteID > 0 {
		action = "Updated"
	}
	message := fmt.Sprintf("%s comment with %d issue(s) to MR !%s", action, issueCount, opts.MergeReqIID)
	if inlineCount > 0 {
		message = fmt.Sprintf("%s comment with %d issue(s) and %d inline comment(s) to MR !%s", action, issueCount, inlineCount, opts.MergeReqIID)
	}

	return &CommentResult{
		Posted:               true,
		IssuesFound:          issueCount,
		InlineCommentsPosted: inlineCount,
		Message:              message,
	}, nil
}

// mrNote represents a note (comment) on a merge request
type mrNote struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

// findExistingKusariNote finds an existing Kusari summary comment on the MR
func findExistingKusariNote(apiURL, projectID, mrIID, token string) (int, error) {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%s/notes", apiURL, projectID, mrIID)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var notes []mrNote
	if err := json.NewDecoder(resp.Body).Decode(&notes); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Look for existing Kusari summary comment (check multiple markers for compatibility)
	for _, note := range notes {
		if strings.Contains(note.Body, "IGNORE_KUSARI_COMMENT") ||
			strings.Contains(note.Body, "Kusari Analysis Results") ||
			strings.Contains(note.Body, "Kusari Security Scan Results") {
			return note.ID, nil
		}
	}

	return 0, nil
}

// updateNote updates an existing note on a merge request
func updateNote(apiURL, projectID, mrIID string, noteID int, token, body string) error {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%s/notes/%d", apiURL, projectID, mrIID, noteID)

	reqBody := noteRequest{Body: body}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// postCodeMitigationComments posts or updates inline comments for each code mitigation
func postCodeMitigationComments(analysis *api.SecurityAnalysis, opts CommentOptions, apiURL string) (int, error) {
	// Get MR diff refs for positioning inline comments
	diffRefs, err := getMRDiffRefs(apiURL, opts.ProjectID, opts.MergeReqIID, opts.Token)
	if err != nil {
		return 0, fmt.Errorf("failed to get MR diff refs: %w", err)
	}

	// Get existing discussions to check for updates
	existingDiscussions, err := listMRDiscussions(apiURL, opts.ProjectID, opts.MergeReqIID, opts.Token)
	if err != nil {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: Could not list existing discussions: %v\n", err)
		}
		existingDiscussions = nil
	}

	posted := 0
	var lastErr error

	for _, issue := range analysis.RequiredCodeMitigations {
		// Skip issues without line numbers
		if issue.LineNumber == 0 {
			continue
		}

		// Format the inline comment message
		message := formatInlineComment(issue)

		// Check if we already have a comment at this location
		existingDiscussionID, existingNoteID := findExistingInlineComment(existingDiscussions, issue.Path, issue.LineNumber)

		if existingDiscussionID != "" && existingNoteID > 0 {
			// Update existing comment
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Updating inline comment at %s:%d\n", issue.Path, issue.LineNumber)
			}
			err := updateDiscussionNote(apiURL, opts.ProjectID, opts.MergeReqIID, existingDiscussionID, existingNoteID, opts.Token, message)
			if err != nil {
				lastErr = err
				if opts.Verbose {
					fmt.Fprintf(os.Stderr, "Warning: Failed to update inline comment at %s:%d: %v\n", issue.Path, issue.LineNumber, err)
				}
				continue
			}
		} else {
			// Post new comment
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Posting inline comment at %s:%d\n", issue.Path, issue.LineNumber)
			}
			err := postInlineComment(apiURL, opts.ProjectID, opts.MergeReqIID, opts.Token, diffRefs, issue.Path, issue.LineNumber, message)
			if err != nil {
				lastErr = err
				if opts.Verbose {
					fmt.Fprintf(os.Stderr, "Warning: Failed to post inline comment at %s:%d: %v\n", issue.Path, issue.LineNumber, err)
				}
				continue
			}
		}
		posted++
	}

	return posted, lastErr
}

// formatInlineComment creates the message for an inline code comment
func formatInlineComment(issue api.CodeMitigationItem) string {
	var sb strings.Builder

	sb.WriteString("🔒 **Kusari Security Issue**\n\n")
	sb.WriteString(issue.Content)

	if issue.Code != "" {
		sb.WriteString("\n\n**Recommended Code Changes:**\n```\n")
		sb.WriteString(issue.Code)
		sb.WriteString("\n```")
	}

	return sb.String()
}

// getMRDiffRefs retrieves the diff refs from a merge request
func getMRDiffRefs(apiURL, projectID, mrIID, token string) (*mrDiffRefs, error) {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%s", apiURL, projectID, mrIID)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var mr mrInfo
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &mr.DiffRefs, nil
}

// discussionNote represents a note in a discussion
type discussionNote struct {
	ID       int    `json:"id"`
	Body     string `json:"body"`
	Position *struct {
		NewPath string `json:"new_path"`
		NewLine int    `json:"new_line"`
	} `json:"position"`
}

// discussion represents a GitLab discussion
type discussion struct {
	ID    string           `json:"id"`
	Notes []discussionNote `json:"notes"`
}

// listMRDiscussions retrieves existing discussions on a merge request
func listMRDiscussions(apiURL, projectID, mrIID, token string) ([]discussion, error) {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%s/discussions", apiURL, projectID, mrIID)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var discussions []discussion
	if err := json.NewDecoder(resp.Body).Decode(&discussions); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return discussions, nil
}

// findExistingInlineComment finds an existing Kusari inline comment at the given location
// Returns the discussion ID and note ID if found, empty string and 0 otherwise
func findExistingInlineComment(discussions []discussion, path string, line int) (string, int) {
	for _, d := range discussions {
		for _, note := range d.Notes {
			// Check if this is a Kusari comment at the same location
			if note.Position != nil &&
				note.Position.NewPath == sanitizePath(path) &&
				note.Position.NewLine == line &&
				strings.Contains(note.Body, "Kusari") {
				return d.ID, note.ID
			}
		}
	}
	return "", 0
}

// updateDiscussionNote updates an existing note in a discussion
func updateDiscussionNote(apiURL, projectID, mrIID, discussionID string, noteID int, token, body string) error {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%s/discussions/%s/notes/%d",
		apiURL, projectID, mrIID, discussionID, noteID)

	reqBody := noteRequest{Body: body}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// discussionRequest is the request body for creating a discussion with inline comment
type discussionRequest struct {
	Body     string                    `json:"body"`
	Position discussionPositionRequest `json:"position"`
}

// discussionPositionRequest represents the position for an inline comment
type discussionPositionRequest struct {
	BaseSHA      string `json:"base_sha"`
	StartSHA     string `json:"start_sha"`
	HeadSHA      string `json:"head_sha"`
	PositionType string `json:"position_type"`
	NewPath      string `json:"new_path"`
	NewLine      int    `json:"new_line"`
}

// postInlineComment posts an inline comment on a specific line
func postInlineComment(apiURL, projectID, mrIID, token string, diffRefs *mrDiffRefs, path string, line int, message string) error {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%s/discussions", apiURL, projectID, mrIID)

	reqBody := discussionRequest{
		Body: message,
		Position: discussionPositionRequest{
			BaseSHA:      diffRefs.BaseSHA,
			StartSHA:     diffRefs.StartSHA,
			HeadSHA:      diffRefs.HeadSHA,
			PositionType: "text",
			NewPath:      sanitizePath(path),
			NewLine:      line,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// sanitizePath ensures the path is in the correct format for GitLab
func sanitizePath(path string) string {
	// Remove leading ./ if present
	path = strings.TrimPrefix(path, "./")
	// Remove leading / if present
	path = strings.TrimPrefix(path, "/")
	return path
}

// checkForIssues determines if there are issues to report
func checkForIssues(analysis *api.SecurityAnalysis) (bool, int) {
	// If analysis failed, that's an issue
	if analysis.FailedAnalysis {
		return true, 1
	}

	// If should not proceed, there are issues
	if !analysis.ShouldProceed {
		issueCount := len(analysis.RequiredCodeMitigations) + len(analysis.RequiredDependencyMitigations)
		if issueCount == 0 {
			issueCount = 1 // At least one issue if ShouldProceed is false
		}
		return true, issueCount
	}

	// Even if ShouldProceed is true, report if there are mitigations
	issueCount := len(analysis.RequiredCodeMitigations) + len(analysis.RequiredDependencyMitigations)
	return issueCount > 0, issueCount
}

// formatComment creates a GitLab-friendly markdown comment from analysis results
// Uses the same template structure as the GitHub implementation for consistency
func formatComment(analysis *api.SecurityAnalysis, consoleURL string) string {
	tmplContent, err := templateFS.ReadFile("templates/analysisComment.tmpl")
	if err != nil {
		// Fallback to basic format if template fails
		return formatCommentFallback(analysis, consoleURL)
	}

	tmpl, err := template.New("analysisComment").Parse(string(tmplContent))
	if err != nil {
		return formatCommentFallback(analysis, consoleURL)
	}

	data := analysisCommentData{
		FinalAnalysis: analysis,
		ConsoleURL:    consoleURL,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return formatCommentFallback(analysis, consoleURL)
	}

	return buf.String()
}

// formatCommentFallback provides a basic format if template rendering fails
func formatCommentFallback(analysis *api.SecurityAnalysis, consoleURL string) string {
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

// noteRequest is the request body for GitLab's notes API
type noteRequest struct {
	Body string `json:"body"`
}

// postNote posts a note (comment) to a GitLab merge request
func postNote(endpoint, token, body string) error {
	reqBody := noteRequest{Body: body}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetTokenFromEnv retrieves the GitLab token from environment variables
// It checks GITLAB_TOKEN first, then falls back to CI_JOB_TOKEN
func GetTokenFromEnv() string {
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("CI_JOB_TOKEN")
}

// GetGitLabAPIURLFromEnv retrieves the GitLab API URL from environment
// Returns empty string if not set (will use default gitlab.com)
func GetGitLabAPIURLFromEnv() string {
	// Check for explicit API URL
	if url := os.Getenv("GITLAB_API_URL"); url != "" {
		return url
	}
	// Check for CI_SERVER_URL and construct API URL
	if serverURL := os.Getenv("CI_SERVER_URL"); serverURL != "" {
		return strings.TrimSuffix(serverURL, "/") + "/api/v4"
	}
	return ""
}

// GetMRInfoFromEnv retrieves MR info from GitLab CI environment variables
func GetMRInfoFromEnv() (projectID, mrIID string) {
	return os.Getenv("CI_PROJECT_ID"), os.Getenv("CI_MERGE_REQUEST_IID")
}
