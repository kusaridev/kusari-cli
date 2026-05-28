// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		flag string
		want bool
	}{
		{"empty args", nil, "--output", false},
		{"bare match", []string{"--output", "x"}, "--output", true},
		{"equals match", []string{"--output=x"}, "--output", true},
		{"absent", []string{"--path", "."}, "--output", false},
		{"prefix-only is NOT a match", []string{"--outputs"}, "--output", false},
		{"value containing flag name is not a match", []string{"--path", "--output"}, "--path", true}, // first one is the bare match
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, hasFlag(c.args, c.flag))
		})
	}
}

func TestCountFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		flag string
		want int
	}{
		{"zero", []string{"--path", "."}, "--output", 0},
		{"one bare", []string{"--output", "x"}, "--output", 1},
		{"one equals", []string{"--output=x"}, "--output", 1},
		{"two bare", []string{"--output", "x", "--output", "y"}, "--output", 2},
		{"two equals", []string{"--output=x", "--output=y"}, "--output", 2},
		{"mixed forms", []string{"--output", "x", "--output=y"}, "--output", 2},
		{"per-format equals (still counts)", []string{"--output", "cdx=a.json", "--output", "spdx=b.json"}, "--output", 2},
		{"prefix-only is NOT counted", []string{"--outputs", "--outputter"}, "--output", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, countFlag(c.args, c.flag))
		})
	}
}

func TestFlagValue(t *testing.T) {
	cases := []struct {
		name string
		args []string
		flag string
		want string
	}{
		{"absent", []string{"--path", "."}, "--output", ""},
		{"bare", []string{"--output", "foo.json"}, "--output", "foo.json"},
		{"equals", []string{"--output=foo.json"}, "--output", "foo.json"},
		{"bare returns next token even if it looks flag-like", []string{"--output", "--quiet"}, "--output", "--quiet"},
		{"missing value at end", []string{"--output"}, "--output", ""},
		{"first match wins (bare)", []string{"--output", "a", "--output", "b"}, "--output", "a"},
		{"first match wins (equals)", []string{"--output=a", "--output=b"}, "--output", "a"},
		{"per-format value with embedded =", []string{"--output", "cdx=foo.json"}, "--output", "cdx=foo.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, flagValue(c.args, c.flag))
		})
	}
}

func TestSbomOutputPath(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		defPath string
		want    string
	}{
		{"no --output uses default", []string{"--path", "."}, "project.cdx.json", "project.cdx.json"},
		{"default flows through", []string{}, "project.spdx.json", "project.spdx.json"},
		{"bare --output overrides default", []string{"--output", "foo.json"}, "project.cdx.json", "foo.json"},
		{"--output=PATH overrides default", []string{"--output=foo.json"}, "project.cdx.json", "foo.json"},
		{"per-format FMT=PATH strips FMT=", []string{"--output", "cyclonedx-json=foo.cdx.json"}, "project.cdx.json", "foo.cdx.json"},
		{"per-format with embedded equals", []string{"--output=cyclonedx-json=foo.cdx.json"}, "project.cdx.json", "foo.cdx.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, sbomOutputPath(c.args, c.defPath))
		})
	}
}

func TestDefaultSbomFilename(t *testing.T) {
	cases := []struct {
		name   string
		format string
		want   string
	}{
		{"empty matches mikebom's default", "", "project.cdx.json"},
		{"explicit cyclonedx-json", "cyclonedx-json", "project.cdx.json"},
		{"spdx-2.3-json", "spdx-2.3-json", "project.spdx.json"},
		{"spdx-3-json", "spdx-3-json", "project.spdx.json"},
		{"spdx-3-json-experimental", "spdx-3-json-experimental", "project.spdx.json"},
		{"unknown format falls back to generic", "future-format-v9", "project.sbom.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, defaultSbomFilename(c.format))
		})
	}
}
