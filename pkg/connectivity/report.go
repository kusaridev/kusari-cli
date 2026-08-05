// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package connectivity

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

// Report is the machine-readable result for a support ticket.
//
// It is written expecting to leave the machine, so it discloses as little about
// the environment as the diagnostics allow: no hostname, no proxy credentials or
// usernames, and no NO_PROXY contents. This command loads no tokens, so there is
// no auth material to leak either.
type Report struct {
	SystemInfo SystemInfo      `json:"systemInfo"`
	Tests      []TestResult    `json:"tests"`
	Allowlist  []AllowlistHost `json:"allowlist"`
	Summary    Summary         `json:"summary"`
}

// SystemInfo deliberately omits the hostname: corporate asset names commonly
// encode a person, department or site, and os/arch already cover the
// platform-specific diagnostics.
type SystemInfo struct {
	OS         string      `json:"os"`
	Arch       string      `json:"arch"`
	CLIVersion string      `json:"cliVersion"`
	Timestamp  string      `json:"timestamp"`
	Proxy      ProxyConfig `json:"proxy"`
}

// ProxyConfig records the proxy environment with passwords redacted. Use
// forReport before serializing.
type ProxyConfig struct {
	HTTP    string `json:"http"`
	HTTPS   string `json:"https"`
	NoProxy string `json:"noProxy"`
}

// forReport strips what the terminal may show but a shared file should not: the
// proxy userinfo (a service-account name is useful for spraying and phishing)
// and the NO_PROXY contents, which are an inventory of internal domains, CIDRs
// and hostnames. No diagnostic value is lost -- a per-target proxy of "direct"
// already identifies which endpoints NO_PROXY matched.
func (p ProxyConfig) forReport() ProxyConfig {
	return ProxyConfig{
		HTTP:    stripUserinfo(p.HTTP),
		HTTPS:   stripUserinfo(p.HTTPS),
		NoProxy: summarizeNoProxy(p.NoProxy),
	}
}

func stripUserinfo(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "(set, unparseable)"
	}
	u.User = nil
	return u.String()
}

func summarizeNoProxy(raw string) string {
	n := 0
	for _, e := range strings.Split(raw, ",") {
		if strings.TrimSpace(e) != "" {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("(set, %d entries)", n)
}

type TestResult struct {
	Name          string   `json:"name"`
	Purpose       string   `json:"purpose"`
	URL           string   `json:"url"`
	Status        string   `json:"status"`
	StatusCode    int      `json:"statusCode,omitempty"`
	LatencyMs     int64    `json:"latencyMs"`
	Category      Category `json:"category"`
	Proxy         string   `json:"proxy,omitempty"`
	Authenticated bool     `json:"authenticated"`
	Error         string   `json:"error,omitempty"`
	TLS           *TLSInfo `json:"tls,omitempty"`
}

type Summary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

// BuildReport assembles a Report; now is injected so callers control the stamp.
func BuildReport(results []Result, allowlist []AllowlistHost, cliVersion string, proxyEnv ProxyConfig, now time.Time) Report {
	rep := Report{
		SystemInfo: SystemInfo{
			OS:         runtime.GOOS,
			Arch:       runtime.GOARCH,
			CLIVersion: cliVersion,
			Timestamp:  now.UTC().Format(time.RFC3339),
			Proxy:      proxyEnv.forReport(),
		},
		Tests:     make([]TestResult, 0, len(results)),
		Allowlist: allowlist,
	}

	for _, r := range results {
		tr := TestResult{
			Name:       r.Target.Name,
			Purpose:    r.Target.Purpose,
			URL:        r.Target.URL,
			StatusCode: r.StatusCode,
			LatencyMs:  r.Duration.Milliseconds(),
			Category:   r.Diag.Category,
			Proxy:      ProxyLabel(r.Proxy),
			TLS:        r.TLS,
		}
		if r.OK() {
			tr.Status = "pass"
			rep.Summary.Passed++
		} else {
			tr.Status = "fail"
			tr.Error = r.Diag.Summary
			rep.Summary.Failed++
		}
		rep.Tests = append(rep.Tests, tr)
	}
	rep.Summary.Total = len(results)

	return rep
}

// WriteReport writes rep to path as indented JSON with owner-only permissions.
func WriteReport(path string, rep Report) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode report: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write report to %s: %w", path, err)
	}
	return nil
}

// ProxyEnvConfig reads the proxy environment, redacted. lookup is os.Getenv.
func ProxyEnvConfig(lookup func(string) string) ProxyConfig {
	return ProxyConfig{
		HTTP:    redactEnvProxy(firstEnv(lookup, "HTTP_PROXY", "http_proxy")),
		HTTPS:   redactEnvProxy(firstEnv(lookup, "HTTPS_PROXY", "https_proxy")),
		NoProxy: firstEnv(lookup, "NO_PROXY", "no_proxy"),
	}
}

func firstEnv(lookup func(string) string, names ...string) string {
	for _, n := range names {
		if v := lookup(n); v != "" {
			return v
		}
	}
	return ""
}

// redactEnvProxy hides any password. An unparseable value is replaced rather
// than echoed, since it may still contain credentials.
func redactEnvProxy(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "(set, unparseable)"
	}
	return u.Redacted()
}
