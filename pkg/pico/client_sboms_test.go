// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/kusaridev/kusari-cli/v2/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// recordedRequest captures what the client sent so tests can assert on it.
type recordedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   []byte
	Auth   string
}

// setupTestAuth points HOME at a temp dir and stores a valid token there so makeRequest can authenticate.
func setupTestAuth(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	require.NoError(t, auth.SaveToken(&oauth2.Token{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(time.Hour),
	}, "kusari"))
}

// recordingServer returns a server that records the last request and replies with statusCode and response.
func recordingServer(t *testing.T, statusCode int, response any) (*httptest.Server, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		*rec = recordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.Query(),
			Body:   body,
			Auth:   r.Header.Get("Authorization"),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if response != nil {
			data, err := json.Marshal(response)
			require.NoError(t, err)
			_, _ = w.Write(data)
		}
	}))
	t.Cleanup(server.Close)
	return server, rec
}

func TestAddPaginationParams(t *testing.T) {
	tests := []struct {
		name string
		page int
		size int
		want map[string]string
	}{
		{"both set", 2, 50, map[string]string{"page": "2", "size": "50"}},
		{"page zero is sent", 0, 10, map[string]string{"page": "0", "size": "10"}},
		{"size zero omitted", 1, 0, map[string]string{"page": "1"}},
		{"negative page omitted", -1, 10, map[string]string{"size": "10"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"existing": "kept"}
			AddPaginationParams(params, tt.page, tt.size)

			want := map[string]string{"existing": "kept"}
			for k, v := range tt.want {
				want[k] = v
			}
			assert.Equal(t, want, params)
		})
	}
}

func TestClient_GetSbomVersions(t *testing.T) {
	setupTestAuth(t)
	server, rec := recordingServer(t, http.StatusOK, map[string]any{"items": []any{}})
	client := NewClient(server.URL)

	t.Run("all params", func(t *testing.T) {
		result, err := client.GetSbomVersions(context.Background(), 42, 1, 25, "sbom_time_asc", "environment", "prod", "2025-01-01T00:00:00Z")
		require.NoError(t, err)
		assert.JSONEq(t, `{"items":[]}`, string(result))

		assert.Equal(t, http.MethodGet, rec.Method)
		assert.Equal(t, "/pico/v2/sboms/42/versions", rec.Path)
		assert.Equal(t, "Bearer test-token", rec.Auth)
		assert.Equal(t, url.Values{
			"page":           {"1"},
			"size":           {"25"},
			"sort":           {"sbom_time_asc"},
			"sbom_tag_label": {"environment"},
			"sbom_tag_value": {"prod"},
			"as_of":          {"2025-01-01T00:00:00Z"},
		}, rec.Query)
	})

	t.Run("optional params omitted when empty", func(t *testing.T) {
		_, err := client.GetSbomVersions(context.Background(), 42, 0, 1000, "", "", "", "")
		require.NoError(t, err)

		assert.Equal(t, url.Values{"page": {"0"}, "size": {"1000"}}, rec.Query)
	})
}

func TestClient_ListSbomVersionTags(t *testing.T) {
	setupTestAuth(t)
	server, rec := recordingServer(t, http.StatusOK, map[string]any{"items": []any{}})
	client := NewClient(server.URL)

	t.Run("with filters", func(t *testing.T) {
		_, err := client.ListSbomVersionTags(context.Background(), 7, 99, 0, 100, "environment", true)
		require.NoError(t, err)

		assert.Equal(t, http.MethodGet, rec.Method)
		assert.Equal(t, "/pico/v2/sboms/7/versions/99/tags", rec.Path)
		assert.Equal(t, url.Values{
			"page":   {"0"},
			"size":   {"100"},
			"label":  {"environment"},
			"active": {"true"},
		}, rec.Query)
	})

	t.Run("active false and empty label omitted", func(t *testing.T) {
		_, err := client.ListSbomVersionTags(context.Background(), 7, 99, 0, 100, "", false)
		require.NoError(t, err)

		assert.Equal(t, url.Values{"page": {"0"}, "size": {"100"}}, rec.Query)
	})
}

func TestClient_CreateSbomVersionTag(t *testing.T) {
	setupTestAuth(t)
	server, rec := recordingServer(t, http.StatusCreated, map[string]any{"id": 5})
	client := NewClient(server.URL)

	result, err := client.CreateSbomVersionTag(context.Background(), 7, 99, "Environment", "Production")
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":5}`, string(result))

	assert.Equal(t, http.MethodPost, rec.Method)
	assert.Equal(t, "/pico/v2/sboms/7/versions/99/tags", rec.Path)
	assert.Empty(t, rec.Query)
	// Body is sent as-is; the server is responsible for lowercasing.
	assert.JSONEq(t, `{"tag_label":"Environment","tag_value":"Production"}`, string(rec.Body))
}

