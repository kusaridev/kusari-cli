// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

// Package useragent holds the CLI's build version and the User-Agent header
// every outbound HTTP request carries.
package useragent

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

// Set at link time by goreleaser:
// -X github.com/kusaridev/kusari-cli/v2/pkg/useragent.version=v{{.Version}} (likewise commit and date).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Version returns the release version, falling back to the module version
// recorded by `go install`, then "dev".
func Version() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return version
}

// Commit returns the release commit, falling back to the short VCS revision
// recorded at build time, then "none".
func Commit() string {
	if commit != "none" {
		return commit
	}
	if v := buildSetting("vcs.revision"); v != "" {
		if len(v) > 7 {
			return v[:7]
		}
		return v
	}
	return commit
}

// Date returns the release build date, falling back to the VCS commit time,
// then "unknown".
func Date() string {
	if date != "unknown" {
		return date
	}
	if v := buildSetting("vcs.time"); v != "" {
		return v
	}
	return date
}

func buildSetting(key string) string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == key {
				return s.Value
			}
		}
	}
	return ""
}

var (
	override     atomic.Pointer[string]
	defaultAgent = sync.OnceValue(func() string { return build(os.Getenv) })
)

// String returns the User-Agent sent on outbound requests: the value passed to
// Set, or kusari-cli/<version> (<os>/<arch>; <commit>[; ci=<provider>]).
func String() string {
	if ua := override.Load(); ua != nil {
		return *ua
	}
	return defaultAgent()
}

// Set replaces the User-Agent, for programs that use these packages as a
// library and want their own name in server logs. An empty string restores
// the default.
func Set(ua string) {
	if ua == "" {
		override.Store(nil)
		return
	}
	override.Store(&ua)
}

func build(getenv func(string) string) string {
	details := runtime.GOOS + "/" + runtime.GOARCH
	if c := Commit(); c != "none" {
		details += "; " + c
	}
	if ci := ciProvider(getenv); ci != "" {
		details += "; ci=" + ci
	}
	return fmt.Sprintf("kusari-cli/%s (%s)", Version(), details)
}

// ciProvider names the CI system the CLI runs in. It returns only these fixed
// names, never an environment variable's value.
func ciProvider(getenv func(string) string) string {
	switch {
	case getenv("GITHUB_ACTIONS") == "true":
		return "github"
	case getenv("GITLAB_CI") == "true":
		return "gitlab"
	case getenv("CIRCLECI") == "true":
		return "circleci"
	}
	return ""
}

// NewTransport returns a RoundTripper that adds the User-Agent header to
// requests that don't already set one (even to ""), then sends them with base, or with
// http.DefaultTransport when base is nil.
func NewTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &transport{base: base}
}

type transport struct {
	base http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, set := req.Header["User-Agent"]; set {
		return t.base.RoundTrip(req)
	}
	// A RoundTripper must not modify the caller's request.
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", String())
	return t.base.RoundTrip(req)
}

// CloseIdleConnections lets http.Client.CloseIdleConnections reach base.
func (t *transport) CloseIdleConnections() {
	if c, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		c.CloseIdleConnections()
	}
}
