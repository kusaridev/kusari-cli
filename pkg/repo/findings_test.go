// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kusaridev/kusari-cli/v2/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resultServer stands in for the platform's result endpoint. Until now no test
// reached queryForResult at all: every scan test passes Wait=false, so the
// whole result-rendering path was unexercised.
func resultServer(t *testing.T, analysis *api.SecurityAnalysis) *httptest.Server {
	t.Helper()
	payload := []api.UserInspectorResult{{
		Analysis: &api.Analysis{
			Proceed:                             analysis.ShouldProceed,
			Results:                             "## Findings\n\nfull markdown body",
			TruncatedCommentWithCodeMitigations: "## Findings\n\ntruncated body",
			RawLLMAnalysis:                      analysis,
		},
		StatusMeta: api.StatusMeta{Status: "complete", UpdatedAt: "2026-01-01T00:00:00Z"},
	}}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// captureStdout collects what a function prints, so a test can assert that
// results were emitted even when the call reports findings.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	callErr := fn()

	require.NoError(t, w.Close())
	os.Stdout = orig
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out), callErr
}

func withFindings() *api.SecurityAnalysis {
	return &api.SecurityAnalysis{
		ShouldProceed: false,
		HealthScore:   2,
		Justification: "hardcoded credential and a vulnerable dependency",
		RequiredCodeMitigations: []api.CodeMitigationItem{
			{Path: "src/config.js", LineNumber: 18, Content: "hardcoded AWS key"},
			{Path: "src/routes/users.js", LineNumber: 27, Content: "SQL injection"},
		},
		RequiredDependencyMitigations: []api.DependencyMitigationItem{
			{Content: "lodash 4.17.15 is vulnerable to prototype pollution"},
		},
	}
}

func clean() *api.SecurityAnalysis {
	return &api.SecurityAnalysis{ShouldProceed: true, HealthScore: 5}
}

func queryOnce(t *testing.T, srv *httptest.Server, format string, failOnFindings bool) (string, error) {
	t.Helper()
	// Sandbox HOME so a stray cache write cannot touch the developer's files.
	t.Setenv("HOME", t.TempDir())
	consoleURL := "https://console.example.com/results/abc"
	return captureStdout(t, func() error {
		// repoDir is empty so the cache path is skipped: this test is about the
		// verdict, not caching.
		return queryForResult(srv.URL, "sort-key", "token", &consoleURL, "ws", format,
			false, "", false, "", "HEAD", true, failOnFindings)
	})
}

// The whole point of the flag: a completed scan that reports findings must exit
// non-zero, and be distinguishable from a scan that broke.
func TestQueryForResult_FailOnFindings_ReportsFindings(t *testing.T) {
	out, err := queryOnce(t, resultServer(t, withFindings()), "markdown", true)

	require.Error(t, err)
	var fe *FindingsError
	require.ErrorAs(t, err, &fe, "callers must be able to tell findings from a failure")
	assert.Equal(t, 2, fe.CodeMitigations)
	assert.Equal(t, 1, fe.DependencyMitigations)
	assert.Equal(t, "https://console.example.com/results/abc", fe.ConsoleURL)
	assert.Equal(t, 3, fe.ExitCode(), "exit 3 keeps findings distinct from a generic failure")

	// The ordering guarantee: results are printed before the verdict is
	// returned. A hook or CI job that only gets an exit code has nothing to act
	// on, which would defeat the purpose of the flag.
	assert.NotEmpty(t, out, "results must still be written when reporting findings")
	assert.Contains(t, out, "Findings", "the report body must reach stdout")
}

// Without the flag, behavior is exactly as before: findings do not fail.
func TestQueryForResult_WithoutFlag_FindingsDoNotFail(t *testing.T) {
	out, err := queryOnce(t, resultServer(t, withFindings()), "markdown", false)

	require.NoError(t, err, "the flag is opt-in; default behavior must not change")
	assert.NotEmpty(t, out)
}

func TestQueryForResult_FailOnFindings_CleanScanSucceeds(t *testing.T) {
	_, err := queryOnce(t, resultServer(t, clean()), "markdown", true)
	assert.NoError(t, err, "a clean scan must exit zero even with the flag set")
}

// SARIF takes a different return path through queryForResult, so it needs its
// own coverage: the verdict must be reported there too, and the SARIF document
// must still be complete on stdout.
func TestQueryForResult_FailOnFindings_SarifStillEmitted(t *testing.T) {
	out, err := queryOnce(t, resultServer(t, withFindings()), "sarif", true)

	var fe *FindingsError
	require.ErrorAs(t, err, &fe)

	assert.True(t, strings.Contains(out, `"should_proceed": false`),
		"the SARIF document must be written in full before the verdict is returned")
	assert.Contains(t, out, "code-mitigation")
	assert.Contains(t, out, "src/config.js")
}

func TestFindingsResult(t *testing.T) {
	t.Run("nil analysis never reports findings", func(t *testing.T) {
		assert.NoError(t, findingsResult(true, nil, "url"))
	})
	t.Run("flag off never reports findings", func(t *testing.T) {
		assert.NoError(t, findingsResult(false, withFindings(), "url"))
	})
	t.Run("should proceed reports nothing", func(t *testing.T) {
		assert.NoError(t, findingsResult(true, clean(), "url"))
	})
	t.Run("findings report an error", func(t *testing.T) {
		err := findingsResult(true, withFindings(), "url")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "2 code and 1 dependency")
	})
}
