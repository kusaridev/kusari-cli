// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/kusaridev/kusari-cli/v2/pkg/auth"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// picoRequest is the path and query string of a request received by the fake Pico server.
type picoRequest struct {
	Path  string
	Query url.Values
}

// fakePico points the pico commands at a fake server that replies `{}` to everything, and stores a
// valid token under a temp HOME so the client can authenticate. The returned func reports the
// requests received so far.
func fakePico(t *testing.T) func() []picoRequest {
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
		mu.Lock()
		requests = append(requests, picoRequest{Path: r.URL.Path, Query: r.URL.Query()})
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
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
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	var err error
	captureStdout(t, func() { err = cmd.Execute() })
	return err
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
