// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package connectivity

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

// wrapGet mimics how http.Client wraps transport errors.
func wrapGet(inner error) error {
	return &url.Error{Op: "Get", URL: "https://platform.api.us.kusari.cloud/user", Err: inner}
}

// dialErr builds the chain http.Client -> *url.Error -> *net.OpError{dial}.
func dialErr(inner error) error {
	return wrapGet(&net.OpError{Op: "dial", Net: "tcp", Err: inner})
}

// proxyConnectErr builds the chain net/http uses when tunnel setup fails:
// *url.Error -> *net.OpError{proxyconnect} -> inner.
func proxyConnectErr(inner error) error {
	return wrapGet(&net.OpError{Op: "proxyconnect", Net: "tcp", Err: inner})
}

func syscallErr(errno syscall.Errno) error {
	return os.NewSyscallError("connect", errno)
}

// certVerifyErr builds a *tls.CertificateVerificationError carrying a chain
// whose root names the given issuer, as a TLS-inspecting proxy would.
func certVerifyErr(issuer string, inner error) error {
	leaf := &x509.Certificate{
		Subject: pkix.Name{CommonName: "*.us.kusari.cloud"},
		Issuer:  pkix.Name{CommonName: issuer},
	}
	return wrapGet(&tls.CertificateVerificationError{
		UnverifiedCertificates: []*x509.Certificate{leaf},
		Err:                    inner,
	})
}

func TestClassify(t *testing.T) {
	proxy := mustURL(t, "http://proxy.corp.example:8080")

	cases := []struct {
		name         string
		cc           classifyCtx
		err          error
		wantCategory Category
		wantInCause  string
	}{
		{
			name:         "nil error is ok",
			err:          nil,
			wantCategory: CategoryOK,
		},
		{
			name:         "dns nxdomain",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          dialErr(&net.DNSError{Err: "no such host", Name: "platform.api.us.kusari.cloud", IsNotFound: true}),
			wantCategory: CategoryDNS,
			wantInCause:  "NXDOMAIN",
		},
		{
			name:         "dns timeout",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          dialErr(&net.DNSError{Err: "i/o timeout", Name: "platform.api.us.kusari.cloud", IsTimeout: true}),
			wantCategory: CategoryDNS,
			wantInCause:  "no DNS server answered",
		},
		{
			name:         "dns other",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          dialErr(&net.DNSError{Err: "server misbehaving", Name: "platform.api.us.kusari.cloud"}),
			wantCategory: CategoryDNS,
			wantInCause:  "server misbehaving",
		},
		{
			name:         "tcp connection refused",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          dialErr(syscallErr(syscall.ECONNREFUSED)),
			wantCategory: CategoryTCP,
			wantInCause:  "actively refused",
		},
		{
			name:         "tcp connection reset",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          dialErr(syscallErr(syscall.ECONNRESET)),
			wantCategory: CategoryTCP,
			wantInCause:  "was reset",
		},
		{
			name:         "tcp host unreachable",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          dialErr(syscallErr(syscall.EHOSTUNREACH)),
			wantCategory: CategoryTCP,
			wantInCause:  "no route",
		},
		{
			name:         "tcp network unreachable",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          dialErr(syscallErr(syscall.ENETUNREACH)),
			wantCategory: CategoryTCP,
			wantInCause:  "no route",
		},
		{
			name:         "tcp generic op error",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          dialErr(errors.New("some dial failure")),
			wantCategory: CategoryTCP,
			wantInCause:  "could not open a TCP connection",
		},
		{
			name:         "tls untrusted authority names the issuer",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud", Proxy: proxy},
			err:          certVerifyErr("ACME Corp TLS Inspection CA", x509.UnknownAuthorityError{}),
			wantCategory: CategoryTLSTrust,
			wantInCause:  `issuer: "ACME Corp TLS Inspection CA"`,
		},
		{
			name:         "bare unknown authority error",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          wrapGet(x509.UnknownAuthorityError{}),
			wantCategory: CategoryTLSTrust,
			wantInCause:  "does not trust",
		},
		{
			name:         "tls hostname mismatch",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          certVerifyErr("Some CA", x509.HostnameError{Host: "platform.api.us.kusari.cloud"}),
			wantCategory: CategoryTLSOther,
			wantInCause:  "issued for a different name",
		},
		{
			name:         "tls certificate expired",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          certVerifyErr("Some CA", x509.CertificateInvalidError{Reason: x509.Expired}),
			wantCategory: CategoryTLSOther,
			wantInCause:  "not usable",
		},
		{
			name:         "server did not speak tls",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          wrapGet(tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}),
			wantCategory: CategoryTLSOther,
			wantInCause:  "not TLS",
		},
		{
			name:         "context deadline exceeded",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud", Proxy: proxy},
			err:          wrapGet(context.DeadlineExceeded),
			wantCategory: CategoryTimeout,
			wantInCause:  "did not respond before the timeout",
		},
		{
			name:         "os deadline exceeded",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          wrapGet(os.ErrDeadlineExceeded),
			wantCategory: CategoryTimeout,
			wantInCause:  "did not respond before the timeout",
		},
		{
			name:         "unknown error",
			cc:           classifyCtx{Host: "platform.api.us.kusari.cloud"},
			err:          wrapGet(errors.New("something exotic")),
			wantCategory: CategoryUnknown,
			wantInCause:  "something exotic",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.cc, c.err)
			assert.Equal(t, c.wantCategory, got.Category)
			if c.wantInCause != "" {
				assert.Contains(t, got.Cause, c.wantInCause)
			}
			if c.wantCategory != CategoryOK {
				assert.NotEmpty(t, got.Summary, "every failure needs a summary")
				assert.NotEmpty(t, got.Remediation, "every failure needs at least one remediation step")
				assert.NotEmpty(t, got.Raw, "raw error must be retained for --verbose")
			}
		})
	}
}

