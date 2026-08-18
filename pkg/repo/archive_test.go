// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWithinRoot covers the boundary that decides whether a symlinked directory
// is followed. Both a false positive (following a link out of the repository)
// and a false negative (dropping a directory that is really inside it) are
// silent, so the edges are pinned explicitly.
func TestWithinRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "src", "repo")

	tests := []struct {
		name string
		p    string
		want bool
	}{
		{"the root itself", root, true},
		{"a direct child", filepath.Join(root, "pkg"), true},
		{"a deep child", filepath.Join(root, "a", "b", "c"), true},
		{"a child with spaces", filepath.Join(root, "vendor mod"), true},
		{
			// The reason this is not a HasPrefix(rel, "..") check: the relative
			// path here is "..cache", which starts with ".." but never leaves.
			name: "a child whose name starts with two dots",
			p:    filepath.Join(root, "..cache"),
			want: true,
		},
		{"a dotfile child", filepath.Join(root, ".config"), true},
		{"the parent", filepath.Dir(root), false},
		{"a sibling", filepath.Join(filepath.Dir(root), "other"), false},
		{
			// "repo-other" shares a textual prefix with "repo" but is not inside it.
			name: "a sibling sharing a name prefix",
			p:    filepath.Join(filepath.Dir(root), "repo-other"),
			want: false,
		},
		{"an unrelated absolute path", filepath.Join(string(filepath.Separator), "etc"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, withinRoot(root, tt.p),
				"withinRoot(%q, %q)", root, tt.p)
		})
	}
}
