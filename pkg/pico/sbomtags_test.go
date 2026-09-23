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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTagServer is a minimal in-memory stand-in for the v2 SBOM versions/tags endpoints that
// MoveSbomVersionTag touches. It records every call so tests can assert on order and bodies.
type fakeTagServer struct {
	t *testing.T

	mu sync.Mutex
	// versions holding tags, keyed by version ID. Only tags in effect are embedded, as the real API does.
	versions map[int][]SbomVersionTag
	// pageSize forces the versions listing to paginate.
	pageSize int
	// nextTagID is used for created tags.
	nextTagID int
	// createStatus overrides the POST response status (0 means 201).
	createStatus int
	// failPatchTagID makes the PATCH for that tag ID return 500.
	failPatchTagID int
	// hideFromVersions keeps these version IDs out of the versions listing, simulating a stale read.
	hideFromVersions map[int]bool

	calls []string // "METHOD path?query" in order
	patch map[int]map[string]any
}

const fakeCreatedStart = "2026-09-22T10:00:00Z"

func newFakeTagServer(t *testing.T, versions map[int][]SbomVersionTag) (*fakeTagServer, *httptest.Server) {
	t.Helper()
	f := &fakeTagServer{t: t, versions: versions, pageSize: 1000, nextTagID: 100, patch: map[int]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeTagServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	body, _ := io.ReadAll(r.Body)
	f.calls = append(f.calls, r.Method+" "+r.URL.RequestURI())
	w.Header().Set("Content-Type", "application/json")

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/pico/v2/sboms/"), "/")
	// parts: [sbomId, "versions"] | [sbomId, "versions", vid, "tags"] | [sbomId, "versions", vid, "tags", tagId]
	switch {
	case r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "versions":
		f.serveVersions(w, r)
	case r.Method == http.MethodPost && len(parts) == 4 && parts[3] == "tags":
		f.serveCreate(w, r, parts[2], body)
	case r.Method == http.MethodGet && len(parts) == 4 && parts[3] == "tags":
		f.serveListTags(w, r, parts[2])
	case r.Method == http.MethodPatch && len(parts) == 5:
		f.servePatch(w, parts[4], body)
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"error":"unexpected %s %s"}`, r.Method, r.URL.Path)
	}
}

func (f *fakeTagServer) serveVersions(w http.ResponseWriter, r *http.Request) {
	label := r.URL.Query().Get("sbom_tag_label")
	value := r.URL.Query().Get("sbom_tag_value")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	// Collect versions carrying the tag, ascending by version ID for determinism.
	type ver struct {
		ID   int              `json:"id"`
		Tags []SbomVersionTag `json:"tags"`
	}
	var all []ver
	for vid, tags := range f.versions {
		if f.hideFromVersions[vid] {
			continue
		}
		var inEffect []SbomVersionTag
		for _, t := range tags {
			if t.EndTimestamp == nil {
				inEffect = append(inEffect, t)
			}
		}
		carries := false
		for _, t := range inEffect {
			if strings.EqualFold(t.TagLabel, label) && strings.EqualFold(t.TagValue, value) {
				carries = true
			}
		}
		if carries {
			all = append(all, ver{ID: vid, Tags: inEffect})
		}
	}
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j-1].ID > all[j].ID; j-- {
			all[j-1], all[j] = all[j], all[j-1]
		}
	}

	totalPages := (len(all) + f.pageSize - 1) / f.pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	start := page * f.pageSize
	end := start + f.pageSize
	if start > len(all) {
		start = len(all)
	}
	if end > len(all) {
		end = len(all)
	}
	resp := map[string]any{"versions": all[start:end], "total_items": len(all), "total_pages": totalPages, "current_page": page}
	_ = json.NewEncoder(w).Encode(resp)
}

func (f *fakeTagServer) serveCreate(w http.ResponseWriter, _ *http.Request, vidStr string, body []byte) {
	if f.createStatus != 0 {
		w.WriteHeader(f.createStatus)
		_, _ = w.Write([]byte(`{"error":"conflict"}`))
		return
	}
	vid, _ := strconv.Atoi(vidStr)
	var req struct {
		TagLabel string `json:"tag_label"`
		TagValue string `json:"tag_value"`
	}
	// Server goroutine: report with Errorf and answer 400, never FailNow.
	if err := json.Unmarshal(body, &req); err != nil {
		f.t.Errorf("create body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	tag := SbomVersionTag{ID: f.nextTagID, VersionID: vid, TagLabel: strings.ToLower(req.TagLabel), TagValue: strings.ToLower(req.TagValue), StartTimestamp: fakeCreatedStart}
	f.nextTagID++
	f.versions[vid] = append(f.versions[vid], tag)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(tag)
}

func (f *fakeTagServer) serveListTags(w http.ResponseWriter, r *http.Request, vidStr string) {
	vid, _ := strconv.Atoi(vidStr)
	label := r.URL.Query().Get("label")
	active := r.URL.Query().Get("active") == "true"
	var out []SbomVersionTag
	for _, t := range f.versions[vid] {
		if label != "" && !strings.EqualFold(t.TagLabel, label) {
			continue
		}
		if active && t.EndTimestamp != nil {
			continue
		}
		out = append(out, t)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"tags": out, "total_items": len(out), "total_pages": 1, "current_page": 0})
}

func (f *fakeTagServer) servePatch(w http.ResponseWriter, tagIDStr string, body []byte) {
	tagID, _ := strconv.Atoi(tagIDStr)
	if tagID == f.failPatchTagID {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"db down"}`))
		return
	}
	var req map[string]any
	// Server goroutine: report with Errorf and answer 400, never FailNow.
	if err := json.Unmarshal(body, &req); err != nil {
		f.t.Errorf("patch body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	f.patch[tagID] = req
	for vid, tags := range f.versions {
		for i := range tags {
			if tags[i].ID != tagID {
				continue
			}
			if end, ok := req["end_timestamp"].(string); ok {
				// Enforce the spec's two rules on end_timestamp so a bad choice of end instant
				// fails here the way it would against the real API.
				endAt, err := time.Parse(time.RFC3339, end)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = fmt.Fprintf(w, `{"error":"end_timestamp not RFC3339: %s"}`, end)
					return
				}
				startAt, err := time.Parse(time.RFC3339, tags[i].StartTimestamp)
				if err == nil && !endAt.After(startAt) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = fmt.Fprintf(w, `{"error":"end_timestamp %s is not after start_timestamp %s"}`, end, tags[i].StartTimestamp)
					return
				}
				if endAt.After(timeNow()) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = fmt.Fprintf(w, `{"error":"end_timestamp %s is in the future"}`, end)
					return
				}
				f.versions[vid][i].EndTimestamp = &end
			}
			_ = json.NewEncoder(w).Encode(f.versions[vid][i])
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func tag(id, vid int, label, value string) SbomVersionTag {
	return SbomVersionTag{ID: id, VersionID: vid, TagLabel: label, TagValue: value, StartTimestamp: "2026-09-01T00:00:00Z"}
}

func TestClient_MoveSbomVersionTag_MovesFromAllHolders(t *testing.T) {
	setupTestAuth(t)
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{
		97: {tag(11, 97, "environment", "prod")},
		98: {tag(12, 98, "environment", "prod"), tag(13, 98, "environment", "dev")},
		99: {},
	})
	client := NewClient(srv.URL)

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "prod")
	require.NoError(t, err)

	require.NotNil(t, res.Created)
	assert.Nil(t, res.Existing)
	assert.Equal(t, 99, res.Created.VersionID)
	assert.Equal(t, fakeCreatedStart, res.Anchor, "old tags end when the new one starts")

	require.Len(t, res.Ended, 2)
	assert.Equal(t, []int{11, 12}, []int{res.Ended[0].ID, res.Ended[1].ID}, "ended in version order")
	assert.Equal(t, map[string]any{"end_timestamp": fakeCreatedStart}, f.patch[11])
	assert.Equal(t, map[string]any{"end_timestamp": fakeCreatedStart}, f.patch[12])
	_, touchedDev := f.patch[13]
	assert.False(t, touchedDev, "a different value on the same label must not be ended")

	require.Len(t, f.calls, 4)
	assert.Contains(t, f.calls[0], "GET /pico/v2/sboms/42/versions?")
	assert.Contains(t, f.calls[0], "sbom_tag_label=environment")
	assert.Contains(t, f.calls[0], "sbom_tag_value=prod")
	assert.Equal(t, "POST /pico/v2/sboms/42/versions/99/tags", f.calls[1])
	assert.Equal(t, "PATCH /pico/v2/sboms/42/versions/97/tags/11", f.calls[2])
	assert.Equal(t, "PATCH /pico/v2/sboms/42/versions/98/tags/12", f.calls[3])
}

