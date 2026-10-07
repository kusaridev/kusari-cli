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
	"path"
	"slices"
	"sort"
	"strings"
)

// Values of SourceVersionMatch.MatchedBy and SourceVersionMatch.VersionFoundBy.
const (
	MatchedByComponent               = "component"
	MatchedByFolder                  = "folder"
	VersionFoundByCommit             = "commit"
	VersionFoundByNewestBeforeCommit = "newest_before_commit"
)

// FindSourceSbomVersionsOptions describes a deploy. CommitSha, CommitTime and at least one of
// ComponentID or Forge/Org/Repo are required.
type FindSourceSbomVersionsOptions struct {
	CommitSha string
	// CommitTime is when the commit landed on the branch (its committer date), RFC3339. It is
	// passed to the API as is.
	CommitTime  string
	ComponentID int // 0 means not set
	Forge       string
	Org         string
	Repo        string
	// SubrepoPath is the folder, from the repo root, the deployed code is built from. Empty means the repo root.
	SubrepoPath string
	// Log receives one line per decision. nil discards them.
	Log io.Writer
}

// SourceVersionMatch is one source SBOM version to tag.
type SourceVersionMatch struct {
	SbomID         int    `json:"sbom_id"`
	VersionID      int    `json:"version_id"`
	Name           string `json:"name"`
	MatchedBy      string `json:"matched_by"`
	VersionFoundBy string `json:"version_found_by"`
	CommitSha      string `json:"commit_sha,omitempty"`
}

// SourceVersionUnmatched is a source SBOM that had no version by the commit time.
type SourceVersionUnmatched struct {
	SbomID    int    `json:"sbom_id"`
	Name      string `json:"name"`
	MatchedBy string `json:"matched_by"`
}

// SourceVersionResult is what FindSourceSbomVersions found. Both slices are non-nil, so they
// encode as [] rather than null.
type SourceVersionResult struct {
	Matches   []SourceVersionMatch     `json:"matches"`
	Unmatched []SourceVersionUnmatched `json:"unmatched"`
}

// repoSbom is the subset of V2SBOMIDAndDetails (a by-repo row) the folder rule needs.
type repoSbom struct {
	SbomID      int    `json:"sbom_id"`
	Name        string `json:"name"`
	SubrepoPath string `json:"subrepo_path"`
	Type        string `json:"type"`
}

// sourceCandidate is a source SBOM that was deployed, before its version is known.
type sourceCandidate struct {
	SbomID    int
	Name      string
	MatchedBy string
}

// FindSourceSbomVersions finds the source SBOM versions that belong to a deploy of a commit.
//
// The candidates are the component's source SBOMs. Only when it has none (or no component is given)
// is the repo's source SBOM whose scan folder is SubrepoPath, or the closest folder above it, used.
// Each candidate gets its version at the commit if there is one, otherwise the newest version
// ingested at or before CommitTime. A candidate with neither is reported in Unmatched.
//
// A 404 from any lookup means that lookup found nothing; finding nothing at all is not an error.
// Any other API error is returned.
func (c *Client) FindSourceSbomVersions(ctx context.Context, opts FindSourceSbomVersionsOptions) (*SourceVersionResult, error) {
	if opts.CommitSha == "" || opts.CommitTime == "" {
		return nil, fmt.Errorf("commit SHA and commit time must not be empty")
	}
	log := opts.Log
	if log == nil {
		log = io.Discard
	}

	res := &SourceVersionResult{Matches: []SourceVersionMatch{}, Unmatched: []SourceVersionUnmatched{}}

	candidates, err := c.sourceCandidates(ctx, opts, log)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return res, nil
	}

	atCommit, err := c.versionsAtCommit(ctx, opts.CommitSha)
	if err != nil {
		return nil, err
	}

	for _, cand := range candidates {
		if vid, ok := atCommit[cand.SbomID]; ok {
			_, _ = fmt.Fprintf(log, "SBOM %d (%s): version %d is at commit %s\n", cand.SbomID, cand.Name, vid, opts.CommitSha)
			res.Matches = append(res.Matches, SourceVersionMatch{
				SbomID: cand.SbomID, VersionID: vid, Name: cand.Name, MatchedBy: cand.MatchedBy,
				VersionFoundBy: VersionFoundByCommit, CommitSha: opts.CommitSha,
			})
			continue
		}

		v, err := c.newestVersionAsOf(ctx, cand.SbomID, opts.CommitTime)
		if err != nil {
			return nil, err
		}
		if v == nil {
			_, _ = fmt.Fprintf(log, "SBOM %d (%s): no version at or before %s\n", cand.SbomID, cand.Name, opts.CommitTime)
			res.Unmatched = append(res.Unmatched, SourceVersionUnmatched(cand))
			continue
		}
		_, _ = fmt.Fprintf(log, "SBOM %d (%s): no version at commit %s; using version %d, the newest before it\n", cand.SbomID, cand.Name, opts.CommitSha, v.ID)
		res.Matches = append(res.Matches, SourceVersionMatch{
			SbomID: cand.SbomID, VersionID: v.ID, Name: cand.Name, MatchedBy: cand.MatchedBy,
			VersionFoundBy: VersionFoundByNewestBeforeCommit, CommitSha: v.CommitSha,
		})
	}

	return res, nil
}

