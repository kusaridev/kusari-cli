// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kusaridev/kusari-cli/api"
	"github.com/kusaridev/kusari-cli/pkg/comment"
)

const (
	defaultGitHubAPIURL = "https://api.github.com"
)

// CommentOptions holds the configuration for posting a comment to GitHub
type CommentOptions struct {
	Owner      string
	Repo       string
	PRNumber   int
	GitHubURL  string
	Token      string
	ConsoleURL string // Link to full results in Kusari console
	Verbose    bool
}

// issueComment represents a GitHub issue/PR comment
type issueComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

// prComment represents a GitHub PR review comment
type prComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

// pullRequest represents minimal PR info needed for comments
type pullRequest struct {
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// PostComment posts scan results as a comment to a GitHub pull request
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
	apiURL := opts.GitHubURL
	if apiURL == "" {
		apiURL = defaultGitHubAPIURL
	}
	apiURL = strings.TrimSuffix(apiURL, "/")

	// Format comment body from analysis results
	commentBody := comment.FormatComment(analysis, opts.ConsoleURL)

	// Check for existing Kusari summary comment and update if found
	existingCommentID, err := findExistingKusariComment(apiURL, opts.Owner, opts.Repo, opts.PRNumber, opts.Token)
	if err != nil {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: Could not check for existing comments: %v\n", err)
		}
	} else if opts.Verbose {
		if existingCommentID > 0 {
			fmt.Fprintf(os.Stderr, "Found existing Kusari summary comment (ID: %d)\n", existingCommentID)
		} else {
			fmt.Fprintf(os.Stderr, "No existing Kusari summary comment found\n")
		}
	}

	if existingCommentID > 0 {
		// Update existing comment
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Updating existing summary comment (ID: %d)\n", existingCommentID)
		}
		if err := updateIssueComment(apiURL, opts.Owner, opts.Repo, existingCommentID, opts.Token, commentBody); err != nil {
			return nil, fmt.Errorf("failed to update comment on GitHub: %w", err)
		}
	} else {
		// Post new comment
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Posting new summary comment\n")
		}
		if err := createIssueComment(apiURL, opts.Owner, opts.Repo, opts.PRNumber, opts.Token, commentBody); err != nil {
			return nil, fmt.Errorf("failed to post comment to GitHub: %w", err)
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
	if existingCommentID > 0 {
		action = "Updated"
	}
	message := fmt.Sprintf("%s comment with %d issue(s) to PR #%d", action, issueCount, opts.PRNumber)
	if inlineCount > 0 {
		message = fmt.Sprintf("%s comment with %d issue(s) and %d inline comment(s) to PR #%d", action, issueCount, inlineCount, opts.PRNumber)
	}

	return &comment.CommentResult{
		Posted:               true,
		IssuesFound:          issueCount,
		InlineCommentsPosted: inlineCount,
		Message:              message,
	}, nil
}

// listIssueComments retrieves all comments on a PR (issue comments)
func listIssueComments(apiURL, owner, repo string, prNumber int, token string) ([]issueComment, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", apiURL, owner, repo, prNumber)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var comments []issueComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return comments, nil
}

// findExistingKusariComment finds an existing Kusari summary comment on the PR
func findExistingKusariComment(apiURL, owner, repo string, prNumber int, token string) (int64, error) {
	comments, err := listIssueComments(apiURL, owner, repo, prNumber, token)
	if err != nil {
		return 0, err
	}

	verbose := os.Getenv("KUSARI_DEBUG") == "true"
	if verbose {
		fmt.Fprintf(os.Stderr, "DEBUG: Searching through %d comments for existing Kusari comment\n", len(comments))
	}

	// Look for existing Kusari summary comment by marker
	for _, c := range comments {
		// Primary marker (consistent with GitLab implementation)
		if strings.Contains(c.Body, "IGNORE_KUSARI_COMMENT") {
			if verbose {
				fmt.Fprintf(os.Stderr, "DEBUG: Found match at comment ID %d via IGNORE_KUSARI_COMMENT marker\n", c.ID)
			}
			return c.ID, nil
		}

		// Legacy text-based markers for backward compatibility
		if strings.Contains(c.Body, "Kusari Analysis Results") ||
			strings.Contains(c.Body, "Kusari Security Scan Results") {
			if verbose {
				fmt.Fprintf(os.Stderr, "DEBUG: Found match at comment ID %d via legacy text marker\n", c.ID)
			}
			return c.ID, nil
		}
	}

	return 0, nil
}