func TestClient_MoveSbomVersionTag_IdempotentWhenTargetAlreadyTagged(t *testing.T) {
	setupTestAuth(t)
	existing := tag(15, 99, "environment", "prod")
	existing.StartTimestamp = "2026-09-10T00:00:00Z"
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{
		98: {tag(12, 98, "environment", "prod")},
		99: {existing},
	})
	client := NewClient(srv.URL)

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "prod")
	require.NoError(t, err)

	assert.Nil(t, res.Created)
	require.NotNil(t, res.Existing)
	assert.Equal(t, 15, res.Existing.ID)
	assert.Equal(t, "2026-09-10T00:00:00Z", res.Anchor, "old tag ends when the existing target tag started")
	require.NotNil(t, res.Ended[0].EndTimestamp)
	assert.Equal(t, "2026-09-10T00:00:00Z", *res.Ended[0].EndTimestamp)
	require.Len(t, res.Ended, 1)
	assert.Equal(t, 12, res.Ended[0].ID)

	for _, c := range f.calls {
		assert.False(t, strings.HasPrefix(c, "POST "), "must not create a duplicate tag: %s", c)
	}
}

func TestClient_MoveSbomVersionTag_NoOtherHolders(t *testing.T) {
	setupTestAuth(t)
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{})
	client := NewClient(srv.URL)

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "prod")
	require.NoError(t, err)

	require.NotNil(t, res.Created)
	assert.Empty(t, res.Ended)
	assert.Len(t, f.calls, 2, "one list, one create")
}

