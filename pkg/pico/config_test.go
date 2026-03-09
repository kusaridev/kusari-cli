// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstants(t *testing.T) {
	assert.Equal(t, "https://auth.us.kusari.cloud/", defaultAuthEndpoint)
	assert.Equal(t, "https://api.us.kusari.cloud", defaultPlatformURL)
}
