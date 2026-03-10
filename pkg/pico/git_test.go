// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoInfo_Structure(t *testing.T) {
	info := &RepoInfo{
		Forge:       "github.com",
		Org:         "kusaridev",
		Repo:        "pico",
		SubrepoPath: ".",
	}

	assert.Equal(t, "github.com", info.Forge)
	assert.Equal(t, "kusaridev", info.Org)
	assert.Equal(t, "pico", info.Repo)
	assert.Equal(t, ".", info.SubrepoPath)
}

// Note: ExtractGitRemoteInfo requires a real git repository, so full testing
// would need integration tests or mocking of exec.Command. For now, we test
// the structure and will rely on manual testing for the git operations.
