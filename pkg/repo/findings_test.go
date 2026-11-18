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
	"time"

	"github.com/kusaridev/kusari-cli/v2/api"
	"github.com/kusaridev/kusari-cli/v2/pkg/login"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workspaceAPIServer serves workspace-api's public scan routes. listResponses
// are the scans the list returns, one per poll, repeating the last.
func workspaceAPIServer(t *testing.T, analysis *api.SecurityAnalysis, listResponses ...[]api.InspectorScan) *httptest.Server {
	t.Helper()
	scan := api.InspectorScan{
		ID:     "scan-1",
		Status: "success",
		Analysis: &api.Analysis{
			Proceed:                             analysis.ShouldProceed,
			Results:                             "## Findings\n\nfull markdown body",
			TruncatedCommentWithCodeMitigations: "## Findings\n\ntruncated body\n\n[View full analysis results](https://console.us.kusari.cloud/analysis)",
			RawLLMAnalysis:                      analysis,
		},
	}
	if len(listResponses) == 0 {
		listResponses = [][]api.InspectorScan{{{ID: scan.ID, Status: scan.Status}}}
	}

	var polls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v0/inspector/scan":
			items := listResponses[min(polls, len(listResponses)-1)]
			polls++
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case "/api/v0/inspector/scan/" + scan.ID:
			_ = json.NewEncoder(w).Encode(scan)
		default:
			http.NotFound(w, r)
		}
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

func captureStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	callErr := fn()

	require.NoError(t, w.Close())
	os.Stderr = orig
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
	return captureStdout(t, func() error {
		// repoDir is empty so the cache path is skipped: this test is about the
		// verdict, not caching.
		return queryForResult(srv.URL, "https://console.example.com", "event-key", "token", "ws", format,
			false, "", false, "", "HEAD", true, failOnFindings)
	})
}

// Catches treating a scan that isn't in workspace-api yet, or is still
// processing, as finished.
func TestQueryForResult_WaitsUntilScanSucceeds(t *testing.T) {
	resultPollInterval = time.Millisecond
	t.Cleanup(func() { resultPollInterval = time.Second })

	srv := workspaceAPIServer(t, clean(),
		nil,
		[]api.InspectorScan{{ID: "scan-1", Status: "processing"}},
		[]api.InspectorScan{{ID: "scan-1", Status: "success"}},
	)

	out, err := queryOnce(t, srv, "markdown", false)

	require.NoError(t, err)
	assert.Contains(t, out, "full markdown body")
}

// Catches polling past a failed scan until the attempts run out.
func TestQueryForResult_FailedScanStops(t *testing.T) {
	srv := workspaceAPIServer(t, clean(),
		[]api.InspectorScan{{ID: "scan-1", Status: "failed", StatusDetail: "bundle unreadable"}},
	)

	stderr, err := captureStderr(t, func() error {
		_, err := queryOnce(t, srv, "markdown", false)
		return err
	})

	require.EqualError(t, err, "processing failed after uploading")
	assert.Contains(t, stderr, "Error: bundle unreadable")
}

// The whole point of the flag: a completed scan that reports findings must exit
// non-zero, and be distinguishable from a scan that broke.
func TestQueryForResult_FailOnFindings_ReportsFindings(t *testing.T) {
	out, err := queryOnce(t, workspaceAPIServer(t, withFindings()), "markdown", true)

	require.Error(t, err)
	var fe *FindingsError
	require.ErrorAs(t, err, &fe, "callers must be able to tell findings from a failure")
	assert.Equal(t, 2, fe.CodeMitigations)
	assert.Equal(t, 1, fe.DependencyMitigations)
	assert.Equal(t, 3, fe.ExitCode(), "exit 3 keeps findings distinct from a generic failure")

	// The ordering guarantee: results are printed before the verdict is
	// returned. A hook or CI job that only gets an exit code has nothing to act
	// on, which would defeat the purpose of the flag.
	assert.NotEmpty(t, out, "results must still be written when reporting findings")
	assert.Contains(t, out, "Findings", "the report body must reach stdout")
}