// A proxyconnect failure wraps exactly the same error types as a direct
// failure. If ordering regressed, these would be reported as dns/tcp against
// the Kusari hostname -- sending the user to fix the wrong thing.
func TestClassifyProxyConnectTakesPrecedence(t *testing.T) {
	proxy := mustURL(t, "http://proxy.corp.example:8080")
	cc := classifyCtx{Host: "platform.api.us.kusari.cloud", Proxy: proxy}

	t.Run("unresolvable proxy is proxy, not dns", func(t *testing.T) {
		err := proxyConnectErr(&net.OpError{Op: "dial", Net: "tcp",
			Err: &net.DNSError{Err: "no such host", Name: "proxy.corp.example", IsNotFound: true}})

		got := Classify(cc, err)
		assert.Equal(t, CategoryProxy, got.Category)
		assert.Contains(t, got.Cause, "proxy.corp.example")
		assert.Contains(t, got.Cause, "is the PROXY hostname")
		// It must not blame the Kusari endpoint.
		assert.NotContains(t, got.Summary, "platform.api.us.kusari.cloud")
	})

	t.Run("refused proxy is proxy, not tcp", func(t *testing.T) {
		err := proxyConnectErr(&net.OpError{Op: "dial", Net: "tcp", Err: syscallErr(syscall.ECONNREFUSED)})

		got := Classify(cc, err)
		assert.Equal(t, CategoryProxy, got.Category)
		assert.Contains(t, got.Cause, "this is your PROXY")
	})

	t.Run("untrusted https proxy cert is tls-trust with proxy context", func(t *testing.T) {
		err := wrapGet(&net.OpError{Op: "proxyconnect", Net: "tcp",
			Err: &tls.CertificateVerificationError{
				UnverifiedCertificates: []*x509.Certificate{{
					Subject: pkix.Name{CommonName: "proxy.corp.example"},
					Issuer:  pkix.Name{CommonName: "Corp Proxy Root"},
				}},
				Err: x509.UnknownAuthorityError{},
			}})

		got := Classify(cc, err)
		assert.Equal(t, CategoryTLSTrust, got.Category)
		assert.Contains(t, got.Cause, "proxy.corp.example:8080")
		assert.Contains(t, got.Cause, `issuer: "Corp Proxy Root"`)
	})
}

