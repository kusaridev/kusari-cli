// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package connectivity

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
)

// Category is a stable, machine-readable classification of a probe outcome.
type Category string

const (
	CategoryOK       Category = "ok"
	CategoryDNS      Category = "dns"
	CategoryTCP      Category = "tcp"
	CategoryTLSTrust Category = "tls-trust"
	CategoryTLSOther Category = "tls-other"
	CategoryProxy    Category = "proxy"
	CategoryTimeout  Category = "timeout"
	CategoryUnknown  Category = "unknown"
)

// Diagnosis explains a probe failure in terms the user can act on.
type Diagnosis struct {
	Category    Category `json:"category"`
	Summary     string   `json:"summary"`
	Cause       string   `json:"cause,omitempty"`
	Remediation []string `json:"remediation,omitempty"`
	Raw         string   `json:"raw,omitempty"`
}

// ProxyConnectError reports a proxy that rejected CONNECT. net/http discards
// the status code and returns only the reason phrase, so we capture it here.
type ProxyConnectError struct {
	ProxyHost   string
	StatusCode  int
	Status      string
	AuthSchemes []string
}

func (e *ProxyConnectError) Error() string {
	return fmt.Sprintf("proxy %s rejected CONNECT: %s", e.ProxyHost, e.Status)
}

// proxyConnectResponse runs before net/http's own 200-OK check, so an error
// here only affects responses net/http was already going to reject.
func proxyConnectResponse(_ context.Context, proxyURL *url.URL, _ *http.Request, res *http.Response) error {
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	host := ""
	if proxyURL != nil {
		host = proxyURL.Host
	}
	return &ProxyConnectError{
		ProxyHost:   host,
		StatusCode:  res.StatusCode,
		Status:      res.Status,
		AuthSchemes: res.Header.Values("Proxy-Authenticate"),
	}
}

// classifyCtx carries what Classify needs to write an accurate message. Proxy
// is nil for a direct connection.
type classifyCtx struct {
	Host  string
	Proxy *url.URL
}

// Classify turns a transport error into an actionable Diagnosis.
//
// Ordering matters: a proxyconnect failure wraps the same *net.DNSError and
// *net.OpError as a direct failure, so it must match first or an unresolvable
// proxy gets reported as an unresolvable Kusari endpoint.
func Classify(cc classifyCtx, err error) Diagnosis {
	if err == nil {
		return Diagnosis{Category: CategoryOK}
	}

	// Proxy rejected CONNECT (407 and friends).
	var pce *ProxyConnectError
	if errors.As(err, &pce) {
		return proxyRejectedDiagnosis(cc, pce, err)
	}

	// Failed while establishing the proxy tunnel.
	if isProxyConnectOp(err) {
		return proxyConnectDiagnosis(cc, err)
	}

	// Explicit deadlines here; *net.DNSError.IsTimeout stays with DNS below.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return timeoutDiagnosis(cc, err)
	}

	if d, ok := tlsDiagnosis(cc, err); ok {
		return d
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsDiagnosis(cc, dnsErr, err)
	}

	if d, ok := tcpDiagnosis(cc, err); ok {
		return d
	}

	// Remaining timeouts, e.g. net/http's unexported tlsHandshakeTimeoutError.
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return timeoutDiagnosis(cc, err)
	}

	return Diagnosis{
		Category: CategoryUnknown,
		Summary:  "connection failed",
		Cause:    fmt.Sprintf("could not reach %s: %v", cc.Host, err),
		Remediation: []string{
			"Re-run with --verbose to see the raw transport error.",
			"Send the output of `kusari connectivity check --report report.json` to " + SupportEmail + ".",
		},
		Raw: err.Error(),
	}
}

// isProxyConnectOp looks for any *net.OpError with Op == "proxyconnect".
func isProxyConnectOp(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		var opErr *net.OpError
		if errors.As(e, &opErr) {
			if opErr.Op == "proxyconnect" {
				return true
			}
			// Unwrap from inside the match, else errors.As keeps
			// returning the same outermost OpError.
			e = opErr
		}
	}
	return false
}