// Without the flag, behavior is exactly as before: findings do not fail.
func TestQueryForResult_WithoutFlag_FindingsDoNotFail(t *testing.T) {
	out, err := queryOnce(t, workspaceAPIServer(t, withFindings()), "markdown", false)

	require.NoError(t, err, "the flag is opt-in; default behavior must not change")
	assert.NotEmpty(t, out)
}

func TestQueryForResult_FailOnFindings_CleanScanSucceeds(t *testing.T) {
	_, err := queryOnce(t, workspaceAPIServer(t, clean()), "markdown", true)
	assert.NoError(t, err, "a clean scan must exit zero even with the flag set")
}

// SARIF takes a different return path through queryForResult, so it needs its
// own coverage: the verdict must be reported there too, and the SARIF document
// must still be complete on stdout.
func TestQueryForResult_FailOnFindings_SarifStillEmitted(t *testing.T) {
	out, err := queryOnce(t, workspaceAPIServer(t, withFindings()), "sarif", true)

	var fe *FindingsError
	require.ErrorAs(t, err, &fe)

	assert.True(t, strings.Contains(out, `"should_proceed": false`),
		"the SARIF document must be written in full before the verdict is returned")
	assert.Contains(t, out, "code-mitigation")
	assert.Contains(t, out, "src/config.js")
	assert.Contains(t, out, "https://console.example.com/workspaces/ws/analysis/scan-1/result",
		"catches linking by the event key or the old sort key instead of the scan id")
}

func TestFindingsResult(t *testing.T) {
	t.Run("nil analysis never reports findings", func(t *testing.T) {
		assert.NoError(t, findingsResult(true, nil))
	})
	t.Run("flag off never reports findings", func(t *testing.T) {
		assert.NoError(t, findingsResult(false, withFindings()))
	})
	t.Run("should proceed reports nothing", func(t *testing.T) {
		assert.NoError(t, findingsResult(true, clean()))
	})
	t.Run("findings report an error", func(t *testing.T) {
		err := findingsResult(true, withFindings())
		require.Error(t, err)
		assert.Equal(t, "Kusari Inspector reported 2 code and 1 dependency finding(s) that must be addressed", err.Error())
	})
}

// End to end through scan(): a live gated scan populates the cache, and the
// next identical scan is served from cache and still reaches the same verdict.
//
// This is the behavior the gate depends on. Serving the cached report and
// returning success would silently pass a commit or a CI job that the analysis
// said not to proceed with, and it would happen precisely on a re-run of an
// unchanged diff.
func TestScan_GatedScanIsServedFromCacheAndStillBlocks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	srv := workspaceAPIServer(t, withFindings())

	repoDir := t.TempDir()
	setupGitRepoAt(t, repoDir)

	var uploads int
	newMock := func() *scanMock {
		return &scanMock{
			fileUploader: func(presignedURL, filePath string) error { uploads++; return nil },
			presignedURLGetter: func(apiEndpoint, jwtToken, filePath, workspace string, full bool, size int64) (string, error) {
				return "https://example.com/workspace/ws/user/human/u/diff/blob/123", nil
			},
			defaultWorkspaceGetter: func(platformUrl, jwtToken string) ([]login.Workspace, map[string][]string, error) {
				return []login.Workspace{{ID: "ws", Description: "Test Workspace"}},
					map[string][]string{"ws": {"test-tenant"}}, nil
			},
			token: "token",
		}
	}

	opts := ScanOptions{
		Dir:            repoDir,
		Rev:            "HEAD",
		PlatformURL:    srv.URL,
		ConsoleURL:     "https://console.example.com",
		OutputFormat:   "sarif",
		Wait:           true,
		FullOutput:     true,
		FailOnFindings: true,
	}

	// First run: a real scan. It must report the findings.
	_, err := captureStdout(t, func() error { return scan(opts, false, newMock()) })
	require.Error(t, err, "a blocking verdict must fail a gated scan")
	var first *FindingsError
	require.ErrorAs(t, err, &first)
	require.Equal(t, 1, uploads, "the first run should actually scan")

	// Second run: identical diff, so it is served from cache.
	out, err := captureStdout(t, func() error { return scan(opts, false, newMock()) })
	require.Error(t, err, "a cached blocking verdict must still fail a gated scan")
	var second *FindingsError
	require.ErrorAs(t, err, &second)

	assert.Equal(t, 1, uploads, "the second run must be served from cache, not re-uploaded")
	assert.Equal(t, first.CodeMitigations, second.CodeMitigations, "the cached verdict must match the live one")
	assert.Equal(t, first.DependencyMitigations, second.DependencyMitigations)
	assert.Equal(t, first.ExitCode(), second.ExitCode())
	assert.NotEmpty(t, out, "cached results must still be printed")
}

