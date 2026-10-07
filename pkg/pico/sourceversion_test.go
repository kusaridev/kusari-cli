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
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCommitTime has a non-UTC offset so that any reformatting before it reaches as_of shows up.
const testCommitTime = "2026-10-01T14:34:56+02:00"

type sourceFakeRequest struct {
	Method string
	Path   string
	Query  url.Values
}

// fakeSourceServer stands in for the four lookups FindSourceSbomVersions makes:
//
//   - Component 7 has SBOM 13 (source), SBOM 20 (image) and, on page 2, SBOM 14 (source).
//   - Component 8 has only SBOM 21 (image).
//   - Component 9 has SBOMs 14, 15 and 16 (all source).
//   - by-identifier for commit aaa returns SBOM 20 version 900 and SBOM 13 version 4821.
//   - versions as_of testCommitTime: SBOM 13 -> 4821, SBOM 14 -> 4790 (any other as_of -> 4800),
//     SBOM 15 -> empty list, SBOM 16 -> 404.
//   - by-repo for github.com/kusaridev/iac returns source SBOM 13 at app-code/frontend-console,
//     source SBOM 30 at the repo root, and build SBOM 31 at app-code/frontend-console.
//
// Any other request fails the test, which is how "no request was made" is checked.
type fakeSourceServer struct {
	t *testing.T

	mu                 sync.Mutex
	componentPages     map[int][][]map[string]any
	byIdentifier       []map[string]any
	byIdentifierStatus int    // overrides the by-identifier status when non-zero
	sbom14CommitSha    string // commit recorded on SBOM 14 version 4790
	requests           []sourceFakeRequest
}

func newFakeSourceServer(t *testing.T) (*fakeSourceServer, *Client) {
	t.Helper()
	setupTestAuth(t)
	sbom := func(id int, name, typ string) map[string]any {
		return map[string]any{"id": id, "name": name, "sbom_type": typ}
	}
	f := &fakeSourceServer{
		t: t,
		componentPages: map[int][][]map[string]any{
			7: {
				{sbom(13, "kusaridev/iac/app-code/frontend-console", "source"), sbom(20, "frontend-console-image", "image")},
				{sbom(14, "kusaridev/iac/app-code/shared", "source")},
			},
			8: {{sbom(21, "other-image", "image")}},
			9: {{
				sbom(14, "kusaridev/iac/app-code/shared", "source"),
				sbom(15, "kusaridev/iac/app-code/new", "source"),
				sbom(16, "kusaridev/iac/app-code/newer", "source"),
			}},
		},
		byIdentifier: []map[string]any{
			{"sbom_id": 20, "version_id": 900},
			{"sbom_id": 13, "version_id": 4821},
		},
		sbom14CommitSha: "bbb",
	}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, NewClient(srv.URL)
}

