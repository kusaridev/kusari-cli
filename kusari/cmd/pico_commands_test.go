// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kusaridev/kusari-cli/v2/pkg/auth"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// picoRequest is one request received by the fake Pico server.
type picoRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
}

// picoResponder decides what the fake Pico server answers; the default is 200 `{}` for everything.
type picoResponder func(r *http.Request) (status int, body string)

// fakePico points the pico commands at a fake server and stores a valid token under a temp HOME so
// the client can authenticate. The returned func reports the requests received so far.
func fakePico(t *testing.T, respond ...picoResponder) func() []picoRequest {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	require.NoError(t, auth.SaveToken(&oauth2.Token{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(time.Hour),
	}, "kusari"))

	var (
		mu       sync.Mutex
		requests []picoRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server goroutine: no require here.
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, picoRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: string(body)})
		mu.Unlock()

		status, resp := http.StatusOK, `{}`
		if len(respond) > 0 {
			status, resp = respond[0](r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(server.Close)

	orig := platformTenantEndpoint
	platformTenantEndpoint = server.URL
	t.Cleanup(func() { platformTenantEndpoint = orig })

	return func() []picoRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]picoRequest(nil), requests...)
	}
}

// runPicoCmd executes cmd with args, discarding its stdout and cobra's usage output.
func runPicoCmd(t *testing.T, cmd *cobra.Command, args ...string) error {
	t.Helper()
	_, err := runPicoCmdOutput(t, cmd, args...)
	return err
}

// runPicoCmdOutput executes cmd with args and returns what it printed to stdout, with cobra's own
// usage output discarded.
func runPicoCmdOutput(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	return out, err
}

func TestPicoCommands_FlagsToQuery(t *testing.T) {
	tests := []struct {
		name      string
		cmd       func() *cobra.Command
		args      []string
		wantPath  string
		wantQuery url.Values
	}{
		{
			name: "sboms versions",
			cmd:  sboms,
			args: []string{"versions", "42", "--page", "1", "--size", "25", "--sort", "sbom_time_asc",
				"--sbom-tag-label", "environment", "--sbom-tag-value", "prod", "--as-of", "2025-01-01T00:00:00Z"},
			wantPath: "/pico/v2/sboms/42/versions",
			wantQuery: url.Values{
				"page":           {"1"},
				"size":           {"25"},
				"sort":           {"sbom_time_asc"},
				"sbom_tag_label": {"environment"},
				"sbom_tag_value": {"prod"},
				"as_of":          {"2025-01-01T00:00:00Z"},
			},
		},
		{
			name:     "sboms tags",
			cmd:      sboms,
			args:     []string{"tags", "7", "99", "--page", "3", "--size", "10", "--label", "environment", "--active"},
			wantPath: "/pico/v2/sboms/7/versions/99/tags",
			wantQuery: url.Values{
				"page":   {"3"},
				"size":   {"10"},
				"label":  {"environment"},
				"active": {"true"},
			},
		},
		{
			name: "sboms id-by-repo",
			cmd:  sboms,
			args: []string{"id-by-repo", "--forge", "github.com", "--org", "kusaridev", "--repo", "iac",
				"--subrepo-path", "app-code/frontend-console", "--visibility", "hidden"},
			wantPath: "/pico/v2/sboms/id/by-repo",
			wantQuery: url.Values{
				"forge":        {"github.com"},
				"org":          {"kusaridev"},
				"repo":         {"iac"},
				"subrepo_path": {"app-code/frontend-console"},
				"visibility":   {"hidden"},
			},
		},
		{
			name: "components sboms",
			cmd:  components,
			args: []string{"sboms", "12", "--page", "2", "--size", "50", "--search", "frontend*",
				"--sort", "first_ingested_desc", "--visibility", "hidden", "--lifecycle-filter", "eol",
				"--sbom-tag-label", "environment", "--sbom-tag-value", "prod", "--as-of", "2025-01-01T00:00:00Z"},
			wantPath: "/pico/v2/components/12/sboms",
			wantQuery: url.Values{
				"page":             {"2"},
				"size":             {"50"},
				"search":           {"frontend*"},
				"sort":             {"first_ingested_desc"},
				"visibility":       {"hidden"},
				"lifecycle_filter": {"eol"},
				"sbom_tag_label":   {"environment"},
				"sbom_tag_value":   {"prod"},
				"as_of":            {"2025-01-01T00:00:00Z"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := fakePico(t)

			require.NoError(t, runPicoCmd(t, tt.cmd(), tt.args...))

			got := requests()
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantPath, got[0].Path)
			assert.Equal(t, tt.wantQuery, got[0].Query)
		})
	}
}

