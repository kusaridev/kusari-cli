// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kusaridev/kusari-cli/pkg/auth"
)

// Client handles HTTP requests to the Kusari Pico API.
type Client struct {
	tenant     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Pico API client.
// Tenant is required - it will be used to construct the API base URL.
func NewClient(tenant string) *Client {
	// Always use US environment for production
	baseURL := fmt.Sprintf("https://%s.api.us.kusari.cloud", tenant)

	return &Client{
		tenant:  tenant,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// makeRequest makes an HTTP request to the Pico API with authentication.
func (c *Client) makeRequest(ctx context.Context, method, path string, params map[string]string, body interface{}) ([]byte, error) {
	// Load access token
	token, err := auth.LoadToken("kusari")
	if err != nil {
		return nil, fmt.Errorf("failed to load auth token: %w", err)
	}

	// Check token expiry
	if err := auth.CheckTokenExpiry(token); err != nil {
		return nil, fmt.Errorf("token expired: %w", err)
	}

	// Build URL
	reqURL := c.baseURL + path
	if len(params) > 0 {
		values := url.Values{}
		for k, v := range params {
			if v != "" {
				values.Add(k, v)
			}
		}
		if len(values) > 0 {
			reqURL += "?" + values.Encode()
		}
	}

	// Prepare request body
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetVulnerabilities retrieves vulnerabilities with optional filters.
func (c *Client) GetVulnerabilities(ctx context.Context, search string, kusariScore string, page, size int) (json.RawMessage, error) {
	params := make(map[string]string)
	if search != "" {
		params["search"] = search
	}
	if kusariScore != "" {
		params["kusari_score"] = kusariScore
	}
	if page >= 0 {
		params["page"] = fmt.Sprintf("%d", page)
	}
	if size > 0 {
		params["size"] = fmt.Sprintf("%d", size)
	}

	respBody, err := c.makeRequest(ctx, "GET", "/pico/v1/vulnerabilities", params, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(respBody), nil
}

// GetVulnerabilityByExternalID retrieves vulnerability details by external ID (CVE, GHSA, etc.).
func (c *Client) GetVulnerabilityByExternalID(ctx context.Context, externalID string) (json.RawMessage, error) {
	path := fmt.Sprintf("/pico/v1/vulnerabilities/by-external-id/%s", url.PathEscape(externalID))
	respBody, err := c.makeRequest(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(respBody), nil
}

// SearchPackages searches for packages by name.
func (c *Client) SearchPackages(ctx context.Context, name, version string) (json.RawMessage, error) {
	path := fmt.Sprintf("/pico/v1/packages/search/%s", url.PathEscape(name))
	params := make(map[string]string)
	if version != "" {
		params["version"] = version
	}

	respBody, err := c.makeRequest(ctx, "GET", path, params, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(respBody), nil
}

// GetSoftwareList retrieves a list of software with optional search filter.
func (c *Client) GetSoftwareList(ctx context.Context, search string, page, size int) (json.RawMessage, error) {
	params := make(map[string]string)
	if search != "" {
		params["search"] = search
	}
	if page >= 0 {
		params["page"] = fmt.Sprintf("%d", page)
	}
	if size > 0 {
		params["size"] = fmt.Sprintf("%d", size)
	}

	respBody, err := c.makeRequest(ctx, "GET", "/pico/v1/software", params, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(respBody), nil
}

// GetSoftwareByID retrieves detailed information about a specific software by ID.
func (c *Client) GetSoftwareByID(ctx context.Context, softwareID int) (json.RawMessage, error) {
	path := fmt.Sprintf("/pico/v1/software/%d", softwareID)
	respBody, err := c.makeRequest(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(respBody), nil
}

// GetStats retrieves aggregate vulnerability statistics.
func (c *Client) GetStats(ctx context.Context) (json.RawMessage, error) {
	respBody, err := c.makeRequest(ctx, "GET", "/pico/v1/stats", nil, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(respBody), nil
}

// GetPackagesWithLifecycle retrieves packages filtered by lifecycle status.
func (c *Client) GetPackagesWithLifecycle(ctx context.Context, params map[string]string) (json.RawMessage, error) {
	respBody, err := c.makeRequest(ctx, "GET", "/pico/v1/packages/lifecycle", params, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(respBody), nil
}

// GetSoftwareIDsByRepo finds software IDs by repository metadata (forge, org, repo, subrepo_path).
func (c *Client) GetSoftwareIDsByRepo(ctx context.Context, forge, org, repo, subrepoPath string) (json.RawMessage, error) {
	params := map[string]string{
		"forge":        forge,
		"org":          org,
		"repo":         repo,
		"subrepo_path": subrepoPath,
	}

	respBody, err := c.makeRequest(ctx, "GET", "/pico/v1/software/id/by-repo", params, nil)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(respBody), nil
}
