// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// picoReason is the structured error body pico returns on 4xx responses.
type picoReason struct {
	Reason string `json:"reason"`
}

type componentIDAndName struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type componentsListPage struct {
	Components []componentIDAndName `json:"components"`
}

// mapSoftwareToComponents ensures every successfully ingested software in
// results is assigned to a component. For each entry with a software ID but
// no component:
//
//  1. Create a component named after the software; if that name already
//     exists, reuse the existing component's ID.
//  2. Assign the software to it. If pico rejects the assignment because the
//     component already has a source software, create a fresh component named
//     "<software-name>-<suffix>" — the suffix is a short prefix of the
//     ingested document's content hash (docRef) — and assign to that instead.
//  3. Verify the mapping via the software detail endpoint and record the
//     final component ID/name back into the results entry, so the results
//     file reflects the post-mapping state.
//
// Entries with a lookup error or an existing component mapping are skipped.
// The first hard failure aborts and is returned; the results slice keeps any
// mappings completed before it.
func mapSoftwareToComponents(ctx context.Context, client *http.Client, accessToken, tenantEndpoint string, results []sbomResult) error {
	for i := range results {
		r := &results[i]
		if r.Error != "" {
			fmt.Fprintf(os.Stderr, "skipping %q: lookup failed: %s\n", r.SbomSubject, r.Error)
			continue
		}
		if r.SoftwareID == nil {
			fmt.Fprintf(os.Stderr, "skipping %q: no software ID\n", r.SbomSubject)
			continue
		}
		if r.ComponentID != nil {
			fmt.Fprintf(os.Stderr, "%s (software %d) already mapped to component %d - nothing to do\n", r.SoftwareName, *r.SoftwareID, *r.ComponentID)
			continue
		}

		fmt.Fprintf(os.Stderr, "%s (software %d) is unmapped - creating/locating component\n", r.SoftwareName, *r.SoftwareID)
		compID, err := ensureComponent(ctx, client, accessToken, tenantEndpoint, r.SoftwareName)
		if err != nil {
			return fmt.Errorf("mapping software %d (%s): %w", *r.SoftwareID, r.SoftwareName, err)
		}

		conflict, err := assignSoftwareToComponent(ctx, client, accessToken, tenantEndpoint, compID, *r.SoftwareID)
		if err != nil {
			return fmt.Errorf("mapping software %d (%s): %w", *r.SoftwareID, r.SoftwareName, err)
		}
		if conflict {
			// The reused component already sources a different software; mint
			// a fresh component with a content-hash suffix instead.
			freshName := fmt.Sprintf("%s-%s", r.SoftwareName, fallbackSuffix(*r))
			fmt.Fprintf(os.Stderr, "component %d already has a source software - creating fallback component %s\n", compID, freshName)
			compID, err = ensureComponent(ctx, client, accessToken, tenantEndpoint, freshName)
			if err != nil {
				return fmt.Errorf("mapping software %d (%s): %w", *r.SoftwareID, r.SoftwareName, err)
			}
			conflict, err = assignSoftwareToComponent(ctx, client, accessToken, tenantEndpoint, compID, *r.SoftwareID)
			if err != nil {
				return fmt.Errorf("mapping software %d (%s): %w", *r.SoftwareID, r.SoftwareName, err)
			}
			if conflict {
				return fmt.Errorf("mapping software %d (%s): fallback component %q already has a source software", *r.SoftwareID, r.SoftwareName, freshName)
			}
		}

		// Verify via the software detail endpoint and record the final
		// mapping from the authoritative response.
		info, err := getSoftwareComponentInfo(ctx, client, accessToken, tenantEndpoint, *r.SoftwareID)
		if err != nil {
			return fmt.Errorf("verifying mapping for software %d (%s): %w", *r.SoftwareID, r.SoftwareName, err)
		}
		if info.ComponentID == nil || *info.ComponentID != compID {
			got := "null"
			if info.ComponentID != nil {
				got = fmt.Sprintf("%d", *info.ComponentID)
			}
			return fmt.Errorf("verification failed for software %d (%s): component_id=%s, expected %d", *r.SoftwareID, r.SoftwareName, got, compID)
		}
		r.ComponentID = info.ComponentID
		r.ComponentName = info.ComponentName
		name := ""
		if info.ComponentName != nil {
			name = *info.ComponentName
		}
		fmt.Fprintf(os.Stderr, "verified: software %d (%s) mapped to component %d (%s)\n", *r.SoftwareID, r.SoftwareName, compID, name)
	}
	return nil
}