func TestClient_MoveSbomVersionTag_CaseInsensitiveMatch(t *testing.T) {
	setupTestAuth(t)
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{
		98: {tag(12, 98, "environment", "prod")},
	})
	client := NewClient(srv.URL)

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "Environment", "PROD")
	require.NoError(t, err)

	require.Len(t, res.Ended, 1)
	assert.Equal(t, 12, res.Ended[0].ID)
	_, patched := f.patch[12]
	assert.True(t, patched)
}

func TestClient_MoveSbomVersionTag_ConflictOnCreateFallsBackToExisting(t *testing.T) {
	setupTestAuth(t)
	// Another writer tagged 99 between our versions scan and our create: the scan does not show
	// 99 as a holder, the create answers 409, and the existing tag must be found and used as the anchor.
	raced := tag(15, 99, "environment", "prod")
	raced.StartTimestamp = "2026-09-15T00:00:00Z"
	// A same-label, different-value tag listed first: the fallback must match on value too,
	// not return the first tag the label filter yields.
	decoy := tag(14, 99, "environment", "dev")
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{
		98: {tag(12, 98, "environment", "prod")},
		99: {decoy, raced},
	})
	f.hideFromVersions = map[int]bool{99: true}
	f.createStatus = http.StatusConflict
	client := NewClient(srv.URL)

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "prod")
	require.NoError(t, err)

	assert.Nil(t, res.Created)
	require.NotNil(t, res.Existing)
	assert.Equal(t, 15, res.Existing.ID, "must pick environment=prod, not the environment=dev decoy")
	assert.Equal(t, "2026-09-15T00:00:00Z", res.Anchor)
	require.Len(t, res.Ended, 1)
	assert.Equal(t, 12, res.Ended[0].ID)

	require.Len(t, f.calls, 4)
	assert.Equal(t, "POST /pico/v2/sboms/42/versions/99/tags", f.calls[1])
	assert.Contains(t, f.calls[2], "GET /pico/v2/sboms/42/versions/99/tags?")
	assert.Contains(t, f.calls[2], "active=true")
	assert.Contains(t, f.calls[2], "label=environment")
	assert.Equal(t, "PATCH /pico/v2/sboms/42/versions/98/tags/12", f.calls[3])
}

func TestClient_MoveSbomVersionTag_NonConflictCreateErrorAborts(t *testing.T) {
	setupTestAuth(t)
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{
		98: {tag(12, 98, "environment", "prod")},
	})
	f.createStatus = http.StatusBadRequest
	client := NewClient(srv.URL)

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "prod")
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "status 400")
	_, patched := f.patch[12]
	assert.False(t, patched, "nothing must be ended if the target could not be tagged")
}

func TestClient_MoveSbomVersionTag_PartialFailureReportsProgress(t *testing.T) {
	setupTestAuth(t)
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{
		97: {tag(11, 97, "environment", "prod")},
		98: {tag(12, 98, "environment", "prod")},
	})
	f.failPatchTagID = 12
	client := NewClient(srv.URL)

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ending tag #12")
	assert.Contains(t, err.Error(), "status 500")

	require.NotNil(t, res, "partial result must be returned so the caller can report what happened")
	require.NotNil(t, res.Created)
	require.Len(t, res.Ended, 1)
	assert.Equal(t, 11, res.Ended[0].ID)
}

func TestClient_MoveSbomVersionTag_PaginatesVersions(t *testing.T) {
	setupTestAuth(t)
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{
		96: {tag(10, 96, "environment", "prod")},
		97: {tag(11, 97, "environment", "prod")},
		98: {tag(12, 98, "environment", "prod")},
	})
	f.pageSize = 2
	client := NewClient(srv.URL)

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "prod")
	require.NoError(t, err)

	require.Len(t, res.Ended, 3)
	pages := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, "GET /pico/v2/sboms/42/versions?") {
			pages++
		}
	}
	assert.Equal(t, 2, pages, "both pages of holders must be read")
}

