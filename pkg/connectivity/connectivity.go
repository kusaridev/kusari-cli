// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

// Package connectivity probes the network path between this machine and the
// endpoints the Kusari CLI needs.
//
// Proxy configuration comes only from HTTP_PROXY/HTTPS_PROXY/NO_PROXY via
// http.ProxyFromEnvironment, so a passing check reflects what every other
// client in this CLI will do. There is deliberately no proxy flag.
package connectivity

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kusaridev/kusari-cli/v2/pkg/constants"
)

const (
	DefaultTimeout = 10 * time.Second
	SupportEmail   = "support@kusari.cloud"

	maxBodyRead = 4 << 10
)

// Target is one endpoint to probe.
type Target struct {
	Name    string
	Purpose string
	URL     string
}

// TLSInfo records what the peer presented. Captured on success too: it reveals
// a trusted interception CA that would otherwise go unnoticed.
type TLSInfo struct {
	Version       string `json:"version"`
	CipherSuite   string `json:"cipherSuite"`
	PeerSubjectCN string `json:"peerSubjectCN,omitempty"`
	PeerIssuerCN  string `json:"peerIssuerCN,omitempty"`
}

// Result is the outcome of probing one Target.
type Result struct {
	Target     Target
	Proxy      *url.URL
	StatusCode int
	Status     string
	Location   string
	Duration   time.Duration
	TLS        *TLSInfo
	Diag       Diagnosis
	Err        error
}

// OK reports whether the endpoint is reachable: the TLS handshake completed and
// an HTTP status came back. 401, 403, 404, 3xx and 5xx all count.
func (r Result) OK() bool { return r.Err == nil }

// ServiceError reports a 5xx: reachable, but the fault is Kusari's rather than
// the user's network.
func (r Result) ServiceError() bool { return r.OK() && r.StatusCode >= 500 }

// AllowlistHost is a hostname a firewall or proxy allowlist needs.
type AllowlistHost struct {
	Host    string `json:"host"`
	Purpose string `json:"purpose"`
}

// Endpoints holds the resolved base URLs to probe.
type Endpoints struct {
	AuthURL     string
	PlatformURL string
	ConsoleURL  string
	Tenant      string
}

// Options configures a Check run.
type Options struct {
	Timeout time.Duration
	// ProxyResolver defaults to http.ProxyFromEnvironment. Injectable because
	// net/http caches the proxy env behind a sync.Once, which makes
	// env-mutating tests order-dependent.
	ProxyResolver func(*http.Request) (*url.URL, error)
	UserAgent     string
}

func (o Options) withDefaults() Options {
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.ProxyResolver == nil {
		o.ProxyResolver = http.ProxyFromEnvironment
	}
	if o.UserAgent == "" {
		o.UserAgent = "kusari-cli/connectivity-check"
	}
	return o
}

// DefaultTargets returns the endpoints to probe, in display order.
//
// Only one S3 bucket hostname is probed: S3 uses wildcard DNS and serves the
// same *.s3.<region>.amazonaws.com certificate for every bucket, so DNS, TCP,
// TLS and proxy egress verified against one bucket hold for all of them. The
// tenant-specific SBOM bucket still appears in AllowlistHosts for proxies that
// filter by exact hostname.
func DefaultTargets(e Endpoints) []Target {
	return []Target{
		{
			Name:    "auth",
			Purpose: "Authentication service (OAuth2)",
			URL:     joinURL(e.AuthURL, "oauth2/token"),
		},
		{
			Name:    "platform",
			Purpose: "Platform API",
			URL:     joinURL(e.PlatformURL, "user"),
		},
		{
			Name:    "console",
			Purpose: "Console UI (web application)",
			URL:     joinURL(e.ConsoleURL, ""),
		},
		{
			Name:    "upload",
			Purpose: "Artifact upload (AWS S3)",
			URL: "https://" + fmt.Sprintf(constants.InspectorUploadHostPattern,
				constants.DefaultUploadEnv, constants.DefaultS3Region, constants.DefaultS3Region),
		},
	}
}

// AllowlistHosts returns every hostname to allow through a firewall or proxy:
// the probed targets plus the tenant-specific SBOM upload bucket, which is not
// probed (see DefaultTargets) but still needed by proxies that filter on exact
// hostnames. When no tenant is known, a <tenant> placeholder is rendered.
func AllowlistHosts(e Endpoints, targets []Target) []AllowlistHost {
	hosts := make([]AllowlistHost, 0, len(targets)+1)
	for _, t := range targets {
		hosts = append(hosts, AllowlistHost{Host: hostOf(t.URL), Purpose: t.Purpose})
	}

	tenant := e.Tenant
	if tenant == "" {
		tenant = "<tenant>"
	}
	return append(hosts, AllowlistHost{
		Host: fmt.Sprintf(constants.SBOMUploadHostPattern,
			constants.DefaultUploadEnv, constants.DefaultS3Region, tenant, constants.DefaultS3Region),
		Purpose: "SBOM artifact upload (AWS S3)",
	})
}