func proxyRejectedDiagnosis(cc classifyCtx, pce *ProxyConnectError, err error) Diagnosis {
	d := Diagnosis{
		Category: CategoryProxy,
		Summary:  fmt.Sprintf("proxy rejected the connection (HTTP %d)", pce.StatusCode),
		Cause: fmt.Sprintf("proxy %q returned %q for CONNECT %s:443. "+
			"The request never reached Kusari.", pce.ProxyHost, pce.Status, cc.Host),
		Raw: err.Error(),
	}
	if pce.StatusCode == http.StatusProxyAuthRequired {
		d.Remediation = []string{
			"The proxy requires authentication. Include credentials in the proxy URL, " +
				"e.g. HTTPS_PROXY=http://user:password@" + pce.ProxyHost,
			"If your proxy uses Kerberos/NTLM, Go cannot negotiate it directly -- " +
				"run a local authenticating proxy (cntlm, px) and point HTTPS_PROXY at that.",
		}
		if len(pce.AuthSchemes) > 0 {
			d.Cause += fmt.Sprintf(" Proxy-Authenticate: %v.", pce.AuthSchemes)
		}
	} else {
		d.Remediation = []string{
			fmt.Sprintf("Ask your network team to allow CONNECT to %s:443 through %s.", cc.Host, pce.ProxyHost),
			fmt.Sprintf("Confirm %s is spelled correctly in your proxy allowlist -- use the "+
				"hostname, not an IP address.", cc.Host),
		}
	}
	return d
}

func proxyConnectDiagnosis(cc classifyCtx, err error) Diagnosis {
	proxyHost := "the configured proxy"
	if cc.Proxy != nil {
		proxyHost = cc.Proxy.Host
	}

	d := Diagnosis{
		Category: CategoryProxy,
		Summary:  "cannot reach the configured proxy",
		Raw:      err.Error(),
	}

	// Be explicit that the hostname at fault is the proxy's, not Kusari's.
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr):
		d.Cause = fmt.Sprintf("the proxy hostname %q does not resolve (%s). "+
			"NOTE: %q is the PROXY hostname from your environment, not the Kusari endpoint.",
			dnsErr.Name, dnsErr.Err, dnsErr.Name)
	case isConnRefused(err):
		d.Cause = fmt.Sprintf("proxy %s refused the connection. "+
			"NOTE: this is your PROXY, not the Kusari endpoint.", proxyHost)
	default:
		var cve *tls.CertificateVerificationError
		if errors.As(err, &cve) {
			d.Category = CategoryTLSTrust
			d.Summary = "the proxy's own TLS certificate is not trusted"
			d.Cause = fmt.Sprintf("proxy %s speaks HTTPS but presented a certificate "+
				"this machine does not trust (%s).", proxyHost, issuerOf(cve))
			d.Remediation = trustRemediation(issuerCN(cve), cc, proxyHost)
			return d
		}
		d.Cause = fmt.Sprintf("failed to establish a tunnel through proxy %s to %s:443: %v. "+
			"NOTE: the failure is at the PROXY, not at the Kusari endpoint.", proxyHost, cc.Host, err)
	}

	d.Remediation = []string{
		fmt.Sprintf("Verify the proxy address %s is correct and reachable from this machine.", proxyHost),
		"Check HTTP_PROXY / HTTPS_PROXY for typos; the value must be a URL such as http://host:port.",
		fmt.Sprintf("If %s should bypass the proxy, add it to NO_PROXY.", cc.Host),
	}
	return d
}