func TestClient_MoveSbomVersionTag_RejectsEmptyLabelOrValue(t *testing.T) {
	setupTestAuth(t)
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{})
	client := NewClient(srv.URL)

	_, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "", "prod")
	require.Error(t, err)
	_, err = client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "")
	require.Error(t, err)
	assert.Empty(t, f.calls, "validation must happen before any request")
}

func TestAPIError_ErrorsAs(t *testing.T) {
	setupTestAuth(t)
	server, _ := recordingServer(t, http.StatusConflict, map[string]string{"error": "dup"})
	client := NewClient(server.URL)

	_, err := client.CreateSbomVersionTag(context.Background(), 1, 2, "environment", "prod")
	require.Error(t, err)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusConflict, apiErr.StatusCode)
	assert.Contains(t, apiErr.Body, "dup")
	assert.Equal(t, "API request failed with status 409: "+apiErr.Body, err.Error(), "error text unchanged for existing callers")
}

func TestClient_MoveSbomVersionTag_OlderTargetTagEndsNewerHoldersAtNow(t *testing.T) {
	setupTestAuth(t)
	// Degenerate case: the target already carries the tag from T1, and someone later tagged another
	// version at T2 > T1 outside move-tag. Ending that tag at T1 would put its end before its start,
	// which the API rejects with a 400, so it must be ended at the current time instead.
	existing := tag(15, 99, "environment", "prod")
	existing.StartTimestamp = "2026-09-10T00:00:00Z"
	later := tag(12, 98, "environment", "prod")
	later.StartTimestamp = "2026-09-12T00:00:00Z"
	earlier := tag(11, 97, "environment", "prod")
	earlier.StartTimestamp = "2026-09-01T00:00:00Z"
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{97: {earlier}, 98: {later}, 99: {existing}})
	client := NewClient(srv.URL)

	fixedNow := time.Date(2026, 9, 22, 15, 4, 5, 999_000_000, time.UTC)
	prev := timeNow
	timeNow = func() time.Time { return fixedNow }
	t.Cleanup(func() { timeNow = prev })

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "prod")
	require.NoError(t, err)

	assert.Equal(t, "2026-09-10T00:00:00Z", res.Anchor)
	require.Len(t, res.Ended, 2)
	// Version 97 started before the anchor: ended at the anchor, intervals meet.
	assert.Equal(t, map[string]any{"end_timestamp": "2026-09-10T00:00:00Z"}, f.patch[11])
	// Version 98 started after the anchor: ended at now, rounded down to whole seconds.
	assert.Equal(t, map[string]any{"end_timestamp": "2026-09-22T15:04:05Z"}, f.patch[12])
}

func TestEndTimestampFor(t *testing.T) {
	fixedNow := time.Date(2026, 9, 22, 15, 4, 5, 0, time.UTC)
	prev := timeNow
	timeNow = func() time.Time { return fixedNow }
	t.Cleanup(func() { timeNow = prev })
	now := "2026-09-22T15:04:05Z"

	tests := []struct {
		name   string
		anchor string
		start  string
		want   string
	}{
		{"anchor after start uses anchor", "2026-09-10T00:00:00Z", "2026-09-01T00:00:00Z", "2026-09-10T00:00:00Z"},
		{"anchor equal to start uses now", "2026-09-10T00:00:00Z", "2026-09-10T00:00:00Z", now},
		{"anchor before start uses now", "2026-09-10T00:00:00Z", "2026-09-12T00:00:00Z", now},
		{"fractional seconds parse", "2026-09-10T00:00:00.500Z", "2026-09-10T00:00:00.250Z", "2026-09-10T00:00:00.500Z"},
		{"unparseable start falls back to anchor", "2026-09-10T00:00:00Z", "yesterday", "2026-09-10T00:00:00Z"},
		{"unparseable anchor falls back to anchor", "soon", "2026-09-10T00:00:00Z", "soon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := endTimestampFor(tt.anchor, SbomVersionTag{StartTimestamp: tt.start})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_MoveSbomVersionTag_ConflictButTagNotListedSurfaces409(t *testing.T) {
	setupTestAuth(t)
	// The API says 409 (tag exists) but the follow-up listing does not show it. The original
	// 409 must be returned and nothing may be ended.
	f, srv := newFakeTagServer(t, map[int][]SbomVersionTag{
		98: {tag(12, 98, "environment", "prod")},
		99: {},
	})
	f.hideFromVersions = map[int]bool{99: true}
	f.createStatus = http.StatusConflict
	client := NewClient(srv.URL)

	res, err := client.MoveSbomVersionTag(context.Background(), 42, 99, "environment", "prod")
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "status 409")
	assert.Empty(t, f.patch, "nothing may be ended when the target's tag cannot be confirmed")

	require.Len(t, f.calls, 3, "versions scan, failed create, tag listing")
	assert.Contains(t, f.calls[2], "GET /pico/v2/sboms/42/versions/99/tags?")
}
