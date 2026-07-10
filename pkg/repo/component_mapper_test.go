// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// mockPico is an in-memory pico API for exercising mapSoftwareToComponents:
// component create (with duplicate-name rejection), list search, software
// assignment (with source-software conflict), and the software detail
// endpoint used for verification.
type mockPico struct {
	mu         sync.Mutex
	nextID     int64
	components map[string]int64 // name -> id
	sources    map[int64]int64  // component id -> source software id
	swToComp   map[int64]int64  // software id -> component id
	failAll    bool
}

func newMockPico() *mockPico {
	return &mockPico{
		nextID:     100,
		components: map[string]int64{},
		sources:    map[int64]int64{},
		swToComp:   map[int64]int64{},
	}
}

// seed adds a component and optionally marks it as already sourcing a software.
func (m *mockPico) seed(name string, sourceSoftware int64) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	m.components[name] = id
	if sourceSoftware != 0 {
		m.sources[id] = sourceSoftware
		m.swToComp[sourceSoftware] = id
	}
	return id
}

func (m *mockPico) nameOf(id int64) string {
	for n, i := range m.components {
		if i == id {
			return n
		}
	}
	return ""
}

func (m *mockPico) handler() http.Handler {
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()

		if m.failAll {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"reason": "boom"})
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/pico/v1/components":
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, exists := m.components[body.Name]; exists {
				writeJSON(w, http.StatusBadRequest, picoReason{Reason: fmt.Sprintf("component with name %q already exists", body.Name)})
				return
			}
			id := m.nextID
			m.nextID++
			m.components[body.Name] = id
			writeJSON(w, http.StatusCreated, componentIDAndName{ID: id, Name: body.Name})

		case r.Method == http.MethodGet && r.URL.Path == "/pico/v1/components":
			search := r.URL.Query().Get("search")
			page := componentsListPage{Components: []componentIDAndName{}}
			for name, id := range m.components {
				if search == "" || strings.Contains(name, search) {
					page.Components = append(page.Components, componentIDAndName{ID: id, Name: name})
				}
			}
			writeJSON(w, http.StatusOK, page)

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/pico/v1/components/") && strings.HasSuffix(r.URL.Path, "/software"):
			var compID int64
			_, _ = fmt.Sscanf(r.URL.Path, "/pico/v1/components/%d/software", &compID)
			var body struct {
				SoftwareIDs []int64 `json:"software_ids"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if src, ok := m.sources[compID]; ok && (len(body.SoftwareIDs) != 1 || src != body.SoftwareIDs[0]) {
				writeJSON(w, http.StatusBadRequest, picoReason{Reason: "component already has a source software"})
				return
			}
			for _, sw := range body.SoftwareIDs {
				m.sources[compID] = sw
				m.swToComp[sw] = compID
			}
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/pico/v1/software/"):
			var swID int64
			_, _ = fmt.Sscanf(r.URL.Path, "/pico/v1/software/%d", &swID)
			info := softwareComponentInfo{}
			if compID, ok := m.swToComp[swID]; ok {
				name := m.nameOf(compID)
				info.ComponentID = &compID
				info.ComponentName = &name
			}
			writeJSON(w, http.StatusOK, info)

		default:
			writeJSON(w, http.StatusNotFound, picoReason{Reason: "unhandled: " + r.Method + " " + r.URL.Path})
		}
	})
}

func i64Ptr(v int64) *int64 {
	return &v
}

func TestMapSoftwareToComponents(t *testing.T) {
	docRef := "sha256_abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	t.Run("clean map creates component and writes back", func(t *testing.T) {
		mock := newMockPico()
		server := httptest.NewServer(mock.handler())
		defer server.Close()

		results := []sbomResult{
			{SbomID: i64Ptr(1), SbomSubject: "cleanapp", SoftwareID: i64Ptr(30), SoftwareName: "cleanapp", docRef: docRef},
		}
		if err := mapSoftwareToComponents(context.Background(), server.Client(), "token", server.URL, results); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results[0].ComponentID == nil {
			t.Fatal("expected component_id to be written back")
		}
		if results[0].ComponentName == nil || *results[0].ComponentName != "cleanapp" {
			t.Errorf("expected component_name 'cleanapp', got %v", results[0].ComponentName)
		}
		if got := mock.componentOf(30); got != *results[0].ComponentID {
			t.Errorf("mock has software 30 in component %d, results say %d", got, *results[0].ComponentID)
		}
	})

	t.Run("already-mapped entry untouched", func(t *testing.T) {
		mock := newMockPico()
		server := httptest.NewServer(mock.handler())
		defer server.Close()

		name := "already"
		results := []sbomResult{
			{SbomID: i64Ptr(1), SbomSubject: "already", SoftwareID: i64Ptr(10), SoftwareName: "already", ComponentID: i64Ptr(55), ComponentName: &name, docRef: docRef},
		}
		if err := mapSoftwareToComponents(context.Background(), server.Client(), "token", server.URL, results); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *results[0].ComponentID != 55 {
			t.Errorf("expected component_id 55 untouched, got %d", *results[0].ComponentID)
		}
	})

	t.Run("error entry skipped", func(t *testing.T) {
		mock := newMockPico()
		server := httptest.NewServer(mock.handler())
		defer server.Close()

		results := []sbomResult{
			{SbomSubject: "broken", SoftwareName: "broken", Error: "lookup timed out", docRef: docRef},
		}
		if err := mapSoftwareToComponents(context.Background(), server.Client(), "token", server.URL, results); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results[0].ComponentID != nil {
			t.Error("expected error entry to stay unmapped")
		}
	})

	t.Run("name exists unoccupied is reused", func(t *testing.T) {
		mock := newMockPico()
		existingID := mock.seed("webapp", 0)
		server := httptest.NewServer(mock.handler())
		defer server.Close()

		results := []sbomResult{
			{SbomID: i64Ptr(4), SbomSubject: "webapp", SoftwareID: i64Ptr(40), SoftwareName: "webapp", docRef: docRef},
		}
		if err := mapSoftwareToComponents(context.Background(), server.Client(), "token", server.URL, results); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *results[0].ComponentID != existingID {
			t.Errorf("expected reuse of existing component %d, got %d", existingID, *results[0].ComponentID)
		}
	})

	t.Run("source conflict falls back to hash-suffixed component", func(t *testing.T) {
		mock := newMockPico()
		mock.seed("webapp", 99) // occupied by another software
		server := httptest.NewServer(mock.handler())
		defer server.Close()

		results := []sbomResult{
			{SbomID: i64Ptr(4), SbomSubject: "webapp", SoftwareID: i64Ptr(40), SoftwareName: "webapp", docRef: docRef},
		}
		if err := mapSoftwareToComponents(context.Background(), server.Client(), "token", server.URL, results); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results[0].ComponentName == nil || *results[0].ComponentName != "webapp-abcdef1" {
			t.Errorf("expected fallback component 'webapp-abcdef1', got %v", results[0].ComponentName)
		}
	})

	t.Run("hard API failure aborts", func(t *testing.T) {
		mock := newMockPico()
		mock.failAll = true
		server := httptest.NewServer(mock.handler())
		defer server.Close()

		results := []sbomResult{
			{SbomID: i64Ptr(1), SbomSubject: "app", SoftwareID: i64Ptr(5), SoftwareName: "app", docRef: docRef},
		}
		err := mapSoftwareToComponents(context.Background(), server.Client(), "token", server.URL, results)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "unexpected status 500") {
			t.Errorf("expected status-500 error, got: %v", err)
		}
	})
}

// componentOf reads the component a software landed in from mock state.
func (m *mockPico) componentOf(sw int64) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.swToComp[sw]
}

func TestFallbackSuffix(t *testing.T) {
	tests := []struct {
		name     string
		result   sbomResult
		expected string
	}{
		{
			name:     "docRef hash prefix",
			result:   sbomResult{docRef: "sha256_abcdef1234567890"},
			expected: "abcdef1",
		},
		{
			name:     "missing docRef falls back to sbom id",
			result:   sbomResult{SbomID: i64Ptr(777)},
			expected: "777",
		},
		{
			name:     "malformed docRef falls back to sbom id",
			result:   sbomResult{docRef: "sha256_ab", SbomID: i64Ptr(777)},
			expected: "777",
		},
		{
			name:     "nothing available",
			result:   sbomResult{},
			expected: "unmapped",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fallbackSuffix(tt.result); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
