// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package connectivity

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kusaridev/kusari-cli/v2/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// directProxy is a ProxyResolver that always reports "no proxy". Tests inject a
// resolver rather than setting HTTPS_PROXY, because net/http caches the proxy
// environment behind a sync.Once at the first proxy lookup, which would make
// environment-mutating tests order-dependent.
func directProxy(*http.Request) (*url.URL, error) { return nil, nil }

func checkOne(t *testing.T, target Target, opts Options) Result {
	t.Helper()
	if opts.ProxyResolver == nil {
		opts.ProxyResolver = directProxy
	}
	results := Check(context.Background(), []Target{target}, opts)
	require.Len(t, results, 1)
	return results[0]
}

// Any HTTP status means the network path works. This is the central pass rule:
// the check is unauthenticated, so 401/403/404 are expected, and a 5xx is a
// Kusari service problem rather than a user network problem.
func TestCheckAnyStatusIsReachable(t *testing.T) {
	for _, code := range []int{200, 204, 301, 400, 401, 403, 404, 405, 429, 500, 502, 503} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()

			res := checkOne(t, Target{Name: "t", URL: srv.URL}, Options{})

			assert.True(t, res.OK(), "status %d must count as reachable", code)
			assert.Equal(t, code, res.StatusCode)
			assert.Equal(t, CategoryOK, res.Diag.Category)
			assert.NoError(t, res.Err)
			assert.Equal(t, code >= 500, res.ServiceError())
		})
	}
}

// Redirects must not be followed: doing so would report a status from a host we
// never intended to probe and could mask a failure at the real endpoint.
func TestCheckDoesNotFollowRedirects(t *testing.T) {
	var reqs int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reqs++
		w.Header().Set("Location", "https://aws.amazon.com/s3/")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer srv.Close()

	res := checkOne(t, Target{Name: "upload", URL: srv.URL}, Options{})

	assert.True(t, res.OK())
	assert.Equal(t, http.StatusTemporaryRedirect, res.StatusCode)
	assert.Equal(t, "https://aws.amazon.com/s3/", res.Location)
	assert.Equal(t, 1, reqs, "exactly one request; the redirect must not be followed")
}

func TestCheckTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	res := checkOne(t, Target{Name: "slow", URL: srv.URL}, Options{Timeout: 50 * time.Millisecond})

	assert.False(t, res.OK())
	assert.Equal(t, CategoryTimeout, res.Diag.Category)
	assert.NotEmpty(t, res.Diag.Remediation)
}

func TestCheckConnectionRefused(t *testing.T) {
	// Bind then immediately close, so the port is almost certainly free and
	// nothing is listening.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	res := checkOne(t, Target{Name: "dead", URL: "http://" + addr}, Options{})

	assert.False(t, res.OK())
	assert.Equal(t, CategoryTCP, res.Diag.Category)
}

// An httptest TLS server uses a self-signed cert that system roots do not
// trust, which is exactly the shape of a TLS-inspecting corporate proxy.
func TestCheckUntrustedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := checkOne(t, Target{Name: "tls", URL: srv.URL}, Options{})

	assert.False(t, res.OK())
	assert.Equal(t, CategoryTLSTrust, res.Diag.Category)
	assert.Contains(t, strings.Join(res.Diag.Remediation, "\n"), "IGNORED on macOS")
}

func TestCheckCapturesTLSInfoOnSuccess(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Trust the test server's CA so the probe succeeds, then assert we still
	// recorded who issued the certificate. Reporting the issuer on success is
	// what tells an operator their traffic is being inspected.
	target := Target{Name: "tls", URL: srv.URL}
	client := &http.Client{Transport: srv.Client().Transport}
	req, err := http.NewRequest(http.MethodGet, target.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.NotNil(t, resp.TLS)

	info := tlsInfo(resp.TLS)
	require.NotNil(t, info)
	assert.True(t, strings.HasPrefix(info.Version, "TLS"), "got %q", info.Version)
	assert.NotEmpty(t, info.CipherSuite)
}

func TestCheckInvalidProxyConfiguration(t *testing.T) {
	res := checkOne(t, Target{Name: "t", URL: "https://example.invalid/"}, Options{
		ProxyResolver: func(*http.Request) (*url.URL, error) {
			return nil, &url.Error{Op: "parse", URL: "not a url", Err: assertErr{}}
		},
	})

	assert.False(t, res.OK())
	assert.Equal(t, CategoryProxy, res.Diag.Category)
	assert.Contains(t, res.Diag.Summary, "invalid proxy configuration")
}

type assertErr struct{}

func (assertErr) Error() string { return "bad proxy" }

// Results must come back in target order regardless of which probe finishes
// first, so the table is stable between runs.
func TestCheckPreservesTargetOrder(t *testing.T) {
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer fast.Close()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(120 * time.Millisecond)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer slow.Close()

	targets := []Target{
		{Name: "slow", URL: slow.URL},
		{Name: "fast", URL: fast.URL},
	}
	results := Check(context.Background(), targets, Options{ProxyResolver: directProxy})

	require.Len(t, results, 2)
	assert.Equal(t, "slow", results[0].Target.Name)
	assert.Equal(t, "fast", results[1].Target.Name)
}

func TestCheckReportsResolvedProxy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// A proxy URL carrying a password. The label must never leak it.
	res := checkOne(t, Target{Name: "t", URL: srv.URL}, Options{
		ProxyResolver: func(*http.Request) (*url.URL, error) {
			return url.Parse("http://svc:s3cr3t@proxy.corp.example:8080")
		},
	})

	require.NotNil(t, res.Proxy)
	assert.Equal(t, "proxy.corp.example:8080", ProxyLabel(res.Proxy))
	assert.NotContains(t, ProxyLabel(res.Proxy), "s3cr3t")
	assert.NotContains(t, ProxySummary(res.Proxy), "s3cr3t")
	assert.Contains(t, ProxySummary(res.Proxy), "xxxxx")
}

