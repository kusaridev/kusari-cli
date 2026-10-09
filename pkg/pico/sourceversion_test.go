// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"bytes"
	"context"
	"encoding/json"
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

// testCommitTime has a non-UTC offset, which testCommitCutoff, 5 minutes later, keeps.
// testUploadTime is when the fake's image versions were uploaded, a few minutes after the commit.
const (
	testCommitTime        = "2026-10-01T14:34:56+02:00"
	testCommitCutoff      = "2026-10-01T14:39:56+02:00"
	testCommitCutoffInLog = testCommitCutoff + " (5 minutes after the commit time)"
	testUploadTime        = "2026-10-01T12:40:00Z"
)

type sourceFakeRequest struct {
	Path  string
	Query url.Values
}

// fakeSourceServer stands in for the lookups FindSourceSbomVersion makes:
//
//   - Image SBOMs 20, 21, 22, 23 and 24 are in components 7, 8, 9, 10 and 11. Image SBOM 25 is in no
//     component, image SBOM 26 is in component 99, which does not exist, and SBOM 27 does not exist.
//   - Image SBOM N has version 880+N (so SBOM 20 has version 900), recorded at commit aaa and
//     uploaded at imageUploadTime (testUploadTime by default). Any other version is 404.
//   - Component 7 has SBOMs 20 (image) and 19 (build) and, on page 2, SBOM 13 (source).
//   - Component 8 has only SBOM 21 (image).
//   - Components 9, 10 and 11 have source SBOMs 14, 15 and 16. The API allows one visible source
//     SBOM per component.
//   - by-identifier for commit aaa returns SBOM 20 version 900 and SBOM 13 version 4821.
//   - versions as_of: SBOM 13 -> 4821; SBOM 14 -> 4790 at testCommitCutoff, 4795 at testUploadTime,
//     4800 at any other time; SBOM 15 -> empty list; SBOM 16 -> 404.
//
// Any other request fails the test, which is how "no request was made" is checked.
type fakeSourceServer struct {
	t *testing.T

	mu                 sync.Mutex
	imageComponent     map[int]any // component_id of each image SBOM; nil means none
	imageCommitSha     string      // commit recorded on the image versions
	imageUploadTime    string      // first_ingested of the image versions
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
		t:               t,
		imageComponent:  map[int]any{20: 7, 21: 8, 22: 9, 23: 10, 24: 11, 25: nil, 26: 99},
		imageCommitSha:  "aaa",
		imageUploadTime: testUploadTime,
		componentPages: map[int][][]map[string]any{
			7: {
				{sbom(20, "web-app-image", "image"), sbom(19, "web-app-build", "build")},
				{sbom(13, "example-org/web-app", "source")},
			},
			8:  {{sbom(21, "other-image", "image")}},
			9:  {{sbom(14, "example-org/api-service", "source"), sbom(22, "api-service-image", "image")}},
			10: {{sbom(15, "example-org/new-service", "source"), sbom(23, "new-service-image", "image")}},
			11: {{sbom(16, "example-org/newer-service", "source"), sbom(24, "newer-service-image", "image")}},
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
	f.requests = append(f.requests, sourceFakeRequest{Path: r.URL.Path, Query: r.URL.Query()})
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
	notFound := func() { reply(http.StatusNotFound, map[string]string{"error": "not found"}) }
	sbomPath := strings.Split(strings.TrimPrefix(r.URL.Path, "/pico/v2/sboms/"), "/")
	sbomID, _ := strconv.Atoi(sbomPath[0])

	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/pico/v2/components/") && strings.HasSuffix(r.URL.Path, "/sboms"):
		id, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/pico/v2/components/"), "/sboms"))
		if id == 99 {
			notFound()
			return
		}
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

	case r.Method == http.MethodGet && len(sbomPath) == 1:
		if sbomID == 27 {
			notFound()
			return
		}
		comp, ok := f.imageComponent[sbomID]
		if !ok {
			unexpected()
			return
		}
		reply(http.StatusOK, map[string]any{"id": sbomID, "component_id": comp})

	case r.Method == http.MethodGet && len(sbomPath) == 3 && sbomPath[1] == "versions":
		if _, ok := f.imageComponent[sbomID]; !ok {
			unexpected()
			return
		}
		vid, _ := strconv.Atoi(sbomPath[2])
		if vid != 880+sbomID {
			notFound()
			return
		}
		reply(http.StatusOK, map[string]any{"id": vid, "sbom_id": sbomID, "commit_sha": f.imageCommitSha, "first_ingested": f.imageUploadTime})

	case r.Method == http.MethodGet && len(sbomPath) == 2 && sbomPath[1] == "versions":
		version := func(vid int, sha string) map[string]any {
			return map[string]any{"versions": []any{map[string]any{"id": vid, "sbom_id": sbomID, "commit_sha": sha}}, "total_pages": 1}
		}
		switch sbomID {
		case 13:
			reply(http.StatusOK, version(4821, "aaa"))
		case 14:
			switch q.Get("as_of") {
			case testCommitCutoff:
				reply(http.StatusOK, version(4790, f.sbom14CommitSha))
			case testUploadTime:
				reply(http.StatusOK, version(4795, "ddd"))
			default:
				reply(http.StatusOK, version(4800, "ccc"))
			}
		case 15:
			reply(http.StatusOK, map[string]any{"versions": []any{}, "total_pages": 0})
		case 16:
			notFound()
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

// findSource runs FindSourceSbomVersion for the fake's version of image SBOM imageID with
// testCommitTime, and returns the result and what was logged.
func findSource(t *testing.T, c *Client, imageID int) (*SourceSbomVersion, string, error) {
	t.Helper()
	return findSourceWith(t, c, FindSourceSbomVersionOptions{ImageSbomID: imageID, ImageVersionID: 880 + imageID, CommitTime: testCommitTime})
}

func findSourceWith(t *testing.T, c *Client, opts FindSourceSbomVersionOptions) (*SourceSbomVersion, string, error) {
	t.Helper()
	var log bytes.Buffer
	opts.Log = &log
	res, err := c.FindSourceSbomVersion(context.Background(), opts)
	return res, log.String(), err
}

// match13ByCommit is SBOM 13's version at commit aaa. match14BeforeCommit is SBOM 14's newest
// version as of testCommitCutoff.
func match13ByCommit() *SourceSbomVersion {
	return &SourceSbomVersion{
		SbomID: 13, Name: "example-org/web-app", VersionID: 4821, VersionFoundBy: VersionFoundByCommit, CommitSha: "aaa",
		Message: "version 4821 is at commit aaa",
	}
}

func match14BeforeCommit() *SourceSbomVersion {
	return &SourceSbomVersion{
		SbomID: 14, Name: "example-org/api-service", VersionID: 4790, VersionFoundBy: VersionFoundByNewestBeforeCommit, CommitSha: "bbb",
		Message: "no version at commit aaa; using version 4790, the newest as of " + testCommitCutoffInLog,
	}
}

func TestFindSourceSbomVersion_VersionAtCommit(t *testing.T) {
	f, c := newFakeSourceServer(t)

	res, log, err := findSource(t, c, 20)
	require.NoError(t, err)

	assert.Equal(t, match13ByCommit(), res, "SBOM 20 is the image and SBOM 19 a build SBOM; SBOM 13 is on page 2")
	pages := f.requestsTo("/pico/v2/components/7/sboms")
	require.Len(t, pages, 2)
	for _, p := range pages {
		assert.Equal(t, "active", p.Query.Get("visibility"), "hidden source SBOMs must never be picked")
	}
	assert.Len(t, f.requestsTo("/pico/v2/sboms/20/versions/900"), 1, "the commit comes from the image version")
	assert.Empty(t, f.requestsTo("/pico/v2/sboms/13/versions"), "SBOM 13 has a version at the commit, so no as_of lookup")
	assert.Contains(t, log, "SBOM 13 (example-org/web-app): version 4821 is at commit aaa", "the message is also logged")
}

func TestFindSourceSbomVersion_NewestVersionBeforeCommit(t *testing.T) {
	f, c := newFakeSourceServer(t)

	res, _, err := findSource(t, c, 22)
	require.NoError(t, err)

	assert.Equal(t, match14BeforeCommit(), res, "4795 and 4800 are newer than the cutoff")
	got := f.requestsTo("/pico/v2/sboms/14/versions")
	require.Len(t, got, 1)
	assert.Equal(t, testCommitCutoff, got[0].Query.Get("as_of"), "5 minutes after the commit time, in its offset")
	assert.Equal(t, "1", got[0].Query.Get("size"))
	assert.Equal(t, "first_ingested_desc", got[0].Query.Get("sort"), "size=1 must return the newest, not the oldest")
}

func TestFindSourceSbomVersion_WithoutCommitTimeUsesImageUploadTime(t *testing.T) {
	f, c := newFakeSourceServer(t)

	res, _, err := findSourceWith(t, c, FindSourceSbomVersionOptions{ImageSbomID: 22, ImageVersionID: 902})
	require.NoError(t, err)

	want := match14BeforeCommit()
	want.VersionID, want.CommitSha = 4795, "ddd"
	want.Message = "no version at commit aaa; using version 4795, the newest as of " + testUploadTime
	assert.Equal(t, want, res)
	got := f.requestsTo("/pico/v2/sboms/14/versions")
	require.Len(t, got, 1)
	assert.Equal(t, testUploadTime, got[0].Query.Get("as_of"), "the upload time is already after the commit, so no grace is added")
}

func TestFindSourceSbomVersion_NoCommitTimeAndNoUploadTimeFails(t *testing.T) {
	f, c := newFakeSourceServer(t)
	f.imageUploadTime = ""

	res, _, err := findSourceWith(t, c, FindSourceSbomVersionOptions{ImageSbomID: 22, ImageVersionID: 902})
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "no upload time")
	assert.Empty(t, f.requestsTo("/pico/v2/sboms/14/versions"), "an empty as_of would be dropped and return the newest version ever")
}

func TestFindSourceSbomVersion_ImageVersionWithoutCommitUsesTimeOnly(t *testing.T) {
	f, c := newFakeSourceServer(t)
	f.imageCommitSha = ""

	res, _, err := findSource(t, c, 22)
	require.NoError(t, err)

	want := match14BeforeCommit()
	want.Message = "the image version has no commit recorded; using version 4790, the newest as of " + testCommitCutoffInLog
	assert.Equal(t, want, res)
	assert.Empty(t, f.requestsTo("/pico/v2/sboms/id/by-identifier"), "no commit to look up")
}

func TestFindSourceSbomVersion_NoSourceSbomIsNull(t *testing.T) {
	tests := []struct {
		name    string
		imageID int
		wantLog string
	}{
		{"component has no source SBOM", 21, "Component 8 of SBOM 21 has no source SBOM"},
		{"image in no component", 25, "SBOM 25 is not in a component"},
		{"component not found", 26, "Component 99 not found"},
		{"image SBOM not found", 27, "SBOM 27 not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, c := newFakeSourceServer(t)

			res, log, err := findSource(t, c, tt.imageID)
			require.NoError(t, err)

			assert.Nil(t, res)
			assert.Contains(t, log, tt.wantLog)
			assert.Empty(t, f.requestsTo("/pico/v2/sboms/id/by-identifier"), "no source SBOM, so no version lookups")
		})
	}
}

func TestFindSourceSbomVersion_ImageVersionNotFoundIsNull(t *testing.T) {
	f, c := newFakeSourceServer(t)

	res, log, err := findSourceWith(t, c, FindSourceSbomVersionOptions{ImageSbomID: 22, ImageVersionID: 999, CommitTime: testCommitTime})
	require.NoError(t, err)

	assert.Nil(t, res)
	assert.Contains(t, log, "Version 999 of SBOM 22 not found")
	assert.Empty(t, f.requestsTo("/pico/v2/sboms/14/versions"))
}

func TestFindSourceSbomVersion_NoVersionBeforeCommitHasNoVersionFields(t *testing.T) {
	tests := []struct {
		name     string
		imageID  int
		wantJSON string
	}{
		{"empty versions list", 23, `{"sbom_id": 15, "name": "example-org/new-service",
			"message": "no version at commit aaa; no version at or before ` + testCommitCutoffInLog + `, so there is no version to tag"}`},
		{"versions 404", 24, `{"sbom_id": 16, "name": "example-org/newer-service",
			"message": "no version at commit aaa; the SBOM was not found, so there is no version to tag"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := newFakeSourceServer(t)

			res, _, err := findSource(t, c, tt.imageID)
			require.NoError(t, err)

			raw, err := json.Marshal(res)
			require.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(raw))
		})
	}
}

func TestFindSourceSbomVersion_VersionWithoutCommitOmitsKey(t *testing.T) {
	f, c := newFakeSourceServer(t)
	f.sbom14CommitSha = ""

	res, _, err := findSource(t, c, 22)
	require.NoError(t, err)

	raw, err := json.Marshal(res)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	assert.NotContains(t, fields, "commit_sha")
	assert.Contains(t, fields, "version_id")
}

func TestFindSourceSbomVersion_ByIdentifier404UsesAsOf(t *testing.T) {
	f, c := newFakeSourceServer(t)
	f.byIdentifierStatus = http.StatusNotFound

	res, _, err := findSource(t, c, 20)
	require.NoError(t, err)

	want := match13ByCommit()
	want.VersionFoundBy = VersionFoundByNewestBeforeCommit
	want.Message = "no version at commit aaa; using version 4821, the newest as of " + testCommitCutoffInLog
	assert.Equal(t, want, res)
}

func TestFindSourceSbomVersion_PicksNewestOfSeveralVersionsAtCommit(t *testing.T) {
	f, c := newFakeSourceServer(t)
	// The same commit uploaded twice gives SBOM 13 two versions at it; the newer one is what is deployed.
	f.byIdentifier = []map[string]any{
		{"sbom_id": 13, "version_id": 4819},
		{"sbom_id": 13, "version_id": 4821},
		{"sbom_id": 13, "version_id": 4820},
	}

	res, _, err := findSource(t, c, 20)
	require.NoError(t, err)

	assert.Equal(t, match13ByCommit(), res)
}

func TestFindSourceSbomVersion_OtherAPIErrorFails(t *testing.T) {
	f, c := newFakeSourceServer(t)
	f.byIdentifierStatus = http.StatusInternalServerError

	res, _, err := findSource(t, c, 20)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "finding versions at commit aaa")
	assert.Contains(t, err.Error(), "status 500")
}

func TestFindSourceSbomVersion_ValidatesOptions(t *testing.T) {
	f, c := newFakeSourceServer(t)

	for name, opts := range map[string]FindSourceSbomVersionOptions{
		"no image SBOM ID":        {ImageVersionID: 900},
		"no image version ID":     {ImageSbomID: 20},
		"commit time not RFC3339": {ImageSbomID: 22, ImageVersionID: 902, CommitTime: "2026-10-01 14:34:56"},
	} {
		_, err := c.FindSourceSbomVersion(context.Background(), opts)
		assert.Error(t, err, name)
	}
	assert.Empty(t, f.requests)
}
