// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package connectivity

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixedTime() time.Time {
	return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
}

func sampleResults(t *testing.T) []Result {
	t.Helper()
	proxy, err := url.Parse("http://svc:s3cr3t@proxy.corp.example:8080")
	require.NoError(t, err)

	return []Result{
		{
			Target:     Target{Name: "platform", Purpose: "Platform API", URL: "https://platform.api.us.kusari.cloud/user"},
			Proxy:      proxy,
			StatusCode: 401,
			Status:     "401 Unauthorized",
			Duration:   266 * time.Millisecond,
			TLS:        &TLSInfo{Version: "TLS 1.3", CipherSuite: "TLS_AES_128_GCM_SHA256"},
			Diag:       Diagnosis{Category: CategoryOK},
		},
		{
			Target:   Target{Name: "auth", Purpose: "Authentication service (OAuth2)", URL: "https://auth.us.kusari.cloud/oauth2/token"},
			Proxy:    proxy,
			Duration: 1204 * time.Millisecond,
			Diag: Diagnosis{
				Category: CategoryTLSTrust,
				Summary:  "TLS certificate not trusted",
				Raw:      "x509: certificate signed by unknown authority",
			},
			Err: assertErr{},
		},
	}
}

func TestBuildReport(t *testing.T) {
	results := sampleResults(t)
	allowlist := []AllowlistHost{{Host: "h", Purpose: "p"}}
	proxyEnv := ProxyConfig{HTTPS: "http://svc:xxxxx@proxy.corp.example:8080", NoProxy: "localhost"}

	rep := BuildReport(results, allowlist, "v2.1.0", proxyEnv, fixedTime())

	assert.Equal(t, "v2.1.0", rep.SystemInfo.CLIVersion)
	assert.Equal(t, "2026-08-04T12:00:00Z", rep.SystemInfo.Timestamp)
	assert.NotEmpty(t, rep.SystemInfo.OS)
	assert.NotEmpty(t, rep.SystemInfo.Arch)
	assert.Equal(t, allowlist, rep.Allowlist)

	// BuildReport must redact on the way in, so a caller cannot serialize the
	// raw environment by accident.
	assert.Equal(t, "http://proxy.corp.example:8080", rep.SystemInfo.Proxy.HTTPS)
	assert.Equal(t, "(set, 1 entries)", rep.SystemInfo.Proxy.NoProxy)

	assert.Equal(t, Summary{Total: 2, Passed: 1, Failed: 1}, rep.Summary)

	require.Len(t, rep.Tests, 2)
	assert.Equal(t, "pass", rep.Tests[0].Status)
	assert.Equal(t, 401, rep.Tests[0].StatusCode)
	assert.Equal(t, int64(266), rep.Tests[0].LatencyMs)
	assert.Equal(t, CategoryOK, rep.Tests[0].Category)
	assert.Empty(t, rep.Tests[0].Error)
	assert.False(t, rep.Tests[0].Authenticated, "this command never authenticates")

	assert.Equal(t, "fail", rep.Tests[1].Status)
	assert.Equal(t, CategoryTLSTrust, rep.Tests[1].Category)
	assert.Equal(t, "TLS certificate not trusted", rep.Tests[1].Error)
}