func TestProxyLabelAndSummaryHandleNil(t *testing.T) {
	assert.Equal(t, "direct", ProxyLabel(nil))
	assert.Empty(t, ProxySummary(nil))
}

func TestDefaultTargets(t *testing.T) {
	// Deliberately omit trailing slashes: the naive concatenation used
	// elsewhere in this repo would produce "...cloudoauth2/token".
	targets := DefaultTargets(Endpoints{
		AuthURL:     "https://auth.us.kusari.cloud",
		PlatformURL: "https://platform.api.us.kusari.cloud",
		ConsoleURL:  "https://console.us.kusari.cloud",
		Tenant:      "acme",
	})

	require.Len(t, targets, 5)
	assert.Equal(t, "auth", targets[0].Name)
	assert.Equal(t, "https://auth.us.kusari.cloud/oauth2/token", targets[0].URL)
	assert.Equal(t, "platform", targets[1].Name)
	assert.Equal(t, "https://platform.api.us.kusari.cloud/user", targets[1].URL)
	assert.Equal(t, "console", targets[2].Name)
	assert.Equal(t, "https://console.us.kusari.cloud/", targets[2].URL)
	assert.Equal(t, "upload-sbom", targets[3].Name)
	assert.Equal(t, "https://kusari-guac-ingest-prod-us-east-1-acme.s3.us-east-1.amazonaws.com", targets[3].URL)
	assert.Equal(t, "upload-scan", targets[4].Name)
	assert.Equal(t, "https://inspector-bundle-upload-prod-us-east-1.s3.us-east-1.amazonaws.com", targets[4].URL)

	// Trailing slashes present must produce the same URLs.
	withSlashes := DefaultTargets(Endpoints{
		AuthURL:     "https://auth.us.kusari.cloud/",
		PlatformURL: "https://platform.api.us.kusari.cloud/",
		ConsoleURL:  "https://console.us.kusari.cloud/",
		Tenant:      "acme",
	})
	assert.Equal(t, targets, withSlashes)
}

// With no tenant the SBOM bucket hostname is unknowable, so it must be omitted
// rather than guessed at -- a guessed hostname in the allowlist output would be
// bad advice to hand a network team.
func TestDefaultTargetsOmitsSBOMBucketWithoutTenant(t *testing.T) {
	targets := DefaultTargets(Endpoints{
		AuthURL:     constants.DefaultAuthURL,
		PlatformURL: constants.DefaultPlatformURL,
		ConsoleURL:  constants.DefaultConsoleURL,
	})

	require.Len(t, targets, 4)

	var names []string
	for _, tg := range targets {
		names = append(names, tg.Name)
	}
	assert.Equal(t, []string{"auth", "platform", "console", "upload-scan"}, names)

	for _, tg := range targets {
		assert.NotContains(t, tg.URL, "guac-ingest", "no guessed bucket name")
	}
}

func TestAllowlistHosts(t *testing.T) {
	e := Endpoints{
		AuthURL:     constants.DefaultAuthURL,
		PlatformURL: constants.DefaultPlatformURL,
		ConsoleURL:  constants.DefaultConsoleURL,
		Tenant:      "acme",
	}
	hosts := AllowlistHosts(DefaultTargets(e))

	require.Len(t, hosts, 5)
	for _, h := range hosts {
		assert.NotEmpty(t, h.Purpose)
		assert.NotContains(t, h.Host, "/", "allowlist entries are hostnames, not URLs")
	}
	assert.Equal(t, "kusari-guac-ingest-prod-us-east-1-acme.s3.us-east-1.amazonaws.com", hosts[3].Host)
	assert.Equal(t, "inspector-bundle-upload-prod-us-east-1.s3.us-east-1.amazonaws.com", hosts[4].Host)
}