func (f *fakeSourceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	body, _ := io.ReadAll(r.Body)
	f.requests = append(f.requests, sourceFakeRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query()})
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query()

	// Server goroutine: report with Errorf, never FailNow.
	unexpected := func() {
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
		w.WriteHeader(http.StatusInternalServerError)
	}
	reply := func(status int, v any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}

	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/pico/v2/components/") && strings.HasSuffix(r.URL.Path, "/sboms"):
		id, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/pico/v2/components/"), "/sboms"))
		pages, ok := f.componentPages[id]
		if !ok {
			unexpected()
			return
		}
		page, _ := strconv.Atoi(q.Get("page"))
		if page >= len(pages) {
			reply(http.StatusOK, map[string]any{"sboms": []any{}, "total_pages": len(pages), "current_page": page})
			return
		}
		reply(http.StatusOK, map[string]any{"sboms": pages[page], "total_pages": len(pages), "current_page": page})

	case r.Method == http.MethodGet && r.URL.Path == "/pico/v2/sboms/id/by-repo":
		if q.Get("forge") != "github.com" || q.Get("org") != "kusaridev" || q.Get("repo") != "iac" || q.Has("subrepo_path") {
			unexpected()
			return
		}
		reply(http.StatusOK, []map[string]any{
			{"sbom_id": 13, "name": "kusaridev/iac/app-code/frontend-console", "subrepo_path": "app-code/frontend-console", "type": "source"},
			{"sbom_id": 30, "name": "kusaridev/iac", "subrepo_path": ".", "type": "source"},
			{"sbom_id": 31, "name": "frontend-console-build", "subrepo_path": "app-code/frontend-console", "type": "build"},
		})

	case r.Method == http.MethodPost && r.URL.Path == "/pico/v2/sboms/id/by-identifier":
		var req struct {
			CommitSha string `json:"commit_sha"`
		}
		if err := json.Unmarshal(body, &req); err != nil || req.CommitSha != "aaa" {
			unexpected()
			return
		}
		if f.byIdentifierStatus != 0 {
			reply(f.byIdentifierStatus, map[string]string{"error": "from fake"})
			return
		}
		reply(http.StatusOK, f.byIdentifier)

	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/pico/v2/sboms/") && strings.HasSuffix(r.URL.Path, "/versions"):
		id, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/pico/v2/sboms/"), "/versions"))
		version := func(vid int, sha string) map[string]any {
			return map[string]any{"versions": []any{map[string]any{"id": vid, "sbom_id": id, "commit_sha": sha}}, "total_pages": 1}
		}
		switch id {
		case 13:
			reply(http.StatusOK, version(4821, "aaa"))
		case 14:
			if q.Get("as_of") == testCommitTime {
				reply(http.StatusOK, version(4790, f.sbom14CommitSha))
			} else {
				reply(http.StatusOK, version(4800, "ccc"))
			}
		case 15:
			reply(http.StatusOK, map[string]any{"versions": []any{}, "total_pages": 0})
		case 16:
			reply(http.StatusNotFound, map[string]string{"error": "SBOM not found"})
		default:
			unexpected()
		}

	default:
		unexpected()
	}
}

