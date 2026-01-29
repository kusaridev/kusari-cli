// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/kusaridev/kusari-cli/api"
	"github.com/kusaridev/kusari-cli/pkg/comment"
)

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
func PostComment(analysis *api.SecurityAnalysis, opts CommentOptions) (*comment.CommentResult, error) {
	if analysis == nil {
		return &comment.CommentResult{
			Posted:      false,
			IssuesFound: 0,
			Message:     "No analysis results available - skipping comment",
		}, nil
	}

	// Check if there are any issues to report
	hasIssues, issueCount := comment.CheckForIssues(analysis)
	if !hasIssues {
		return &comment.CommentResult{
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
	commentBody := comment.FormatComment(analysis, opts.ConsoleURL)

	// Check for existing Kusari summary comment and update if found
	existingNoteID, err := findExistingKusariNote(apiURL, opts.ProjectID, opts.MergeReqIID, opts.Token)
	if err != nil {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: Could not check for existing comments: %v\n", err)
		}
	} else if opts.Verbose {
		if existingNoteID > 0 {
			fmt.Fprintf(os.Stderr, "Found existing Kusari summary comment (note ID: %d)\n", existingNoteID)
		} else {
			fmt.Fprintf(os.Stderr, "No existing Kusari summary comment found\n")
		}
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

	return &comment.CommentResult{
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

// listMRNotes retrieves all notes on a merge request
func listMRNotes(apiURL, projectID, mrIID, token string) ([]mrNote, error) {
	endpoint := fmt.Sprintf("%s/projects/%s/merge_requests/%s/notes", apiURL, projectID, mrIID)

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

	var notes []mrNote
	if err := json.NewDecoder(resp.Body).Decode(&notes); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return notes, nil
}

// findExistingKusariNote finds an existing Kusari summary comment on the MR
func findExistingKusariNote(apiURL, projectID, mrIID, token string) (int, error) {
	notes, err := listMRNotes(apiURL, projectID, mrIID, token)
	if err != nil {
		return 0, err
	}

	// Debug: log what we're searching through
	verbose := os.Getenv("KUSARI_DEBUG") == "true"
	if verbose {
		fmt.Fprintf(os.Stderr, "DEBUG: Searching through %d notes for existing Kusari comment\n", len(notes))
	}

	// Look for existing Kusari summary comment by marker
	// Check for primary marker first, then fall back to legacy text-based markers for backward compatibility
	for i, note := range notes {
		if verbose {
			preview := note.Body
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			fmt.Fprintf(os.Stderr, "DEBUG: Note %d (ID %d): %s\n", i, note.ID, preview)
		}

		// Primary marker (consistent with GitHub implementation)
		if strings.Contains(note.Body, "IGNORE_KUSARI_COMMENT") {
			if verbose {
				fmt.Fprintf(os.Stderr, "DEBUG: Found match at note ID %d via IGNORE_KUSARI_COMMENT marker\n", note.ID)
			}
			return note.ID, nil
		}

		// Legacy text-based markers for backward compatibility with old comments
		if strings.Contains(note.Body, "Kusari Analysis Results") ||
			strings.Contains(note.Body, "Kusari Security Scan Results") {
			if verbose {
				fmt.Fprintf(os.Stderr, "DEBUG: Found match at note ID %d via legacy text marker\n", note.ID)
			}
			return note.ID, nil
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "DEBUG: No existing Kusari note found\n")
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

	// Get existing notes to check for updates
	// Inline diff comments are returned by the Notes API, not the Discussions API
	existingNotes, err := listMRNotes(apiURL, opts.ProjectID, opts.MergeReqIID, opts.Token)
	if err != nil {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: Could not list existing notes: %v\n", err)
		}
		existingNotes = nil
	}

	posted := 0
	var lastErr error

	for _, issue := range analysis.RequiredCodeMitigations {
		// Skip issues without line numbers
		if issue.LineNumber == 0 {
			continue
		}

		// Format the inline comment message
		message := comment.FormatInlineComment(issue)

		// Check if we already have a comment at this location
		existingNoteID := findExistingInlineCommentInNotes(existingNotes, issue.Path, issue.LineNumber)

		if opts.Verbose {
			if existingNoteID > 0 {
				fmt.Fprintf(os.Stderr, "Found existing inline comment at %s:%d (note: %d)\n", issue.Path, issue.LineNumber, existingNoteID)
			} else {
				fmt.Fprintf(os.Stderr, "No existing inline comment found at %s:%d\n", issue.Path, issue.LineNumber)
			}
		}

		if existingNoteID > 0 {
			// Update existing comment
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Updating inline comment at %s:%d\n", issue.Path, issue.LineNumber)
			}
			err := updateNote(apiURL, opts.ProjectID, opts.MergeReqIID, existingNoteID, opts.Token, message)
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

// findExistingInlineCommentInNotes finds an existing Kusari inline comment at the given location
// by searching through the Notes API response. Inline diff comments created with position parameters
// are returned by the Notes API but NOT by the Discussions API.
// Returns the note ID if found, 0 otherwise
func findExistingInlineCommentInNotes(notes []mrNote, path string, line int) int {
	sanitizedPath := comment.SanitizePath(path)
	verbose := os.Getenv("KUSARI_DEBUG") == "true"

	if verbose {
		fmt.Fprintf(os.Stderr, "DEBUG: Looking for inline comment at %s:%d (sanitized: %s)\n", path, line, sanitizedPath)
		fmt.Fprintf(os.Stderr, "DEBUG: Searching through %d notes\n", len(notes))
	}

	// Regex to extract marker: <!-- KUSARI_INLINE:path:line -->
	markerRegex := regexp.MustCompile(`<!-- KUSARI_INLINE:([^:]+):(\d+) -->`)

	for i, note := range notes {
		if verbose {
			bodyPreview := note.Body
			if len(bodyPreview) > 100 {
				bodyPreview = bodyPreview[:100] + "..."
			}
			fmt.Fprintf(os.Stderr, "DEBUG: Note %d (ID %d): %s\n", i, note.ID, bodyPreview)
		}

		// Check if this is a Kusari inline comment by looking for the marker in the body
		if !strings.Contains(note.Body, "KUSARI_INLINE:") {
			if verbose {
				fmt.Fprintf(os.Stderr, "DEBUG:   -> No KUSARI_INLINE marker found\n")
			}
			continue
		}

		matches := markerRegex.FindStringSubmatch(note.Body)
		if len(matches) != 3 {
			if verbose {
				fmt.Fprintf(os.Stderr, "DEBUG:   -> Found KUSARI_INLINE marker but couldn't parse (matches: %d)\n", len(matches))
			}
			continue
		}

		commentPath := matches[1]
		commentLine := matches[2]

		if verbose {
			fmt.Fprintf(os.Stderr, "DEBUG:   -> Found Kusari comment for %s:%s\n", commentPath, commentLine)
		}

		// Match by path and line number from the marker
		if commentPath == sanitizedPath && commentLine == fmt.Sprintf("%d", line) {
			if verbose {
				fmt.Fprintf(os.Stderr, "DEBUG: MATCH! Found existing inline comment via marker (note: %d)\n", note.ID)
			}
			return note.ID
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "DEBUG: No existing inline comment found at %s:%d\n", path, line)
	}

	return 0
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
			NewPath:      comment.SanitizePath(path),
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
