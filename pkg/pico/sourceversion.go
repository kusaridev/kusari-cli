// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Values of SourceVersionMatch.VersionFoundBy.
const (
	VersionFoundByCommit             = "commit"
	VersionFoundByNewestBeforeCommit = "newest_before_commit"
)

// FindSourceSbomVersionsOptions describes a deployed version of an image SBOM.
type FindSourceSbomVersionsOptions struct {
	ImageSbomID    int
	ImageVersionID int
	// CommitTime, if set, is when the image's commit landed on the branch (its committer date),
	// RFC3339, passed to the API as is. Without it, the image version's upload time is used, which
	// is usually a few minutes later.
	CommitTime string
	// Log receives one line per decision. nil discards them.
	Log io.Writer
}

// SourceVersionMatch is the source SBOM version to tag.
type SourceVersionMatch struct {
	SbomID         int    `json:"sbom_id"`
	VersionID      int    `json:"version_id"`
	Name           string `json:"name"`
	VersionFoundBy string `json:"version_found_by"`
	CommitSha      string `json:"commit_sha,omitempty"`
}

// SourceSbom is a source SBOM that had no version by the time used.
type SourceSbom struct {
	SbomID int    `json:"sbom_id"`
	Name   string `json:"name"`
}

// SourceVersionResult is what FindSourceSbomVersions found. A component holds at most one visible
// source SBOM, so each list has at most one entry. Both are non-nil, so they encode as [] rather than null.
type SourceVersionResult struct {
	Matches   []SourceVersionMatch `json:"matches"`
	Unmatched []SourceSbom         `json:"unmatched"`
}

// sbomVersionRef is the subset of V2SBOMVersion FindSourceSbomVersions needs.
type sbomVersionRef struct {
	ID            int    `json:"id"`
	CommitSha     string `json:"commit_sha"`
	FirstIngested string `json:"first_ingested"`
}

// FindSourceSbomVersions finds the version of the source SBOM that was deployed with a version of an
// image SBOM.
//
// The source SBOM is the one in the image SBOM's component. If the image SBOM is in no component, or
// its component has no source SBOM, the result is empty. Otherwise the source SBOM gets its version at
// the image version's commit if there is one, else the newest version ingested at or before CommitTime
// (or the image version's upload time); with neither, it is reported in Unmatched.
//
// A 404 from any lookup means that lookup found nothing; finding nothing at all is not an error.
// Any other API error is returned.
func (c *Client) FindSourceSbomVersions(ctx context.Context, opts FindSourceSbomVersionsOptions) (*SourceVersionResult, error) {
	if opts.ImageSbomID <= 0 || opts.ImageVersionID <= 0 {
		return nil, fmt.Errorf("image SBOM ID and version ID must be set")
	}
	log := opts.Log
	if log == nil {
		log = io.Discard
	}

	res := &SourceVersionResult{Matches: []SourceVersionMatch{}, Unmatched: []SourceSbom{}}

	src, err := c.imageSourceSbom(ctx, opts.ImageSbomID, log)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return res, nil
	}

	image, err := c.imageVersion(ctx, opts.ImageSbomID, opts.ImageVersionID)
	if isNotFound(err) {
		_, _ = fmt.Fprintf(log, "Version %d of SBOM %d not found\n", opts.ImageVersionID, opts.ImageSbomID)
		return res, nil
	}
	if err != nil {
		return nil, err
	}

	noVersionAtCommit := ""
	if image.CommitSha == "" {
		_, _ = fmt.Fprintf(log, "Version %d of SBOM %d has no commit recorded, so the source version is found by time only\n", opts.ImageVersionID, opts.ImageSbomID)
	} else {
		vid, err := c.versionAtCommit(ctx, src.SbomID, image.CommitSha)
		if err != nil {
			return nil, err
		}
		if vid > 0 {
			_, _ = fmt.Fprintf(log, "SBOM %d (%s): version %d is at commit %s\n", src.SbomID, src.Name, vid, image.CommitSha)
			res.Matches = append(res.Matches, SourceVersionMatch{
				SbomID: src.SbomID, VersionID: vid, Name: src.Name, VersionFoundBy: VersionFoundByCommit, CommitSha: image.CommitSha,
			})
			return res, nil
		}
		noVersionAtCommit = fmt.Sprintf("no version at commit %s; ", image.CommitSha)
	}

	asOf := opts.CommitTime
	if asOf == "" {
		if image.FirstIngested == "" {
			return nil, fmt.Errorf("version %d of SBOM %d has no upload time; give the commit time instead", opts.ImageVersionID, opts.ImageSbomID)
		}
		asOf = image.FirstIngested
	}

	v, err := c.newestVersionAsOf(ctx, src.SbomID, asOf)
	switch {
	case isNotFound(err):
		_, _ = fmt.Fprintf(log, "SBOM %d (%s) not found\n", src.SbomID, src.Name)
		res.Unmatched = append(res.Unmatched, *src)
	case err != nil:
		return nil, err
	case v == nil:
		_, _ = fmt.Fprintf(log, "SBOM %d (%s): no version at or before %s\n", src.SbomID, src.Name, asOf)
		res.Unmatched = append(res.Unmatched, *src)
	default:
		_, _ = fmt.Fprintf(log, "SBOM %d (%s): %susing version %d, the newest as of %s\n", src.SbomID, src.Name, noVersionAtCommit, v.ID, asOf)
		res.Matches = append(res.Matches, SourceVersionMatch{
			SbomID: src.SbomID, VersionID: v.ID, Name: src.Name, VersionFoundBy: VersionFoundByNewestBeforeCommit, CommitSha: v.CommitSha,
		})
	}
	return res, nil
}