// fallbackSuffix derives the fallback component-name suffix for a results
// entry: a short prefix of the ingested document's sha256 (from docRef),
// falling back to the SBOM ID when the docRef is absent or malformed.
func fallbackSuffix(r sbomResult) string {
	if h, ok := strings.CutPrefix(r.docRef, "sha256_"); ok && len(h) >= 7 {
		return h[:7]
	}
	if r.SbomID != nil {
		return fmt.Sprintf("%d", *r.SbomID)
	}
	return "unmapped"
}

// ensureComponent creates a component with the given name and returns its ID.
// If pico reports the name already exists, the existing component's ID is
// looked up and returned instead.
func ensureComponent(ctx context.Context, client *http.Client, accessToken, tenantEndpoint, name string) (int64, error) {
	status, body, err := makePicoJSONRequest(ctx, client, accessToken, tenantEndpoint, http.MethodPost, "pico/v1/components", map[string]any{"name": name})
	if err != nil {
		return 0, fmt.Errorf("creating component %q: %w", name, err)
	}
	if status < 300 {
		var comp componentIDAndName
		if err := json.Unmarshal(body, &comp); err != nil {
			return 0, fmt.Errorf("decoding create response for component %q: %w", name, err)
		}
		return comp.ID, nil
	}
	if status == http.StatusBadRequest && strings.Contains(reasonOf(body), "already exists") {
		return findComponentByName(ctx, client, accessToken, tenantEndpoint, name)
	}
	return 0, fmt.Errorf("creating component %q: unexpected status %d: %s", name, status, string(body))
}

// findComponentByName looks up a component's ID by exact name match.
func findComponentByName(ctx context.Context, client *http.Client, accessToken, tenantEndpoint, name string) (int64, error) {
	query := url.Values{"search": {name}, "size": {"1000"}}
	status, body, err := makePicoJSONRequest(ctx, client, accessToken, tenantEndpoint, http.MethodGet, "pico/v1/components?"+query.Encode(), nil)
	if err != nil {
		return 0, fmt.Errorf("listing components searching for %q: %w", name, err)
	}
	if status >= 300 {
		return 0, fmt.Errorf("listing components searching for %q: unexpected status %d: %s", name, status, string(body))
	}
	var page componentsListPage
	if err := json.Unmarshal(body, &page); err != nil {
		return 0, fmt.Errorf("decoding component list for %q: %w", name, err)
	}
	for _, c := range page.Components {
		if c.Name == name {
			return c.ID, nil
		}
	}
	return 0, fmt.Errorf("component %q reported as existing but not found via list", name)
}

// assignSoftwareToComponent assigns the software to the component. Returns
// conflict=true when pico rejects because the component already has a source
// software (the caller then falls back to a fresh component).
func assignSoftwareToComponent(ctx context.Context, client *http.Client, accessToken, tenantEndpoint string, compID, softwareID int64) (bool, error) {
	path := fmt.Sprintf("pico/v1/components/%d/software", compID)
	status, body, err := makePicoJSONRequest(ctx, client, accessToken, tenantEndpoint, http.MethodPost, path, map[string]any{"software_ids": []int64{softwareID}})
	if err != nil {
		return false, fmt.Errorf("assigning software %d to component %d: %w", softwareID, compID, err)
	}
	if status < 300 {
		return false, nil
	}
	if status == http.StatusBadRequest && strings.Contains(reasonOf(body), "already has a source software") {
		return true, nil
	}
	return false, fmt.Errorf("assigning software %d to component %d: unexpected status %d: %s", softwareID, compID, status, string(body))
}

// reasonOf extracts the structured "reason" field from a pico 4xx body.
// Returns the empty string when the body isn't the expected shape.
func reasonOf(body []byte) string {
	var r picoReason
	if err := json.Unmarshal(body, &r); err != nil {
		return ""
	}
	return r.Reason
}

// makePicoJSONRequest makes an authenticated request with an optional JSON
// body and returns the response status and body. Unlike makePicoRequest it
// supports non-GET methods and does not treat 4xx as a transport error —
// callers inspect the status to detect structured pico rejections.
func makePicoJSONRequest(ctx context.Context, client *http.Client, accessToken, tenantURL, method, pathAndQS string, body any) (int, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/%s", tenantURL, pathAndQS), reqBody)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("reading response body: %w", err)
	}
	return res.StatusCode, respBody, nil
}