func TestPicoCommands_InvalidFlagsSendNoRequest(t *testing.T) {
	tests := []struct {
		name    string
		cmd     func() *cobra.Command
		args    []string
		wantErr string
	}{
		{
			name:    "sboms versions tag label without value",
			cmd:     sboms,
			args:    []string{"versions", "42", "--sbom-tag-label", "environment"},
			wantErr: "missing [sbom-tag-value]",
		},
		{
			name:    "sboms id-by-repo missing required flag",
			cmd:     sboms,
			args:    []string{"id-by-repo", "--forge", "github.com", "--org", "kusaridev"},
			wantErr: `required flag(s) "repo" not set`,
		},
		{
			name:    "components sboms invalid as-of",
			cmd:     components,
			args:    []string{"sboms", "12", "--as-of", "yesterday"},
			wantErr: "invalid --as-of",
		},
		{
			name:    "sboms versions non-numeric id",
			cmd:     sboms,
			args:    []string{"versions", "x"},
			wantErr: "invalid SBOM ID",
		},
		{
			name:    "sboms update-tag with no change flags",
			cmd:     sboms,
			args:    []string{"update-tag", "7", "99", "5"},
			wantErr: "at least one of the flags in the group [label value start end reopen] is required",
		},
		{
			name:    "sboms update-tag end and reopen together",
			cmd:     sboms,
			args:    []string{"update-tag", "7", "99", "5", "--end", "now", "--reopen"},
			wantErr: "none of the others can be",
		},
		{
			name:    "sboms update-tag reopen=false changes nothing",
			cmd:     sboms,
			args:    []string{"update-tag", "7", "99", "5", "--reopen=false"},
			wantErr: "no changes requested",
		},
		{
			name:    "sboms update-tag bad end timestamp",
			cmd:     sboms,
			args:    []string{"update-tag", "7", "99", "5", "--end", "tomorrow"},
			wantErr: "invalid --end",
		},
		{
			name:    "sboms id-by-identifier missing commit-sha",
			cmd:     sboms,
			args:    []string{"id-by-identifier"},
			wantErr: `required flag(s) "commit-sha" not set`,
		},
		{
			name:    "sboms move-tag empty value",
			cmd:     sboms,
			args:    []string{"move-tag", "7", "99", "environment", ""},
			wantErr: "label and value must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := fakePico(t)

			err := runPicoCmd(t, tt.cmd(), tt.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Empty(t, requests())
		})
	}
}

func TestPicoCommands_FlagsToBody(t *testing.T) {
	tests := []struct {
		name       string
		cmd        func() *cobra.Command
		args       []string
		wantMethod string
		wantPath   string
		wantBody   string // JSON, or "" for no body
	}{
		{
			name:       "sboms create-tag",
			cmd:        sboms,
			args:       []string{"create-tag", "7", "99", "environment", "prod"},
			wantMethod: http.MethodPost,
			wantPath:   "/pico/v2/sboms/7/versions/99/tags",
			wantBody:   `{"tag_label":"environment","tag_value":"prod"}`,
		},
		{
			name:       "sboms update-tag reopen sends explicit null",
			cmd:        sboms,
			args:       []string{"update-tag", "7", "99", "5", "--reopen"},
			wantMethod: http.MethodPatch,
			wantPath:   "/pico/v2/sboms/7/versions/99/tags/5",
			wantBody:   `{"end_timestamp":null}`,
		},
		{
			name:       "sboms update-tag sends only the flags given",
			cmd:        sboms,
			args:       []string{"update-tag", "7", "99", "5", "--end", "2025-01-02T00:00:00Z", "--value", "staging"},
			wantMethod: http.MethodPatch,
			wantPath:   "/pico/v2/sboms/7/versions/99/tags/5",
			wantBody:   `{"end_timestamp":"2025-01-02T00:00:00Z","tag_value":"staging"}`,
		},
		{
			name:       "sboms id-by-identifier",
			cmd:        sboms,
			args:       []string{"id-by-identifier", "--commit-sha", "abc123"},
			wantMethod: http.MethodPost,
			wantPath:   "/pico/v2/sboms/id/by-identifier",
			wantBody:   `{"commit_sha":"abc123"}`,
		},
		{
			name:       "sboms tag",
			cmd:        sboms,
			args:       []string{"tag", "7", "99", "5"},
			wantMethod: http.MethodGet,
			wantPath:   "/pico/v2/sboms/7/versions/99/tags/5",
		},
		{
			name:       "sboms delete-tag",
			cmd:        sboms,
			args:       []string{"delete-tag", "7", "99", "5"},
			wantMethod: http.MethodDelete,
			wantPath:   "/pico/v2/sboms/7/versions/99/tags/5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := fakePico(t)

			require.NoError(t, runPicoCmd(t, tt.cmd(), tt.args...))

			got := requests()
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantMethod, got[0].Method)
			assert.Equal(t, tt.wantPath, got[0].Path)
			if tt.wantBody == "" {
				assert.Empty(t, got[0].Body)
			} else {
				assert.JSONEq(t, tt.wantBody, got[0].Body)
			}
		})
	}
}