// Check probes every target concurrently and returns results in target order.
// Every failure is reported as a Result, so Check returns no error.
func Check(ctx context.Context, targets []Target, opts Options) []Result {
	opts = opts.withDefaults()

	client := &http.Client{
		Transport: newTransport(),
		// Never follow redirects: doing so would report a status from a host we
		// did not intend to probe and could mask a failure at the real endpoint.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()

	results := make([]Result, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t Target) {
			defer wg.Done()
			results[i] = probe(ctx, client, opts, t)
		}(i, t)
	}
	wg.Wait()
	return results
}

// newTransport clones http.DefaultTransport so probes negotiate exactly what
// the rest of the CLI negotiates. A hand-built &http.Transport{} would silently
// lose proxy support, HTTP/2 and every default timeout.
func newTransport() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()

	// net/http discards the CONNECT status code and returns only the reason
	// phrase, so capture the response to turn a 407 into a typed error.
	tr.OnProxyConnectResponse = proxyConnectResponse

	// Each target needs a fresh DNS -> TCP -> TLS path.
	tr.DisableKeepAlives = true

	// TLSClientConfig stays nil: system roots, verification on.
	return tr
}

func probe(ctx context.Context, client *http.Client, opts Options, t Target) Result {
	res := Result{Target: t}
	cc := classifyCtx{Host: hostOf(t.URL)}

	// Resolve the proxy first so a failure can still name it.
	proxy, err := resolveProxy(opts.ProxyResolver, t.URL)
	if err != nil {
		res.Err = err
		res.Diag = Diagnosis{
			Category: CategoryProxy,
			Summary:  "invalid proxy configuration",
			Cause: fmt.Sprintf("the proxy environment variable that applies to %s is not usable: %v",
				t.URL, err),
			Remediation: []string{
				"HTTP_PROXY / HTTPS_PROXY must be a URL such as http://host:port.",
			},
			Raw: err.Error(),
		}
		return res
	}
	res.Proxy = proxy
	cc.Proxy = proxy

	// A context deadline surfaces as context.DeadlineExceeded, which Classify
	// can match, and it honours cancellation of the parent context.
	rctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(rctx, http.MethodGet, t.URL, nil)
	if err != nil {
		res.Err = err
		res.Diag = Diagnosis{
			Category: CategoryUnknown,
			Summary:  "invalid target URL",
			Cause:    fmt.Sprintf("%q is not a valid URL: %v", t.URL, err),
			Raw:      err.Error(),
		}
		return res
	}
	req.Header.Set("User-Agent", opts.UserAgent)

	start := time.Now()
	resp, err := client.Do(req)
	res.Duration = time.Since(start)
	if err != nil {
		res.Err = err
		res.Diag = Classify(cc, err)
		return res
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyRead))

	res.StatusCode = resp.StatusCode
	res.Status = resp.Status
	res.Location = resp.Header.Get("Location")
	if resp.TLS != nil {
		res.TLS = tlsInfo(resp.TLS)
	}
	res.Diag = Diagnosis{Category: CategoryOK}
	return res
}

// resolveProxy reports which proxy the resolver picks for rawURL. Using the
// same function the transport uses guarantees the reported proxy is the one
// actually used, including NO_PROXY and loopback rules.
func resolveProxy(resolver func(*http.Request) (*url.URL, error), rawURL string) (*url.URL, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return resolver(req)
}

func tlsInfo(cs *tls.ConnectionState) *TLSInfo {
	i := &TLSInfo{
		Version:     tls.VersionName(cs.Version),
		CipherSuite: tls.CipherSuiteName(cs.CipherSuite),
	}
	if len(cs.PeerCertificates) > 0 {
		leaf := cs.PeerCertificates[0]
		i.PeerSubjectCN = leaf.Subject.CommonName
		i.PeerIssuerCN = leaf.Issuer.CommonName
	}
	return i
}

// joinURL appends path to base with exactly one separating slash, dropping any
// query or fragment on base. Naive concatenation would append after the query,
// probing the wrong path, and would copy a credential-bearing base URL into the
// report.
func joinURL(base, path string) string {
	base = strings.TrimSpace(base)
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")
	}
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.User = nil
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	return u.String()
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Hostname()
}

// ProxyLabel renders a proxy as host only, never credentials.
func ProxyLabel(u *url.URL) string {
	if u == nil {
		return "direct"
	}
	return u.Host
}

// ProxySummary renders a proxy URL with any password replaced.
func ProxySummary(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Redacted()
}