func TestClient_GetSbomVersionTag(t *testing.T) {
	setupTestAuth(t)
	server, rec := recordingServer(t, http.StatusOK, map[string]any{"id": 5})
	client := NewClient(server.URL)

	result, err := client.GetSbomVersionTag(context.Background(), 7, 99, 5)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":5}`, string(result))

	assert.Equal(t, http.MethodGet, rec.Method)
	assert.Equal(t, "/pico/v2/sboms/7/versions/99/tags/5", rec.Path)
	assert.Empty(t, rec.Query)
	assert.Empty(t, rec.Body)
}

func TestClient_UpdateSbomVersionTag(t *testing.T) {
	setupTestAuth(t)
	server, rec := recordingServer(t, http.StatusOK, map[string]any{"id": 5})
	client := NewClient(server.URL)

	t.Run("close tag with end_timestamp", func(t *testing.T) {
		body := map[string]any{"end_timestamp": "2025-01-02T00:00:00Z"}
		result, err := client.UpdateSbomVersionTag(context.Background(), 7, 99, 5, body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"id":5}`, string(result))

		assert.Equal(t, http.MethodPatch, rec.Method)
		assert.Equal(t, "/pico/v2/sboms/7/versions/99/tags/5", rec.Path)
		assert.JSONEq(t, `{"end_timestamp":"2025-01-02T00:00:00Z"}`, string(rec.Body))
	})

	t.Run("reopen sends explicit null", func(t *testing.T) {
		body := map[string]any{"end_timestamp": nil}
		_, err := client.UpdateSbomVersionTag(context.Background(), 7, 99, 5, body)
		require.NoError(t, err)

		// null must be present in the body, not omitted, since omitting means "leave unchanged".
		assert.JSONEq(t, `{"end_timestamp":null}`, string(rec.Body))
	})

	t.Run("only provided fields are sent", func(t *testing.T) {
		body := map[string]any{"tag_value": "staging"}
		_, err := client.UpdateSbomVersionTag(context.Background(), 7, 99, 5, body)
		require.NoError(t, err)

		assert.JSONEq(t, `{"tag_value":"staging"}`, string(rec.Body))
	})
}

func TestClient_DeleteSbomVersionTag(t *testing.T) {
	setupTestAuth(t)
	server, rec := recordingServer(t, http.StatusNoContent, nil)
	client := NewClient(server.URL)

	err := client.DeleteSbomVersionTag(context.Background(), 7, 99, 5)
	require.NoError(t, err)

	assert.Equal(t, http.MethodDelete, rec.Method)
	assert.Equal(t, "/pico/v2/sboms/7/versions/99/tags/5", rec.Path)
	assert.Empty(t, rec.Body)
}

func TestClient_FindSbomIDsByIdentifier(t *testing.T) {
	setupTestAuth(t)
	response := []map[string]any{{"sbom_id": 7, "version_id": 99}}
	server, rec := recordingServer(t, http.StatusOK, response)
	client := NewClient(server.URL)

	result, err := client.FindSbomIDsByIdentifier(context.Background(), "abc123")
	require.NoError(t, err)
	assert.JSONEq(t, `[{"sbom_id":7,"version_id":99}]`, string(result))

	assert.Equal(t, http.MethodPost, rec.Method)
	assert.Equal(t, "/pico/v2/sboms/id/by-identifier", rec.Path)
	assert.Empty(t, rec.Query)
	assert.JSONEq(t, `{"commit_sha":"abc123"}`, string(rec.Body))
}

func TestClient_SbomMethods_ErrorResponses(t *testing.T) {
	setupTestAuth(t)

	tests := []struct {
		name       string
		statusCode int
		call       func(c *Client) error
	}{
		{"versions 404", http.StatusNotFound, func(c *Client) error {
			_, err := c.GetSbomVersions(context.Background(), 1, 0, 10, "", "", "", "")
			return err
		}},
		{"create tag 409 conflict", http.StatusConflict, func(c *Client) error {
			_, err := c.CreateSbomVersionTag(context.Background(), 1, 2, "environment", "prod")
			return err
		}},
		{"update tag 400", http.StatusBadRequest, func(c *Client) error {
			_, err := c.UpdateSbomVersionTag(context.Background(), 1, 2, 3, map[string]any{"end_timestamp": "bogus"})
			return err
		}},
		{"delete tag 404", http.StatusNotFound, func(c *Client) error {
			return c.DeleteSbomVersionTag(context.Background(), 1, 2, 3)
		}},
		{"by-identifier 404", http.StatusNotFound, func(c *Client) error {
			_, err := c.FindSbomIDsByIdentifier(context.Background(), "nope")
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := recordingServer(t, tt.statusCode, map[string]string{"error": "boom"})
			client := NewClient(server.URL)

			err := tt.call(client)
			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("status %d", tt.statusCode))
			assert.Contains(t, err.Error(), "boom")
		})
	}
}
