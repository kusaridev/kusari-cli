// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package useragent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransportSetsHeaderUnlessCallerDid(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get("User-Agent"))
	}))
	defer srv.Close()
	client := &http.Client{Transport: NewTransport(nil)}

	send := func(ua *string) *http.Request {
		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		if ua != nil {
			req.Header.Set("User-Agent", *ua)
		}
		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return req
	}

	plain := send(nil)
	caller := "my-tool/1.0"
	send(&caller)
	Set("example-bot/1.0")
	send(nil)
	Set("")
	send(nil)

	require.Len(t, got, 4)
	assert.Equal(t, String(), got[0])
	assert.True(t, strings.HasPrefix(got[0], "kusari-cli/"), got[0])
	// Catches a transport that sets the header on the caller's request instead of a clone.
	assert.Empty(t, plain.Header.Get("User-Agent"))
	// Catches a transport that always overwrites.
	assert.Equal(t, caller, got[1])
	assert.Equal(t, "example-bot/1.0", got[2])
	// Catches Set("") leaving an empty override in place.
	assert.Equal(t, got[0], got[3])
}

func TestBuildAddsCIName(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(k string) string { return vars[k] }
	}

	local := build(env(nil))
	assert.True(t, strings.HasPrefix(local, "kusari-cli/"+Version()+" ("), local)
	assert.NotContains(t, local, "ci=")

	gitlab := build(env(map[string]string{"GITLAB_CI": "true"}))
	assert.True(t, strings.HasSuffix(gitlab, "; ci=gitlab)"), gitlab)
}