// createIssueComment creates a new comment on a PR
func createIssueComment(apiURL, owner, repo string, prNumber int, token, body string) error {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", apiURL, owner, repo, prNumber)

	reqBody := map[string]string{"body": body}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// updateIssueComment updates an existing comment on a PR
func updateIssueComment(apiURL, owner, repo string, commentID int64, token, body string) error {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d", apiURL, owner, repo, commentID)

	reqBody := map[string]string{"body": body}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("PATCH", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// getPRInfo retrieves PR information including the head SHA
func getPRInfo(apiURL, owner, repo string, prNumber int, token string) (*pullRequest, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", apiURL, owner, repo, prNumber)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var pr pullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &pr, nil
}

// listPRReviewComments retrieves all review comments on a PR
func listPRReviewComments(apiURL, owner, repo string, prNumber int, token string) ([]prComment, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/comments", apiURL, owner, repo, prNumber)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var comments []prComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return comments, nil
}

// postCodeMitigationComments posts or updates inline comments for each code mitigation
func postCodeMitigationComments(analysis *api.SecurityAnalysis, opts CommentOptions, apiURL string) (int, error) {
	// Get PR info for the commit SHA
	prInfo, err := getPRInfo(apiURL, opts.Owner, opts.Repo, opts.PRNumber, opts.Token)
	if err != nil {
		return 0, fmt.Errorf("failed to get PR info: %w", err)
	}

	// Get existing review comments
	existingComments, err := listPRReviewComments(apiURL, opts.Owner, opts.Repo, opts.PRNumber, opts.Token)
	if err != nil {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: Could not list existing review comments: %v\n", err)
		}
		existingComments = nil
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
		sanitizedPath := comment.SanitizePath(issue.Path)

		// Check if we already have a comment at this location
		existingCommentID := findExistingInlineComment(existingComments, sanitizedPath, issue.LineNumber)

		if opts.Verbose {
			if existingCommentID > 0 {
				fmt.Fprintf(os.Stderr, "Found existing inline comment at %s:%d (ID: %d)\n", issue.Path, issue.LineNumber, existingCommentID)
			} else {
				fmt.Fprintf(os.Stderr, "No existing inline comment found at %s:%d\n", issue.Path, issue.LineNumber)
			}
		}

		if existingCommentID > 0 {
			// Update existing comment
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Updating inline comment at %s:%d\n", issue.Path, issue.LineNumber)
			}
			err := updatePRReviewComment(apiURL, opts.Owner, opts.Repo, existingCommentID, opts.Token, message)
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
			err := createPRReviewComment(apiURL, opts.Owner, opts.Repo, opts.PRNumber, opts.Token, prInfo.Head.SHA, sanitizedPath, issue.LineNumber, message)
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

// findExistingInlineComment finds an existing Kusari inline comment at the given location
func findExistingInlineComment(comments []prComment, path string, line int) int64 {
	verbose := os.Getenv("KUSARI_DEBUG") == "true"

	// Regex to extract marker: <!-- KUSARI_INLINE:path:line -->
	markerRegex := regexp.MustCompile(`<!-- KUSARI_INLINE:([^:]+):(\d+) -->`)

	for _, c := range comments {
		if !strings.Contains(c.Body, "KUSARI_INLINE:") {
			continue
		}

		matches := markerRegex.FindStringSubmatch(c.Body)
		if len(matches) != 3 {
			continue
		}

		commentPath := matches[1]
		commentLine := matches[2]

		if commentPath == path && commentLine == fmt.Sprintf("%d", line) {
			if verbose {
				fmt.Fprintf(os.Stderr, "DEBUG: Found existing inline comment via marker (ID: %d)\n", c.ID)
			}
			return c.ID
		}
	}

	return 0
}

// createPRReviewComment creates a new review comment on a specific line
func createPRReviewComment(apiURL, owner, repo string, prNumber int, token, commitSHA, path string, line int, body string) error {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/comments", apiURL, owner, repo, prNumber)

	reqBody := map[string]interface{}{
		"body":      body,
		"commit_id": commitSHA,
		"path":      path,
		"line":      line,
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

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// updatePRReviewComment updates an existing review comment
func updatePRReviewComment(apiURL, owner, repo string, commentID int64, token, body string) error {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls/comments/%d", apiURL, owner, repo, commentID)

	reqBody := map[string]string{"body": body}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("PATCH", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetTokenFromEnv retrieves the GitHub token from environment variables
func GetTokenFromEnv() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GH_TOKEN")
}

// GetGitHubAPIURLFromEnv retrieves the GitHub API URL from environment
// Returns empty string if not set (will use default api.github.com)
func GetGitHubAPIURLFromEnv() string {
	if url := os.Getenv("GITHUB_API_URL"); url != "" {
		return url
	}
	return ""
}

// GetPRInfoFromEnv retrieves PR info from GitHub Actions environment variables
// Returns owner, repo, and PR number
func GetPRInfoFromEnv() (owner, repo string, prNumber int) {
	// GITHUB_REPOSITORY is in format "owner/repo"
	repository := os.Getenv("GITHUB_REPOSITORY")
	if repository != "" {
		parts := strings.SplitN(repository, "/", 2)
		if len(parts) == 2 {
			owner = parts[0]
			repo = parts[1]
		}
	}

	// For pull request events, GITHUB_REF_NAME contains the PR number
	// Format: "123/merge" for PR #123
	refName := os.Getenv("GITHUB_REF_NAME")
	if refName != "" {
		parts := strings.Split(refName, "/")
		if len(parts) > 0 {
			if num, err := strconv.Atoi(parts[0]); err == nil {
				prNumber = num
			}
		}
	}

	// Alternative: parse from GITHUB_REF which is "refs/pull/123/merge"
	if prNumber == 0 {
		ref := os.Getenv("GITHUB_REF")
		if strings.HasPrefix(ref, "refs/pull/") {
			parts := strings.Split(ref, "/")
			if len(parts) >= 3 {
				if num, err := strconv.Atoi(parts[2]); err == nil {
					prNumber = num
				}
			}
		}
	}

	return owner, repo, prNumber
}