// moveTagResponder fakes the three endpoints move-tag touches: one other version (98) holds
// environment=prod via tag 12, creating on the target yields tag 15, and PATCH answers patchStatus.
func moveTagResponder(patchStatus int, holders bool) picoResponder {
	return func(r *http.Request) (int, string) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/versions"):
			if !holders {
				return http.StatusOK, `{"versions":[],"total_items":0,"total_pages":1,"current_page":0}`
			}
			return http.StatusOK, `{"versions":[{"id":98,"tags":[{"id":12,"version_id":98,"tag_label":"environment","tag_value":"prod","start_timestamp":"2026-09-01T00:00:00Z"}]}],"total_items":1,"total_pages":1,"current_page":0}`
		case r.Method == http.MethodPost:
			return http.StatusCreated, `{"id":15,"version_id":99,"tag_label":"environment","tag_value":"prod","start_timestamp":"2026-09-22T10:00:00Z"}`
		case r.Method == http.MethodPatch:
			return patchStatus, `{"error":"db down"}`
		}
		return http.StatusOK, `{}`
	}
}

func TestPicoCommands_MoveTagOutput(t *testing.T) {
	t.Run("moves and reports each tag", func(t *testing.T) {
		requests := fakePico(t, moveTagResponder(http.StatusOK, true))

		out, err := runPicoCmdOutput(t, sboms(), "move-tag", "42", "99", "environment", "prod")
		require.NoError(t, err)

		assert.Contains(t, out, "Created tag 15 (environment=prod) on SBOM 42 version 99")
		assert.Contains(t, out, "Ended tag 12 (environment=prod) on SBOM 42 version 98 at 2026-09-22T10:00:00Z")
		assert.NotContains(t, out, "No other versions carried that tag")

		got := requests()
		require.Len(t, got, 3)
		assert.Equal(t, http.MethodPatch, got[2].Method)
		assert.Equal(t, "/pico/v2/sboms/42/versions/98/tags/12", got[2].Path)
		assert.JSONEq(t, `{"end_timestamp":"2026-09-22T10:00:00Z"}`, got[2].Body)
	})

	t.Run("no other holders", func(t *testing.T) {
		fakePico(t, moveTagResponder(http.StatusOK, false))

		out, err := runPicoCmdOutput(t, sboms(), "move-tag", "42", "99", "environment", "prod")
		require.NoError(t, err)

		assert.Contains(t, out, "Created tag 15")
		assert.Contains(t, out, "No other versions carried that tag")
	})

	t.Run("failure while ending reports progress, not a false all-clear", func(t *testing.T) {
		fakePico(t, moveTagResponder(http.StatusInternalServerError, true))

		out, err := runPicoCmdOutput(t, sboms(), "move-tag", "42", "99", "environment", "prod")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ending tag #12")

		assert.Contains(t, out, "Created tag 15", "the completed step is still reported")
		assert.NotContains(t, out, "Ended tag", "nothing was ended")
		assert.NotContains(t, out, "No other versions carried that tag", "must not claim success on failure")
	})
}
