// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package ai

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAuthError_DetectsNoTokenError(t *testing.T) {
	err := fmt.Errorf("failed to load auth token: no stored tokens found. Run `kusari auth login`")
	assert.True(t, isAuthError(err))
}

func TestIsAuthError_DetectsExpiredTokenError(t *testing.T) {
	err := fmt.Errorf("Token is expired. Re-run `kusari auth login`")
	assert.True(t, isAuthError(err))
}

func TestIsAuthError_ReturnsFalseForOtherErrors(t *testing.T) {
	err := fmt.Errorf("some other error")
	assert.False(t, isAuthError(err))
}

func TestIsAuthError_ReturnsFalseForNilError(t *testing.T) {
	assert.False(t, isAuthError(nil))
}