// imageVersion returns the commit and upload time recorded on a version of the image SBOM.
func (c *Client) imageVersion(ctx context.Context, sbomID, versionID int) (*sbomVersionRef, error) {
	raw, err := c.GetSbomVersion(ctx, sbomID, versionID)
	if err != nil {
		return nil, fmt.Errorf("looking up version %d of SBOM %d: %w", versionID, sbomID, err)
	}
	var v sbomVersionRef
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("decoding version %d of SBOM %d: %w", versionID, sbomID, err)
	}
	return &v, nil
}

// imageSourceSbom returns the source SBOM in the image SBOM's component, or nil if there is none.
func (c *Client) imageSourceSbom(ctx context.Context, imageID int, log io.Writer) (*SourceSbom, error) {
	raw, err := c.GetSbom(ctx, imageID)
	if isNotFound(err) {
		_, _ = fmt.Fprintf(log, "SBOM %d not found\n", imageID)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("looking up SBOM %d: %w", imageID, err)
	}
	var image struct {
		ComponentID *int `json:"component_id"`
	}
	if err := json.Unmarshal(raw, &image); err != nil {
		return nil, fmt.Errorf("decoding SBOM %d: %w", imageID, err)
	}
	if image.ComponentID == nil {
		_, _ = fmt.Fprintf(log, "SBOM %d is not in a component; put it and its source SBOM in one to tag the source\n", imageID)
		return nil, nil
	}
	compID := *image.ComponentID

	for page := 0; ; page++ {
		raw, err := c.ListComponentSboms(ctx, compID, ListComponentSbomsOptions{Page: page, Size: 1000})
		if isNotFound(err) {
			_, _ = fmt.Fprintf(log, "Component %d not found\n", compID)
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("listing SBOMs of component %d: %w", compID, err)
		}
		var pg struct {
			Sboms []struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				SbomType string `json:"sbom_type"`
			} `json:"sboms"`
			TotalPages int `json:"total_pages"`
		}
		if err := json.Unmarshal(raw, &pg); err != nil {
			return nil, fmt.Errorf("decoding SBOMs of component %d: %w", compID, err)
		}
		for _, s := range pg.Sboms {
			if s.SbomType == "source" {
				_, _ = fmt.Fprintf(log, "SBOM %d is in component %d, whose source SBOM is %d (%s)\n", imageID, compID, s.ID, s.Name)
				return &SourceSbom{SbomID: s.ID, Name: s.Name}, nil
			}
		}
		if page+1 >= pg.TotalPages || len(pg.Sboms) == 0 {
			break
		}
	}
	_, _ = fmt.Fprintf(log, "Component %d of SBOM %d has no source SBOM; add one to it to tag the source\n", compID, imageID)
	return nil, nil
}

// versionAtCommit returns the SBOM's newest version at the commit, or 0 if it has none there.
func (c *Client) versionAtCommit(ctx context.Context, sbomID int, commitSha string) (int, error) {
	raw, err := c.FindSbomIDsByIdentifier(ctx, commitSha)
	if isNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("finding versions at commit %s: %w", commitSha, err)
	}
	var rows []struct {
		SbomID    int `json:"sbom_id"`
		VersionID int `json:"version_id"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return 0, fmt.Errorf("decoding versions at commit %s: %w", commitSha, err)
	}
	// The rows cover every SBOM built from the commit. Version IDs grow with each upload, so the
	// highest is the newest upload of this commit.
	vid := 0
	for _, r := range rows {
		if r.SbomID == sbomID && r.VersionID > vid {
			vid = r.VersionID
		}
	}
	return vid, nil
}

// newestVersionAsOf returns the SBOM's newest version ingested at or before asOf, or nil if it had
// none by then. A missing SBOM is returned as the API's 404.
func (c *Client) newestVersionAsOf(ctx context.Context, sbomID int, asOf string) (*sbomVersionRef, error) {
	raw, err := c.GetSbomVersions(ctx, sbomID, GetSbomVersionsOptions{AsOf: asOf, Size: 1, Sort: "first_ingested_desc"})
	if err != nil {
		return nil, fmt.Errorf("finding the newest version of SBOM %d at or before %s: %w", sbomID, asOf, err)
	}
	var pg struct {
		Versions []sbomVersionRef `json:"versions"`
	}
	if err := json.Unmarshal(raw, &pg); err != nil {
		return nil, fmt.Errorf("decoding versions of SBOM %d: %w", sbomID, err)
	}
	if len(pg.Versions) == 0 {
		return nil, nil
	}
	return &pg.Versions[0], nil
}

// isNotFound reports whether err is, or wraps, an API 404.
func isNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}
