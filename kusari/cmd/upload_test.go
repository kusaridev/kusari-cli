// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestLoadUploadFromViper guards against drift between uploadStringVars /
// uploadBoolVars and the package-level upload* variables. If a new key is
// added to either map without a corresponding var (or vice versa), this
// test will catch it at build time (compile error on &nonexistentVar) or
// at run time (wrong value materialized).
func TestLoadUploadFromViper(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	stringExpected := map[string]string{
		"file-path":                     "fp",
		"alias":                         "a",
		"document-type":                 "dt",
		"tag":                           "tg",
		"software-id":                   "sid",
		"sbom-subject":                  "ss",
		"component-name":                "cn",
		"sbom-subject-name-override":    "ssno",
		"sbom-subject-version-override": "ssvo",
		"forge":                         "fg",
		"org":                           "og",
		"repo":                          "rp",
		"subrepo-path":                  "srp",
		"commit-sha":                    "csh",
		"results-file":                  "rf",
	}
	boolExpected := map[string]bool{
		"openvex":                true,
		"check-blocked-packages": true,
		"wait":                   true,
	}

	for k, v := range stringExpected {
		viper.Set(k, v)
	}
	for k, v := range boolExpected {
		viper.Set(k, v)
	}

	loadUploadFromViper()

	// Every key in the maps must have been materialized into the matching var.
	for key, want := range stringExpected {
		ptr, ok := uploadStringVars[key]
		assert.True(t, ok, "uploadStringVars missing key %q", key)
		if ok {
			assert.Equal(t, want, *ptr, "viper key %q did not flow into its var", key)
		}
	}
	for key, want := range boolExpected {
		ptr, ok := uploadBoolVars[key]
		assert.True(t, ok, "uploadBoolVars missing key %q", key)
		if ok {
			assert.Equal(t, want, *ptr, "viper key %q did not flow into its var", key)
		}
	}

	// And every map key must have been covered by the test (drift in the
	// other direction: var added, test not updated).
	for key := range uploadStringVars {
		_, ok := stringExpected[key]
		assert.True(t, ok, "uploadStringVars has key %q not covered by test", key)
	}
	for key := range uploadBoolVars {
		_, ok := boolExpected[key]
		assert.True(t, ok, "uploadBoolVars has key %q not covered by test", key)
	}
}