func TestClassifyProxyRejectedConnect(t *testing.T) {
	cc := classifyCtx{Host: "platform.api.us.kusari.cloud",
		Proxy: mustURL(t, "http://proxy.corp.example:8080")}

	t.Run("407 explains proxy auth", func(t *testing.T) {
		err := wrapGet(&ProxyConnectError{
			ProxyHost:   "proxy.corp.example:8080",
			StatusCode:  http.StatusProxyAuthRequired,
			Status:      "407 Proxy Authentication Required",
			AuthSchemes: []string{"Negotiate", "Basic realm=\"corp\""},
		})

		got := Classify(cc, err)
		assert.Equal(t, CategoryProxy, got.Category)
		assert.Contains(t, got.Summary, "407")
		assert.Contains(t, got.Cause, "Proxy-Authenticate")
		assert.Contains(t, strings.Join(got.Remediation, "\n"), "requires authentication")
	})

	t.Run("403 explains allowlist", func(t *testing.T) {
		err := wrapGet(&ProxyConnectError{
			ProxyHost:  "proxy.corp.example:8080",
			StatusCode: http.StatusForbidden,
			Status:     "403 Forbidden",
		})

		got := Classify(cc, err)
		assert.Equal(t, CategoryProxy, got.Category)
		assert.Contains(t, strings.Join(got.Remediation, "\n"), "allow CONNECT")
	})
}

func TestProxyConnectResponseHook(t *testing.T) {
	proxy := mustURL(t, "http://proxy.corp.example:8080")

	t.Run("2xx is not an error", func(t *testing.T) {
		err := proxyConnectResponse(context.Background(), proxy, nil,
			&http.Response{StatusCode: 200, Status: "200 OK", Header: http.Header{}})
		assert.NoError(t, err)
	})

	t.Run("407 becomes a typed error carrying the status", func(t *testing.T) {
		res := &http.Response{
			StatusCode: 407,
			Status:     "407 Proxy Authentication Required",
			Header:     http.Header{"Proxy-Authenticate": []string{"Basic"}},
		}
		err := proxyConnectResponse(context.Background(), proxy, nil, res)
		require.Error(t, err)

		var pce *ProxyConnectError
		require.ErrorAs(t, err, &pce)
		assert.Equal(t, 407, pce.StatusCode)
		assert.Equal(t, "proxy.corp.example:8080", pce.ProxyHost)
		assert.Equal(t, []string{"Basic"}, pce.AuthSchemes)
	})
}

func TestTrustRemediationIsPlatformSpecific(t *testing.T) {
	fixes := strings.Join(trustRemediation("Corp CA", classifyCtx{Host: "h"}, "p:8080"), "\n")

	assert.Contains(t, fixes, `"Corp CA"`)
	// The macOS caveat is the whole reason this text exists: SSL_CERT_FILE is
	// silently ignored there, so telling a Mac user to set it would be wrong.
	assert.Contains(t, fixes, "IGNORED on macOS")
	assert.Contains(t, fixes, "update-ca-certificates")
	assert.Contains(t, fixes, "NO_PROXY")
	assert.Contains(t, fixes, "no --insecure")
}

func TestIssuerCNFallsBackToOrganization(t *testing.T) {
	cve := &tls.CertificateVerificationError{
		UnverifiedCertificates: []*x509.Certificate{{
			Issuer: pkix.Name{Organization: []string{"Example Corp"}},
		}},
	}
	assert.Equal(t, "Example Corp", issuerCN(cve))

	assert.Empty(t, issuerCN(nil))
	assert.Empty(t, issuerCN(&tls.CertificateVerificationError{}))
}