func tlsDiagnosis(cc classifyCtx, err error) (Diagnosis, bool) {
	var cve *tls.CertificateVerificationError
	if !errors.As(err, &cve) {
		// Certificate problems can also surface as bare x509 errors.
		var ua x509.UnknownAuthorityError
		if errors.As(err, &ua) {
			return Diagnosis{
				Category:    CategoryTLSTrust,
				Summary:     "TLS certificate not trusted",
				Cause:       fmt.Sprintf("%s presented a certificate chain terminating in a CA this machine does not trust.", cc.Host),
				Remediation: trustRemediation("", cc, ""),
				Raw:         err.Error(),
			}, true
		}
		var rhe tls.RecordHeaderError
		if errors.As(err, &rhe) {
			return Diagnosis{
				Category: CategoryTLSOther,
				Summary:  "the server did not speak TLS",
				Cause: fmt.Sprintf("%s answered the TLS handshake with something that is not TLS. "+
					"A captive portal or a proxy that expects plain HTTP on 443 will do this.", cc.Host),
				Remediation: []string{
					"Confirm you are not behind a captive portal (open a browser and complete any sign-in).",
					"Confirm the proxy, if any, supports HTTPS CONNECT tunnelling.",
				},
				Raw: err.Error(),
			}, true
		}
		return Diagnosis{}, false
	}

	// Untrusted root (the corporate-proxy signature) needs a different fix
	// than a hostname or validity problem.
	var (
		ua  x509.UnknownAuthorityError
		he  x509.HostnameError
		cie x509.CertificateInvalidError
	)
	switch {
	case errors.As(cve.Err, &ua), errors.As(err, &ua):
		return Diagnosis{
			Category: CategoryTLSTrust,
			Summary:  "TLS certificate not trusted",
			Cause: fmt.Sprintf("%s presented a certificate chain that terminates in a CA this "+
				"machine does not trust (%s). This is the signature of a TLS-inspecting proxy "+
				"re-signing traffic.", cc.Host, issuerOf(cve)),
			Remediation: trustRemediation(issuerCN(cve), cc, proxyHostOf(cc)),
			Raw:         err.Error(),
		}, true

	case errors.As(cve.Err, &he), errors.As(err, &he):
		return Diagnosis{
			Category: CategoryTLSOther,
			Summary:  "TLS certificate is not valid for this hostname",
			Cause: fmt.Sprintf("the certificate presented for %s is issued for a different name (%s).",
				cc.Host, cve.Err),
			Remediation: []string{
				fmt.Sprintf("Confirm %s is correct and that no proxy or DNS entry is redirecting it elsewhere.", cc.Host),
				"If a TLS-inspecting proxy is in the path, its certificate must carry the correct SAN for this host.",
			},
			Raw: err.Error(),
		}, true

	case errors.As(cve.Err, &cie), errors.As(err, &cie):
		return Diagnosis{
			Category: CategoryTLSOther,
			Summary:  "TLS certificate is invalid",
			Cause:    fmt.Sprintf("the certificate presented for %s is not usable: %s.", cc.Host, cve.Err),
			Remediation: []string{
				"Check this machine's system clock -- a wrong date makes valid certificates look expired.",
				"If a TLS-inspecting proxy is in the path, its certificate may have expired.",
			},
			Raw: err.Error(),
		}, true
	}

	return Diagnosis{
		Category:    CategoryTLSOther,
		Summary:     "TLS handshake failed",
		Cause:       fmt.Sprintf("could not complete the TLS handshake with %s: %s.", cc.Host, cve.Err),
		Remediation: []string{"Re-run with --verbose for the raw TLS error."},
		Raw:         err.Error(),
	}, true
}

func dnsDiagnosis(cc classifyCtx, dnsErr *net.DNSError, err error) Diagnosis {
	d := Diagnosis{Category: CategoryDNS, Raw: err.Error()}
	switch {
	case dnsErr.IsNotFound:
		d.Summary = "DNS name does not exist"
		d.Cause = fmt.Sprintf("%q returned no records (NXDOMAIN).", dnsErr.Name)
	case dnsErr.IsTimeout:
		d.Summary = "DNS lookup timed out"
		d.Cause = fmt.Sprintf("no DNS server answered for %q in time.", dnsErr.Name)
	default:
		d.Summary = "DNS lookup failed"
		d.Cause = fmt.Sprintf("could not resolve %q: %s.", dnsErr.Name, dnsErr.Err)
	}
	d.Remediation = []string{
		fmt.Sprintf("Check %q for typos in the endpoint flag or KUSARI_* environment variable.", dnsErr.Name),
		"On a split-horizon or VPN network the name may only resolve while connected to the VPN.",
		fmt.Sprintf("Verify DNS resolution directly: nslookup %s", dnsErr.Name),
	}
	return d
}