// sourceCandidates returns the component's source SBOMs, or, if there are none, the one the folder rule picks.
func (c *Client) sourceCandidates(ctx context.Context, opts FindSourceSbomVersionsOptions, log io.Writer) ([]sourceCandidate, error) {
	if opts.ComponentID > 0 {
		cands, err := c.componentSourceSboms(ctx, opts.ComponentID)
		if err != nil {
			return nil, err
		}
		if len(cands) > 0 {
			names := make([]string, len(cands))
			for i, cand := range cands {
				names[i] = fmt.Sprintf("%d (%s)", cand.SbomID, cand.Name)
			}
			_, _ = fmt.Fprintf(log, "Component %d has source SBOMs %s\n", opts.ComponentID, strings.Join(names, ", "))
			return cands, nil
		}
		_, _ = fmt.Fprintf(log, "Component %d has no source SBOM\n", opts.ComponentID)
	}

	if opts.Repo == "" {
		_, _ = fmt.Fprintln(log, "No repo given, so there is no folder to look up")
		return nil, nil
	}
	return c.folderSourceSbom(ctx, opts, log)
}

// componentSourceSboms lists every page of the component's SBOMs and keeps the source ones, by SBOM ID.
// A 404 (no such component) gives none.
func (c *Client) componentSourceSboms(ctx context.Context, compID int) ([]sourceCandidate, error) {
	var cands []sourceCandidate
	for page := 0; ; page++ {
		raw, err := c.ListComponentSboms(ctx, compID, ListComponentSbomsOptions{Page: page, Size: 1000})
		if isNotFound(err) {
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
				cands = append(cands, sourceCandidate{SbomID: s.ID, Name: s.Name, MatchedBy: MatchedByComponent})
			}
		}
		if page+1 >= pg.TotalPages || len(pg.Sboms) == 0 {
			break
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].SbomID < cands[j].SbomID })
	return cands, nil
}

// folderSourceSbom looks up the repo's source SBOMs and returns the one the folder rule picks, if any.
func (c *Client) folderSourceSbom(ctx context.Context, opts FindSourceSbomVersionsOptions, log io.Writer) ([]sourceCandidate, error) {
	repo := opts.Forge + "/" + opts.Org + "/" + opts.Repo
	// No subrepo_path filter: the API matches it exactly, and the folder rule needs every scan folder.
	raw, err := c.FindSbomIDsByRepo(ctx, FindSbomIDsByRepoOptions{Forge: opts.Forge, Org: opts.Org, Repo: opts.Repo})
	if isNotFound(err) {
		_, _ = fmt.Fprintf(log, "No SBOM is recorded for %s\n", repo)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding SBOMs in %s: %w", repo, err)
	}
	var rows []repoSbom
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("decoding SBOMs in %s: %w", repo, err)
	}
	var sources []repoSbom
	for _, r := range rows {
		if r.Type == "source" {
			sources = append(sources, r)
		}
	}

	picked, below, err := pickScanFolder(opts.SubrepoPath, sources)
	if err != nil {
		return nil, fmt.Errorf("picking the source SBOM for folder %s in %s: %w", cleanDir(opts.SubrepoPath), repo, err)
	}
	if picked == nil {
		msg := fmt.Sprintf("No source SBOM in %s has a scan folder at or above %s", repo, cleanDir(opts.SubrepoPath))
		if len(below) > 0 {
			msg += "; scan folders below it: " + strings.Join(below, ", ")
		}
		_, _ = fmt.Fprintln(log, msg)
		return nil, nil
	}
	_, _ = fmt.Fprintf(log, "Folder %s is covered by scan folder %s: SBOM %d (%s)\n", cleanDir(opts.SubrepoPath), cleanDir(picked.SubrepoPath), picked.SbomID, picked.Name)
	return []sourceCandidate{{SbomID: picked.SbomID, Name: picked.Name, MatchedBy: MatchedByFolder}}, nil
}