// requestsTo returns the recorded requests whose path is exactly path.
func (f *fakeSourceServer) requestsTo(path string) []sourceFakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []sourceFakeRequest
	for _, r := range f.requests {
		if r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

// findSource runs FindSourceSbomVersions for commit aaa at testCommitTime, plus whatever opts sets,
// and returns the result and what was logged.
func findSource(t *testing.T, c *Client, opts FindSourceSbomVersionsOptions) (*SourceVersionResult, string, error) {
	t.Helper()
	var log bytes.Buffer
	opts.CommitSha = "aaa"
	opts.CommitTime = testCommitTime
	opts.Log = &log
	res, err := c.FindSourceSbomVersions(context.Background(), opts)
	return res, log.String(), err
}

func iacRepo(subrepoPath string) FindSourceSbomVersionsOptions {
	return FindSourceSbomVersionsOptions{Forge: "github.com", Org: "kusaridev", Repo: "iac", SubrepoPath: subrepoPath}
}

var (
	match13ByCommit = SourceVersionMatch{
		SbomID: 13, VersionID: 4821, Name: "kusaridev/iac/app-code/frontend-console",
		MatchedBy: MatchedByComponent, VersionFoundBy: VersionFoundByCommit, CommitSha: "aaa",
	}
	match14BeforeCommit = SourceVersionMatch{
		SbomID: 14, VersionID: 4790, Name: "kusaridev/iac/app-code/shared",
		MatchedBy: MatchedByComponent, VersionFoundBy: VersionFoundByNewestBeforeCommit, CommitSha: "bbb",
	}
)

func TestFindSourceSbomVersions_ComponentVersionAtAndBeforeCommit(t *testing.T) {
	f, c := newFakeSourceServer(t)

	res, log, err := findSource(t, c, FindSourceSbomVersionsOptions{ComponentID: 7})
	require.NoError(t, err)

	assert.Equal(t, []SourceVersionMatch{match13ByCommit, match14BeforeCommit}, res.Matches, "SBOM 20 is an image SBOM and must not be matched")
	assert.Empty(t, res.Unmatched)
	assert.Empty(t, f.requestsTo("/pico/v2/sboms/13/versions"), "SBOM 13 has a version at the commit, so no as_of lookup")
	assert.Contains(t, log, "no version at commit aaa; using version 4790")
}

func TestFindSourceSbomVersions_AsOfIsCommitTimeUnchanged(t *testing.T) {
	f, c := newFakeSourceServer(t)

	res, _, err := findSource(t, c, FindSourceSbomVersionsOptions{ComponentID: 7})
	require.NoError(t, err)

	got := f.requestsTo("/pico/v2/sboms/14/versions")
	require.Len(t, got, 1)
	assert.Equal(t, testCommitTime, got[0].Query.Get("as_of"))
	assert.Equal(t, "1", got[0].Query.Get("size"))
	assert.Contains(t, res.Matches, match14BeforeCommit, "4790 is the version at the commit time, 4800 is not")
}

func TestFindSourceSbomVersions_ReadsEveryComponentPage(t *testing.T) {
	f, c := newFakeSourceServer(t)

	res, _, err := findSource(t, c, FindSourceSbomVersionsOptions{ComponentID: 7})
	require.NoError(t, err)

	assert.Len(t, f.requestsTo("/pico/v2/components/7/sboms"), 2)
	assert.Contains(t, res.Matches, match14BeforeCommit, "SBOM 14 is on page 2")
}

func TestFindSourceSbomVersions_ComponentWithoutSourceFallsBackToFolder(t *testing.T) {
	_, c := newFakeSourceServer(t)

	opts := iacRepo("app-code/frontend-console")
	opts.ComponentID = 8
	res, _, err := findSource(t, c, opts)
	require.NoError(t, err)

	want := match13ByCommit
	want.MatchedBy = MatchedByFolder
	assert.Equal(t, []SourceVersionMatch{want}, res.Matches)
	assert.Empty(t, res.Unmatched)
}

func TestFindSourceSbomVersions_ComponentWithSourceIgnoresFolder(t *testing.T) {
	f, c := newFakeSourceServer(t)

	opts := iacRepo(".")
	opts.ComponentID = 7
	res, _, err := findSource(t, c, opts)
	require.NoError(t, err)

	assert.Empty(t, f.requestsTo("/pico/v2/sboms/id/by-repo"))
	assert.Equal(t, []SourceVersionMatch{match13ByCommit, match14BeforeCommit}, res.Matches, "SBOM 30 (the repo root) must not be added")
}

func TestFindSourceSbomVersions_FolderOnly(t *testing.T) {
	f, c := newFakeSourceServer(t)

	res, _, err := findSource(t, c, iacRepo("app-code/frontend-console"))
	require.NoError(t, err)

	want := match13ByCommit
	want.MatchedBy = MatchedByFolder
	assert.Equal(t, []SourceVersionMatch{want}, res.Matches)
	assert.Len(t, f.requestsTo("/pico/v2/sboms/id/by-repo"), 1)
}

func TestFindSourceSbomVersions_NoVersionBeforeCommitIsUnmatched(t *testing.T) {
	_, c := newFakeSourceServer(t)

	res, _, err := findSource(t, c, FindSourceSbomVersionsOptions{ComponentID: 9})
	require.NoError(t, err)

	assert.Equal(t, []SourceVersionMatch{match14BeforeCommit}, res.Matches)
	assert.Equal(t, []SourceVersionUnmatched{
		{SbomID: 15, Name: "kusaridev/iac/app-code/new", MatchedBy: MatchedByComponent},
		{SbomID: 16, Name: "kusaridev/iac/app-code/newer", MatchedBy: MatchedByComponent},
	}, res.Unmatched, "an empty list and a 404 both mean no version yet")
}

func TestFindSourceSbomVersions_VersionWithoutCommitOmitsKey(t *testing.T) {
	f, c := newFakeSourceServer(t)
	f.sbom14CommitSha = ""

	res, _, err := findSource(t, c, FindSourceSbomVersionsOptions{ComponentID: 7})
	require.NoError(t, err)

	require.Len(t, res.Matches, 2)
	require.Equal(t, 14, res.Matches[1].SbomID)
	raw, err := json.Marshal(res.Matches[1])
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	assert.NotContains(t, fields, "commit_sha")
	assert.Contains(t, fields, "version_id")
}

func TestFindSourceSbomVersions_ByIdentifier404UsesAsOfForAll(t *testing.T) {
	f, c := newFakeSourceServer(t)
	f.byIdentifierStatus = http.StatusNotFound

	res, _, err := findSource(t, c, FindSourceSbomVersionsOptions{ComponentID: 7})
	require.NoError(t, err)

	want13 := match13ByCommit
	want13.VersionFoundBy = VersionFoundByNewestBeforeCommit
	assert.Equal(t, []SourceVersionMatch{want13, match14BeforeCommit}, res.Matches)
}

func TestFindSourceSbomVersions_PicksNewestOfSeveralVersionsAtCommit(t *testing.T) {
	f, c := newFakeSourceServer(t)
	// The same commit uploaded twice gives SBOM 13 two versions at it; the newer one is what is deployed.
	f.byIdentifier = []map[string]any{
		{"sbom_id": 13, "version_id": 4819},
		{"sbom_id": 13, "version_id": 4821},
		{"sbom_id": 13, "version_id": 4820},
	}

	res, _, err := findSource(t, c, FindSourceSbomVersionsOptions{ComponentID: 7})
	require.NoError(t, err)

	assert.Equal(t, []SourceVersionMatch{match13ByCommit, match14BeforeCommit}, res.Matches)
}

func TestFindSourceSbomVersions_OtherAPIErrorFails(t *testing.T) {
	f, c := newFakeSourceServer(t)
	f.byIdentifierStatus = http.StatusInternalServerError

	res, _, err := findSource(t, c, FindSourceSbomVersionsOptions{ComponentID: 7})
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "finding versions at commit aaa")
	assert.Contains(t, err.Error(), "status 500")
}