func tcpDiagnosis(cc classifyCtx, err error) (Diagnosis, bool) {
	var (
		summary string
		cause   string
		fixes   []string
	)

	switch {
	case isConnRefused(err):
		summary = "connection refused"
		cause = fmt.Sprintf("%s:443 actively refused the connection.", cc.Host)
		fixes = []string{
			"Something answered but rejected the connection -- often a proxy or firewall on the path.",
			fmt.Sprintf("Ask your network team to allow outbound TCP 443 to %s.", cc.Host),
		}
	case isConnReset(err):
		summary = "connection reset"
		cause = fmt.Sprintf("the connection to %s:443 was reset before completing.", cc.Host)
		fixes = []string{
			"A firewall or intrusion-prevention device commonly resets connections it blocks.",
			fmt.Sprintf("Ask your network team whether TLS to %s is being terminated or dropped.", cc.Host),
		}
	case isUnreachable(err):
		summary = "host unreachable"
		cause = fmt.Sprintf("no route to %s:443 from this machine.", cc.Host)
		fixes = []string{
			"Check that this machine has working internet access (or VPN, if required).",
			"If egress requires a proxy, set HTTPS_PROXY.",
		}
	default:
		var opErr *net.OpError
		if !errors.As(err, &opErr) {
			return Diagnosis{}, false
		}
		summary = "TCP connection failed"
		cause = fmt.Sprintf("could not open a TCP connection to %s:443: %v.", cc.Host, opErr.Err)
		fixes = []string{
			fmt.Sprintf("Ask your network team to allow outbound TCP 443 to %s.", cc.Host),
			"Re-run with --verbose for the raw transport error.",
		}
	}

	return Diagnosis{
		Category:    CategoryTCP,
		Summary:     summary,
		Cause:       cause,
		Remediation: fixes,
		Raw:         err.Error(),
	}, true
}

func timeoutDiagnosis(cc classifyCtx, err error) Diagnosis {
	cause := fmt.Sprintf("%s did not respond before the timeout elapsed.", cc.Host)
	fixes := []string{
		"Re-run with a longer timeout, e.g. --timeout 30s.",
		fmt.Sprintf("A silent drop (no refusal, no reset) usually means a firewall is blackholing traffic to %s:443.", cc.Host),
	}
	if cc.Proxy != nil {
		fixes = append(fixes, fmt.Sprintf("Traffic is routed through proxy %s; confirm that proxy can reach %s.",
			cc.Proxy.Host, cc.Host))
	}
	return Diagnosis{
		Category:    CategoryTimeout,
		Summary:     "connection timed out",
		Cause:       cause,
		Remediation: fixes,
		Raw:         err.Error(),
	}
}

// trustRemediation returns OS-specific CA install steps. issuerCN may be empty.
func trustRemediation(issuerCN string, cc classifyCtx, proxyHost string) []string {
	name := "your corporate root CA"
	if issuerCN != "" {
		name = fmt.Sprintf("%q", issuerCN)
	}

	fixes := []string{
		fmt.Sprintf("Install the CA certificate %s into this machine's system trust store.", name),
		"macOS: add the CA to the System keychain and mark it 'Always Trust'. Go uses the macOS " +
			"platform verifier, so SSL_CERT_FILE is IGNORED on macOS.",
		"Linux: copy the CA PEM into /usr/local/share/ca-certificates/ and run `update-ca-certificates`, " +
			"or set SSL_CERT_FILE=/path/to/ca-bundle.pem.",
		"Windows: import the CA into the Local Machine 'Trusted Root Certification Authorities' store.",
	}
	if proxyHost != "" {
		fixes = append(fixes, fmt.Sprintf("Traffic to %s:443 goes through proxy %s, which is terminating TLS, "+
			"so that proxy's CA must be trusted. Alternatively add %s to NO_PROXY.", cc.Host, proxyHost, cc.Host))
	}
	return append(fixes, "The Kusari CLI intentionally provides no --insecure or --ca-cert flag: trust must be "+
		"configured at the OS level so every tool on this machine behaves consistently.")
}

// issuerCN names the untrusted authority, or "" if undeterminable.
func issuerCN(cve *tls.CertificateVerificationError) string {
	if cve == nil || len(cve.UnverifiedCertificates) == 0 {
		return ""
	}
	// The last cert sent is closest to the root, so it names the authority.
	last := cve.UnverifiedCertificates[len(cve.UnverifiedCertificates)-1]
	if last.Issuer.CommonName != "" {
		return last.Issuer.CommonName
	}
	if len(last.Issuer.Organization) > 0 {
		return last.Issuer.Organization[0]
	}
	return ""
}

func issuerOf(cve *tls.CertificateVerificationError) string {
	if cn := issuerCN(cve); cn != "" {
		return fmt.Sprintf("issuer: %q", cn)
	}
	return "issuer could not be determined"
}

func proxyHostOf(cc classifyCtx) string {
	if cc.Proxy == nil {
		return ""
	}
	return cc.Proxy.Host
}