// The report is written expecting to leave the machine, so it must disclose no
// credentials, no service-account name, no hostname, and no internal network
// inventory. Everything asserted absent here is real-world sensitive: a proxy
// service account is a spraying/phishing target, an asset hostname commonly
// encodes a person or department, and NO_PROXY is an inventory of internal
// domains, CIDRs and hosts.
func TestReportDisclosesNoEnvironmentDetail(t *testing.T) {
	// The NO_PROXY domain suffix is deliberately NOT a substring of the proxy
	// hostname, which is retained; otherwise the assertions below contradict
	// each other.
	env := map[string]string{
		"HTTPS_PROXY": "http://svc-kusari:s3cr3t@proxy.emea.corp.example.com:8080",
		"NO_PROXY": "localhost,.intranet.example.net,10.0.0.0/8," +
			"vault.prod.internal,sap-prod-db01.fin.internal",
	}
	proxyEnv := ProxyEnvConfig(func(k string) string { return env[k] })

	rep := BuildReport(sampleResults(t), nil, "v2.1.0", proxyEnv, fixedTime())
	data, err := json.Marshal(rep)
	require.NoError(t, err)
	out := string(data)

	for _, secret := range []string{
		"s3cr3t",                     // password
		"svc-kusari",                 // service-account name
		"vault.prod.internal",        // internal hostname
		"sap-prod-db01.fin.internal", // internal hostname
		"10.0.0.0/8",                 // internal CIDR
		".intranet.example.net",      // internal domain suffix
	} {
		assert.NotContains(t, out, secret)
	}

	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		assert.NotContains(t, out, hostname, "the report must not carry the machine hostname")
	}

	// The proxy host:port is kept -- it is the diagnostic -- and NO_PROXY is
	// reduced to the fact that it is set.
	assert.Contains(t, out, "proxy.emea.corp.example.com:8080")
	assert.Contains(t, out, "(set, 5 entries)")
}

func TestProxyConfigForReport(t *testing.T) {
	cases := []struct {
		name string
		in   ProxyConfig
		want ProxyConfig
	}{
		{"empty stays empty", ProxyConfig{}, ProxyConfig{}},
		{
			"userinfo stripped entirely, not just the password",
			ProxyConfig{HTTPS: "http://svc:xxxxx@h:8080"},
			ProxyConfig{HTTPS: "http://h:8080"},
		},
		{
			"host-only proxy is untouched",
			ProxyConfig{HTTP: "http://h:8080"},
			ProxyConfig{HTTP: "http://h:8080"},
		},
		{
			"no_proxy reduced to a count",
			ProxyConfig{NoProxy: "localhost, .corp.example, 10.0.0.0/8"},
			ProxyConfig{NoProxy: "(set, 3 entries)"},
		},
		{"no_proxy of only separators counts as unset", ProxyConfig{NoProxy: " , , "}, ProxyConfig{}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.in.forReport())
		})
	}
}

func TestWriteReportRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	want := BuildReport(sampleResults(t), []AllowlistHost{{Host: "h", Purpose: "p"}},
		"v2.1.0", ProxyConfig{}, fixedTime())

	require.NoError(t, WriteReport(path, want))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got Report
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, want, got)
}

func TestWriteReportRejectsBadPath(t *testing.T) {
	err := WriteReport(filepath.Join(t.TempDir(), "missing-dir", "r.json"), Report{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write report")
}

func TestProxyEnvConfig(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want ProxyConfig
	}{
		{
			name: "unset",
			env:  map[string]string{},
			want: ProxyConfig{},
		},
		{
			name: "uppercase wins over lowercase",
			env: map[string]string{
				"HTTPS_PROXY": "http://upper:8080",
				"https_proxy": "http://lower:8080",
			},
			want: ProxyConfig{HTTPS: "http://upper:8080"},
		},
		{
			name: "lowercase used when uppercase unset",
			env:  map[string]string{"https_proxy": "http://lower:8080"},
			want: ProxyConfig{HTTPS: "http://lower:8080"},
		},
		{
			name: "password redacted",
			env:  map[string]string{"HTTP_PROXY": "http://u:p@h:8080"},
			want: ProxyConfig{HTTP: "http://u:xxxxx@h:8080"},
		},
		{
			// An unparseable value may still contain credentials, so we must not
			// echo it back verbatim.
			name: "unparseable value is not echoed",
			env:  map[string]string{"HTTPS_PROXY": "::not a url::"},
			want: ProxyConfig{HTTPS: "(set, unparseable)"},
		},
		{
			name: "no_proxy passes through",
			env:  map[string]string{"NO_PROXY": "localhost,.corp.example"},
			want: ProxyConfig{NoProxy: "localhost,.corp.example"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ProxyEnvConfig(func(k string) string { return c.env[k] })
			assert.Equal(t, c.want, got)
		})
	}
}