// The same cache entry must not fail an ungated scan: the flag decides, not the
// stored verdict.
func TestScan_CachedVerdictDoesNotAffectUngatedScan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	srv := workspaceAPIServer(t, withFindings())
	repoDir := t.TempDir()
	setupGitRepoAt(t, repoDir)

	mock := func() *scanMock {
		return &scanMock{
			fileUploader: func(presignedURL, filePath string) error { return nil },
			presignedURLGetter: func(apiEndpoint, jwtToken, filePath, workspace string, full bool, size int64) (string, error) {
				return "https://example.com/workspace/ws/user/human/u/diff/blob/123", nil
			},
			defaultWorkspaceGetter: func(platformUrl, jwtToken string) ([]login.Workspace, map[string][]string, error) {
				return []login.Workspace{{ID: "ws", Description: "Test Workspace"}},
					map[string][]string{"ws": {"test-tenant"}}, nil
			},
			token: "token",
		}
	}

	base := ScanOptions{
		Dir: repoDir, Rev: "HEAD", PlatformURL: srv.URL,
		ConsoleURL: "https://console.example.com", OutputFormat: "sarif",
		Wait: true, FullOutput: true,
	}

	gated := base
	gated.FailOnFindings = true
	_, err := captureStdout(t, func() error { return scan(gated, false, mock()) })
	require.Error(t, err)

	// Same repo, same diff, same cached entry -- but not gated.
	_, err = captureStdout(t, func() error { return scan(base, false, mock()) })
	assert.NoError(t, err, "without --fail-on-findings a cached blocking verdict must not fail the scan")
}

// Catches a console link left on stderr or in the rendered results, live or on a
// cache hit, and a cache that serves truncated text to a --full-output rerun.
func TestScan_TruncatedResultsHintAtFullOutputAndPrintNoLink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	srv := workspaceAPIServer(t, clean())
	repoDir := t.TempDir()
	setupGitRepoAt(t, repoDir)

	var uploads int
	mock := &scanMock{
		fileUploader: func(presignedURL, filePath string) error { uploads++; return nil },
		presignedURLGetter: func(apiEndpoint, jwtToken, filePath, workspace string, full bool, size int64) (string, error) {
			return "https://example.com/workspace/ws/user/human/u/diff/blob/123", nil
		},
		defaultWorkspaceGetter: func(platformUrl, jwtToken string) ([]login.Workspace, map[string][]string, error) {
			return []login.Workspace{{ID: "ws", Description: "Test Workspace"}},
				map[string][]string{"ws": {"test-tenant"}}, nil
		},
		token: "token",
	}
	opts := ScanOptions{
		Dir: repoDir, Rev: "HEAD", PlatformURL: srv.URL,
		ConsoleURL: "https://console.example.com", OutputFormat: "markdown", Wait: true,
	}
	run := func() (stdout, stderr string) {
		stderr, err := captureStderr(t, func() error {
			var err error
			stdout, err = captureStdout(t, func() error { return scan(opts, false, mock) })
			return err
		})
		require.NoError(t, err)
		return stdout, stderr
	}

	for _, label := range []string{"live", "cache hit"} {
		stdout, stderr := run()
		assert.Contains(t, stdout, "--full-output", label)
		assert.NotContains(t, stdout+stderr, "https://console", label)
	}
	require.Equal(t, 1, uploads, "the second run should be served from cache")

	opts.FullOutput = true
	stdout, _ := run()
	assert.Equal(t, 2, uploads, "a --full-output rerun must not be served the truncated entry")
	assert.Contains(t, stdout, "full markdown body")
}