// pickScanFolder returns the SBOM whose scan folder is folder itself or, failing that, the closest
// folder above it ("." is the repo root and covers everything). This is the rule gh-pr uses
// (coveringUnit in gh-pr's supersede.go). A folder that only shares a name prefix is not above it.
// When nothing covers folder, it returns the scan folders below it instead, for the log. Two SBOMs at
// the picked folder is an error rather than a guess.
func pickScanFolder(folder string, sboms []repoSbom) (*repoSbom, []string, error) {
	folder = cleanDir(folder)
	var (
		bestDir string
		best    []repoSbom
		below   []string
	)
	for _, s := range sboms {
		dir := cleanDir(s.SubrepoPath)
		switch {
		case dirCovers(dir, folder):
			if len(best) == 0 || len(dir) > len(bestDir) {
				bestDir, best = dir, []repoSbom{s}
			} else if dir == bestDir {
				best = append(best, s)
			}
		case dirCovers(folder, dir):
			below = append(below, dir)
		}
	}

	switch len(best) {
	case 0:
		sort.Strings(below)
		return nil, slices.Compact(below), nil
	case 1:
		return &best[0], nil, nil
	default:
		ids := make([]string, len(best))
		for i, s := range best {
			ids[i] = fmt.Sprintf("%d", s.SbomID)
		}
		return nil, nil, fmt.Errorf("%d source SBOMs (%s) have scan folder %s; not guessing which was deployed", len(best), strings.Join(ids, ", "), bestDir)
	}
}

// dirCovers reports whether dir is folder itself or a folder above it. Both must be cleaned by cleanDir.
func dirCovers(dir, folder string) bool {
	return dir == "." || dir == folder || strings.HasPrefix(folder, dir+"/")
}

// cleanDir normalizes a repo-relative folder: no leading "./" or slashes, and "." for the repo root.
func cleanDir(d string) string {
	d = strings.TrimPrefix(path.Clean("/"+d), "/")
	if d == "" {
		return "."
	}
	return d
}

// versionsAtCommit returns, for each SBOM with a version at the commit, its newest such version ID.
// A 404 (nothing at the commit) gives an empty map.
func (c *Client) versionsAtCommit(ctx context.Context, commitSha string) (map[int]int, error) {
	raw, err := c.FindSbomIDsByIdentifier(ctx, commitSha)
	if isNotFound(err) {
		return map[int]int{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding versions at commit %s: %w", commitSha, err)
	}
	var rows []struct {
		SbomID    int `json:"sbom_id"`
		VersionID int `json:"version_id"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("decoding versions at commit %s: %w", commitSha, err)
	}
	// Version IDs grow with each upload, so the highest is the newest upload of this commit.
	out := make(map[int]int, len(rows))
	for _, r := range rows {
		if r.VersionID > out[r.SbomID] {
			out[r.SbomID] = r.VersionID
		}
	}
	return out, nil
}

// sbomVersionRef is the subset of V2SBOMVersion FindSourceSbomVersions needs.
type sbomVersionRef struct {
	ID        int    `json:"id"`
	CommitSha string `json:"commit_sha"`
}

// newestVersionAsOf returns the SBOM's newest version ingested at or before asOf, or nil if it had
// none by then. A 404 (no such SBOM) also gives nil.
func (c *Client) newestVersionAsOf(ctx context.Context, sbomID int, asOf string) (*sbomVersionRef, error) {
	raw, err := c.GetSbomVersions(ctx, sbomID, GetSbomVersionsOptions{AsOf: asOf, Size: 1, Sort: "first_ingested_desc"})
	if isNotFound(err) {
		return nil, nil
	}
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

// isNotFound reports whether err is an API 404.
func isNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}