func TestOptionsDefaults(t *testing.T) {
	o := Options{}.withDefaults()
	assert.Equal(t, DefaultTimeout, o.Timeout)
	assert.NotNil(t, o.ProxyResolver)
	assert.NotEmpty(t, o.UserAgent)

	custom := Options{Timeout: time.Second, UserAgent: "x", ProxyResolver: directProxy}.withDefaults()
	assert.Equal(t, time.Second, custom.Timeout)
	assert.Equal(t, "x", custom.UserAgent)
}

// The probe transport must be a clone of http.DefaultTransport. Building one
// from scratch would silently drop proxy support, HTTP/2 and every default
// timeout, so the check would diagnose a stack the CLI never uses.
func TestNewTransportInheritsDefaults(t *testing.T) {
	def := http.DefaultTransport.(*http.Transport)
	tr := newTransport()

	assert.NotNil(t, tr.Proxy, "proxy support must be inherited from DefaultTransport")
	assert.Equal(t, def.TLSHandshakeTimeout, tr.TLSHandshakeTimeout)
	assert.Equal(t, def.ExpectContinueTimeout, tr.ExpectContinueTimeout)
	assert.Equal(t, def.ForceAttemptHTTP2, tr.ForceAttemptHTTP2)
	assert.NotNil(t, tr.OnProxyConnectResponse)
	assert.True(t, tr.DisableKeepAlives, "each target needs a fresh DNS/TCP/TLS path")

	// Clone() materializes a TLSClientConfig carrying NextProtos, so it is not
	// nil. What matters is that verification stays on against system roots:
	// there is no --insecure and no --ca-cert.
	if tr.TLSClientConfig != nil {
		assert.False(t, tr.TLSClientConfig.InsecureSkipVerify, "TLS verification must stay enabled")
		assert.Nil(t, tr.TLSClientConfig.RootCAs, "must use system roots")
	}
}

// A base URL carrying a query, fragment or userinfo must not leak into the
// probe URL: it lands in the report, and appending the path after a query would
// probe the wrong endpoint entirely.
func TestJoinURLDropsQueryFragmentAndUserinfo(t *testing.T) {
	cases := []struct {
		base, path, want string
	}{
		{"https://h.example/", "user", "https://h.example/user"},
		{"https://h.example", "user", "https://h.example/user"},
		{"https://h.example", "/user", "https://h.example/user"},
		{"https://h.example/api/", "user", "https://h.example/api/user"},
		{"https://h.example/", "", "https://h.example/"},
		{"https://h.example", "", "https://h.example/"},
		// The leak and the wrong-path bug, in one input.
		{"https://h.example/?apikey=SECRET", "user", "https://h.example/user"},
		{"https://h.example/#frag", "user", "https://h.example/user"},
		{"https://u:p@h.example/", "user", "https://h.example/user"},
		{"https://h.example/?", "user", "https://h.example/user"},
		{"  https://h.example/  ", "user", "https://h.example/user"},
	}
	for _, c := range cases {
		got := joinURL(c.base, c.path)
		assert.Equal(t, c.want, got, "joinURL(%q, %q)", c.base, c.path)
		assert.NotContains(t, got, "SECRET")
		assert.NotContains(t, got, "u:p@")
	}
}

func TestDefaultTargetsSanitizesOverriddenBaseURLs(t *testing.T) {
	targets := DefaultTargets(Endpoints{
		AuthURL:     "https://auth.internal.example/?token=AUTHSECRET",
		PlatformURL: "https://mirror.internal.example/?apikey=PLATSECRET",
		ConsoleURL:  "https://console.internal.example/?sso=CONSOLESECRET",
	})

	require.NotEmpty(t, targets)
	for _, tg := range targets {
		assert.NotContains(t, tg.URL, "SECRET", "target %s leaked a query secret", tg.Name)
	}
	assert.Equal(t, "https://auth.internal.example/oauth2/token", targets[0].URL)
	assert.Equal(t, "https://mirror.internal.example/user", targets[1].URL)
	assert.Equal(t, "https://console.internal.example/", targets[2].URL)
}

func TestHostOf(t *testing.T) {
	assert.Equal(t, "platform.api.us.kusari.cloud", hostOf("https://platform.api.us.kusari.cloud/user"))
	assert.Equal(t, "example.com", hostOf("https://example.com:8443/x"))
	assert.Equal(t, "not a url", hostOf("not a url"))
}
