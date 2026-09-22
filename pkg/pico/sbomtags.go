// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// SbomVersionTag is a tag on an SBOM version (V2SBOMTag). A tag is an interval: it applies from
// StartTimestamp until EndTimestamp, and a nil EndTimestamp means it is still in effect.
type SbomVersionTag struct {
	ID             int     `json:"id"`
	VersionID      int     `json:"version_id"`
	TagLabel       string  `json:"tag_label"`
	TagValue       string  `json:"tag_value"`
	StartTimestamp string  `json:"start_timestamp"`
	EndTimestamp   *string `json:"end_timestamp"`
}

// MoveTagResult reports what MoveSbomVersionTag did. Exactly one of Created or Existing is set.
type MoveTagResult struct {
	// Created is the tag newly placed on the target version, or nil if it already carried the tag.
	Created *SbomVersionTag
	// Existing is the tag the target version already carried, or nil if one had to be created.
	Existing *SbomVersionTag
	// Ended lists the tags on other versions that were closed, ordered by version ID.
	// On a partial failure it holds the tags that were closed before the error.
	Ended []SbomVersionTag
	// EndTimestamp is the instant the Ended tags were closed at: the start of the tag on the target version.
	EndTimestamp string
}

// sbomVersionsPage is the subset of V2SBOMVersionList that MoveSbomVersionTag needs.
type sbomVersionsPage struct {
	Versions []struct {
		ID   int              `json:"id"`
		Tags []SbomVersionTag `json:"tags"`
	} `json:"versions"`
	TotalPages int `json:"total_pages"`
}

// sbomTagsPage is the subset of V2SBOMTagList that MoveSbomVersionTag needs.
type sbomTagsPage struct {
	Tags []SbomVersionTag `json:"tags"`
}

// MoveSbomVersionTag makes versionID the only version of sbomID carrying label=value.
//
// It tags versionID (unless it already carries the tag in effect) and then closes that tag on every
// other version of the SBOM, setting their end_timestamp to the start_timestamp of the tag on
// versionID so the intervals meet with no gap or overlap. The timestamp comes from the server, so
// the caller's clock is never involved.
//
// The operation is idempotent: rerunning it after a partial failure skips the create if the target
// already carries the tag and closes whatever is still open. On error the returned result is
// non-nil and describes what was completed before the failure.
func (c *Client) MoveSbomVersionTag(ctx context.Context, sbomID, versionID int, label, value string) (*MoveTagResult, error) {
	if label == "" || value == "" {
		return nil, fmt.Errorf("label and value must not be empty")
	}

	// 1. Find every version currently carrying label=value, with the tag IDs that carry it.
	// The sbom_tag_* filter selects the versions, but each version embeds all of its in-effect
	// tags (e.g. environment=dev alongside environment=prod), so pick out the matching tag(s)
	// to get the IDs to close (for example, closing dev during a dev deploy but keeping prod).
	// Only tags in effect are embedded, so no end_timestamp check is needed.
	holders := map[int][]SbomVersionTag{}
	const pageSize = 1000
	for page := 0; ; page++ {
		raw, err := c.GetSbomVersions(ctx, sbomID, page, pageSize, "", label, value, "")
		if err != nil {
			return nil, fmt.Errorf("listing versions carrying %s=%s: %w", label, value, err)
		}
		var pg sbomVersionsPage
		if err := json.Unmarshal(raw, &pg); err != nil {
			return nil, fmt.Errorf("decoding versions response: %w", err)
		}
		for _, v := range pg.Versions {
			for _, t := range v.Tags {
				if tagMatches(t, label, value) {
					holders[v.ID] = append(holders[v.ID], t)
				}
			}
		}
		if page+1 >= pg.TotalPages || len(pg.Versions) == 0 {
			break
		}
	}

	res := &MoveTagResult{}

	// 2. Make sure the target version carries the tag.
	// If it already has the tag, do nothing
	if existing := holders[versionID]; len(existing) > 0 {
		res.Existing = &existing[0]
	} else {
		// Otherwise, tag it
		tag, created, err := c.createOrFindSbomVersionTag(ctx, sbomID, versionID, label, value)
		if err != nil {
			return nil, err
		}
		if created {
			res.Created = tag
		} else {
			res.Existing = tag
		}
	}

	anchor := res.Created
	if anchor == nil {
		anchor = res.Existing
	}
	res.EndTimestamp = anchor.StartTimestamp

	// 3. Close the tag on every other version, in a stable order.
	otherVersions := make([]int, 0, len(holders))
	for vid := range holders {
		if vid != versionID {
			otherVersions = append(otherVersions, vid)
		}
	}
	sort.Ints(otherVersions)

	for _, vid := range otherVersions {
		for _, t := range holders[vid] {
			body := map[string]any{"end_timestamp": res.EndTimestamp}
			if _, err := c.UpdateSbomVersionTag(ctx, sbomID, vid, t.ID, body); err != nil {
				return res, fmt.Errorf("ending tag #%d (%s=%s) on version #%d: %w", t.ID, t.TagLabel, t.TagValue, vid, err)
			}
			end := res.EndTimestamp
			t.EndTimestamp = &end
			res.Ended = append(res.Ended, t)
		}
	}

	return res, nil
}

// createOrFindSbomVersionTag creates label=value on the version and reports created=true. If the API
// answers 409 because the version already carries it (a race with another writer), the existing tag
// is looked up and returned with created=false.
func (c *Client) createOrFindSbomVersionTag(ctx context.Context, sbomID, versionID int, label, value string) (tag *SbomVersionTag, created bool, err error) {
	raw, err := c.CreateSbomVersionTag(ctx, sbomID, versionID, label, value)
	if err != nil {
		// Create failed. Only a 409 (already exists) is recoverable; anything else is a real failure, return it.
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict {
			return nil, false, fmt.Errorf("creating tag %s=%s on version #%d: %w", label, value, versionID, err)
		}

		// It is a 409, so the version already carries label=value in effect.
		// Fetch the version's in-effect tags with this label so we can hand back the
		// existing tag; its start_timestamp is what the caller uses to end the old holders.
		listed, listErr := c.ListSbomVersionTags(ctx, sbomID, versionID, 0, 1000, label, true)
		if listErr != nil {
			return nil, false, fmt.Errorf("creating tag %s=%s on version #%d: %w (and listing existing tags failed: %v)", label, value, versionID, err, listErr)
		}
		var pg sbomTagsPage
		if jsonErr := json.Unmarshal(listed, &pg); jsonErr != nil {
			return nil, false, fmt.Errorf("decoding tags response: %w", jsonErr)
		}

		// The label query param filters on label only, so the list can also hold e.g.
		// environment=dev when we asked about environment=prod. Match on value too, and
		// report created=false since we did not make this tag.
		for i := range pg.Tags {
			if tagMatches(pg.Tags[i], label, value) {
				return &pg.Tags[i], false, nil
			}
		}

		// The API said the tag exists but the listing does not show it. Surface the original 409
		// rather than guess; a rerun re-scans and will most likely pick it up.
		return nil, false, fmt.Errorf("creating tag %s=%s on version #%d: %w", label, value, versionID, err)
	}

	var newTag SbomVersionTag
	if err := json.Unmarshal(raw, &newTag); err != nil {
		return nil, false, fmt.Errorf("decoding created tag: %w", err)
	}
	return &newTag, true, nil
}

// tagMatches compares label and value case-insensitively, since the API lowercases both on store.
func tagMatches(t SbomVersionTag, label, value string) bool {
	return strings.EqualFold(t.TagLabel, label) && strings.EqualFold(t.TagValue, value)
}