func TestFindSourceSbomVersions_NothingFoundPrintsEmptyArrays(t *testing.T) {
	_, c := newFakeSourceServer(t)

	res, log, err := findSource(t, c, FindSourceSbomVersionsOptions{ComponentID: 8})
	require.NoError(t, err)

	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.JSONEq(t, `{"matches":[],"unmatched":[]}`, string(raw))
	assert.Contains(t, log, "Component 8 has no source SBOM")
}

func TestFindSourceSbomVersions_RequiresCommit(t *testing.T) {
	f, c := newFakeSourceServer(t)

	_, err := c.FindSourceSbomVersions(context.Background(), FindSourceSbomVersionsOptions{ComponentID: 7, CommitTime: testCommitTime})
	require.Error(t, err)
	_, err = c.FindSourceSbomVersions(context.Background(), FindSourceSbomVersionsOptions{ComponentID: 7, CommitSha: "aaa"})
	require.Error(t, err)
	assert.Empty(t, f.requests)
}

func TestPickScanFolder(t *testing.T) {
	at := func(id int, dir string) repoSbom {
		return repoSbom{SbomID: id, Name: fmt.Sprintf("sbom-%d", id), SubrepoPath: dir, Type: "source"}
	}
	tests := []struct {
		name      string
		scans     []repoSbom
		folder    string
		wantID    int // 0 means nothing picked
		wantBelow []string
		wantErr   string
	}{
		{"exact folder", []repoSbom{at(1, "app-code/frontend-console")}, "app-code/frontend-console", 1, nil, ""},
		{"parent folder", []repoSbom{at(1, "app-code/apigatewayv2")}, "app-code/apigatewayv2/webhooks", 1, nil, ""},
		{"repo root covers everything", []repoSbom{at(1, ".")}, "app-code/frontend-console", 1, nil, ""},
		{"similar name is not a parent", []repoSbom{at(1, "app-code/frontend")}, "app-code/frontend-console", 0, nil, ""},
		{"child folder is not picked", []repoSbom{at(1, "app-code/frontend-console/server")}, "app-code/frontend-console", 0, []string{"app-code/frontend-console/server"}, ""},
		{"two at the same folder is an error", []repoSbom{at(1, "app-code/frontend-console"), at(2, "app-code/frontend-console")}, "app-code/frontend-console", 0, nil, "2 source SBOMs"},
		{"closest of several parents", []repoSbom{at(1, "."), at(2, "app-code/apigatewayv2"), at(3, "app-code")}, "app-code/apigatewayv2/webhooks", 2, nil, ""},
		{"repo root asked for", []repoSbom{at(1, "app-code/frontend-console"), at(2, ".")}, ".", 2, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, below, err := pickScanFolder(tt.folder, tt.scans)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.wantID == 0 {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantID, got.SbomID)
			}
			assert.Equal(t, tt.wantBelow, below)
		})
	}
}
