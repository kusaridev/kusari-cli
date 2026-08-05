// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kusaridev/kusari-cli/v2/pkg/auth"
	"github.com/kusaridev/kusari-cli/v2/pkg/connectivity"
	"github.com/kusaridev/kusari-cli/v2/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	ruleWidth  = 70
	nameColumn = 13
)

func connectivityCheck() *cobra.Command {
	var (
		timeout    time.Duration
		reportPath string
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check network connectivity to Kusari endpoints",
		Long: `Verify that this machine can reach the endpoints the Kusari CLI needs.

Each endpoint is probed with an unauthenticated GET. An endpoint counts as
reachable when the TLS handshake completes and any HTTP status is returned,
including 401, 403, 404, 307 and 5xx: this checks the network path, not
authentication, authorization or service health.

Proxy settings are taken from the standard HTTP_PROXY, HTTPS_PROXY and NO_PROXY
environment variables, exactly like every other Kusari CLI command. There is
deliberately no --proxy flag, so a passing check reflects what the rest of the
CLI will actually do.

Endpoints can be overridden with --platform-url and --console-url, or with the
KUSARI_PLATFORM_URL, KUSARI_CONSOLE_URL, KUSARI_AUTH_ENDPOINT and KUSARI_TENANT
environment variables.`,
		Example: `  kusari connectivity check
  kusari conn check --timeout 30s --verbose
  kusari connectivity check --report kusari-preflight.json
  HTTPS_PROXY=http://proxy.corp:8080 kusari conn check`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			endpoints := connectivity.Endpoints{
				AuthURL:     authEndpointFromConfig(),
				PlatformURL: platformUrl,
				ConsoleURL:  consoleUrl,
				Tenant:      tenantFromConfig(),
			}

			targets := connectivity.DefaultTargets(endpoints)
			allowlist := connectivity.AllowlistHosts(targets)
			proxyEnv := connectivity.ProxyEnvConfig(os.Getenv)

			printCheckHeader(proxyEnv)

			results := connectivity.Check(cmd.Context(), targets, connectivity.Options{
				Timeout: timeout,
			})

			printResults(results)
			if endpoints.Tenant == "" {
				fmt.Println("Skipped: the tenant-specific SBOM upload bucket, because no tenant is")
				fmt.Println("  configured. Run `kusari auth login`, or set KUSARI_TENANT, to include it.")
				fmt.Println()
			}
			printFailures(results)
			printAllowlist(allowlist, proxyEnv)

			if reportPath != "" {
				rep := connectivity.BuildReport(results, allowlist, getVersion(), proxyEnv, time.Now())
				if err := connectivity.WriteReport(reportPath, rep); err != nil {
					return err
				}
				fmt.Printf("Report written to %s\n", reportPath)
			}

			var failed []string
			for _, r := range results {
				if !r.OK() {
					failed = append(failed, r.Target.Name)
				}
			}
			if len(failed) > 0 {
				return fmt.Errorf("connectivity check failed: %d of %d endpoints unreachable (%s)",
					len(failed), len(results), strings.Join(failed, ", "))
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", connectivity.DefaultTimeout, "Per-endpoint timeout")
	cmd.Flags().StringVar(&reportPath, "report", "", "Write a JSON report to this path (for support tickets)")

	return cmd
}

// authEndpointFromConfig reads the auth endpoint. Do NOT bind a flag to
// "auth-endpoint" here: viper keeps one pflag per key, so a second binding would
// overwrite auth_login.go's and silently discard `auth login --auth-endpoint`.
// Reading the key still picks up KUSARI_AUTH_ENDPOINT and .env.
func authEndpointFromConfig() string {
	if v := strings.TrimSpace(viper.GetString("auth-endpoint")); v != "" {
		return v
	}
	return constants.DefaultAuthURL
}

// tenantFromConfig resolves the tenant for the SBOM upload bucket hostname,
// mirroring platform.go's precedence without re-binding those viper keys.
// Returns "" when no tenant is known, in which case the tenant-specific bucket
// is not probed rather than guessed at.
func tenantFromConfig() string {
	if v := strings.TrimSpace(viper.GetString("tenant-endpoint")); v != "" {
		return tenantFromEndpoint(v)
	}
	if v := strings.TrimSpace(viper.GetString("tenant")); v != "" {
		return v
	}
	workspace, err := auth.LoadWorkspace(platformUrl, "")
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Note: could not load workspace configuration: %v\n", err)
		}
		return ""
	}
	return workspace.Tenant
}

// tenantFromEndpoint takes the first hostname label, as pkg/repo/uploader.go
// does when recovering a tenant name from an endpoint.
func tenantFromEndpoint(endpoint string) string {
	host := endpoint
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		host = u.Hostname()
	}
	label, _, found := strings.Cut(host, ".")
	if !found {
		return ""
	}
	return label
}

func printCheckHeader(proxyEnv connectivity.ProxyConfig) {
	fmt.Println(strings.Repeat("=", ruleWidth))
	fmt.Println("Kusari Connectivity Check")
	fmt.Println(strings.Repeat("=", ruleWidth))
	fmt.Println()
	fmt.Println("Proxy configuration (from environment):")
	fmt.Printf("  HTTPS_PROXY  %s\n", orNotSet(proxyEnv.HTTPS))
	fmt.Printf("  HTTP_PROXY   %s\n", orNotSet(proxyEnv.HTTP))
	fmt.Printf("  NO_PROXY     %s\n", orNotSet(proxyEnv.NoProxy))
	fmt.Println()
}

func printResults(results []connectivity.Result) {
	fmt.Println(strings.Repeat("-", ruleWidth))

	reachable := 0
	for _, r := range results {
		glyph := "✗"
		if r.OK() {
			glyph = "✓"
			reachable++
		}
		fmt.Printf("  %s %-*s %s\n", glyph, nameColumn, r.Target.Name, r.Target.URL)

		detail := fmt.Sprintf("FAILED: %s", r.Diag.Summary)
		if r.OK() {
			detail = fmt.Sprintf("HTTP %s", r.Status)
		}
		route := "direct"
		if r.Proxy != nil {
			route = "via " + connectivity.ProxyLabel(r.Proxy)
		}
		fmt.Printf("      %*s%s  |  %s  |  %s\n", nameColumn, "", detail,
			formatDuration(r.Duration), route)

		if r.ServiceError() {
			fmt.Printf("      %*sNOTE: 5xx indicates a Kusari service issue, not a network\n", nameColumn, "")
			fmt.Printf("      %*s      problem on your side. Contact %s.\n", nameColumn, "", connectivity.SupportEmail)
		}

		if verbose {
			printVerboseDetail(r)
		}
	}

	fmt.Println(strings.Repeat("-", ruleWidth))
	fmt.Printf("Total: %d targets | Reachable: %d | Unreachable: %d\n",
		len(results), reachable, len(results)-reachable)
	fmt.Println()

	if reachable == len(results) {
		fmt.Println("All endpoints reachable.")
		fmt.Println("  401/403/404/307 and 5xx count as reachable: this check is unauthenticated")
		fmt.Println("  and only verifies DNS, TCP, TLS and proxy connectivity.")
		fmt.Println()
	}
}

func printVerboseDetail(r connectivity.Result) {
	pad := fmt.Sprintf("      %*s", nameColumn, "")
	if r.TLS != nil {
		fmt.Printf("%s%s (%s)\n", pad, r.TLS.Version, r.TLS.CipherSuite)
		if r.TLS.PeerSubjectCN != "" || r.TLS.PeerIssuerCN != "" {
			fmt.Printf("%scert %q issued by %q\n", pad, r.TLS.PeerSubjectCN, r.TLS.PeerIssuerCN)
		}
	}
	if r.Location != "" {
		fmt.Printf("%sLocation: %s (not followed)\n", pad, locationForDisplay(r.Location))
	}
	if r.Diag.Raw != "" {
		fmt.Fprintf(os.Stderr, "%s%s\n", pad, r.Diag.Raw)
	}
}

func printFailures(results []connectivity.Result) {
	var failures []connectivity.Result
	for _, r := range results {
		if !r.OK() {
			failures = append(failures, r)
		}
	}
	if len(failures) == 0 {
		return
	}

	fmt.Println("Failures")
	fmt.Println(strings.Repeat("-", ruleWidth))
	for _, r := range failures {
		fmt.Printf("✗ %s - %s\n", r.Target.Name, r.Target.Purpose)
		fmt.Printf("  Endpoint: %s\n", r.Target.URL)
		if s := connectivity.ProxySummary(r.Proxy); s != "" {
			fmt.Printf("  Proxy:    %s\n", s)
		} else {
			fmt.Printf("  Proxy:    (direct connection)\n")
		}
		fmt.Printf("  Category: %s\n", r.Diag.Category)
		if r.Diag.Cause != "" {
			printWrapped("  Cause:    ", r.Diag.Cause)
		}
		if len(r.Diag.Remediation) > 0 {
			fmt.Println("  Fix:")
			for i, step := range r.Diag.Remediation {
				printWrapped(fmt.Sprintf("    %d. ", i+1), step)
			}
		}
		fmt.Println()
	}
	fmt.Println(strings.Repeat("-", ruleWidth))
	if !verbose {
		fmt.Println("Re-run with --verbose for the raw transport errors.")
	}
	fmt.Println()
}

func printAllowlist(hosts []connectivity.AllowlistHost, proxyEnv connectivity.ProxyConfig) {
	fmt.Println("Network Requirements")
	fmt.Println(strings.Repeat("-", ruleWidth))
	fmt.Println("Allow outbound HTTPS (TCP 443) to the following hostnames:")
	fmt.Println()
	for _, h := range hosts {
		fmt.Printf("  • %s\n", h.Host)
		fmt.Printf("    Purpose: %s\n", h.Purpose)
		fmt.Println()
	}
	if proxyEnv.HTTP != "" || proxyEnv.HTTPS != "" {
		fmt.Println("  Proxy: configured via HTTP_PROXY/HTTPS_PROXY")
	} else {
		fmt.Println("  Proxy: none configured (direct connection)")
	}
	fmt.Println()
	fmt.Println("Important:")
	fmt.Println("  • Use hostnames, not IP addresses, in firewall and proxy rules.")
	fmt.Println("    The underlying addresses change frequently.")
	fmt.Println()
	fmt.Printf("For assistance, contact %s.\n", connectivity.SupportEmail)
	fmt.Println(strings.Repeat("=", ruleWidth))
}

// printWrapped prints text after prefix, aligning continuation lines under it.
func printWrapped(prefix, text string) {
	indent := strings.Repeat(" ", len(prefix))
	limit := ruleWidth - len(prefix)
	if limit < 20 {
		limit = 20
	}

	line := ""
	first := true
	flush := func() {
		if first {
			fmt.Printf("%s%s\n", prefix, line)
			first = false
		} else {
			fmt.Printf("%s%s\n", indent, line)
		}
		line = ""
	}

	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= limit:
			line += " " + word
		default:
			flush()
			line = word
		}
	}
	if line != "" {
		flush()
	}
}

// locationForDisplay strips the query from a redirect target: OAuth redirects
// carry state tokens, and this output gets pasted into support tickets.
func locationForDisplay(location string) string {
	u, err := url.Parse(location)
	if err != nil || u.Host == "" {
		return location
	}
	shown := u.Scheme + "://" + u.Host + u.Path
	if u.RawQuery != "" {
		shown += "?…"
	}
	return shown
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.3fs", d.Seconds())
}

func orNotSet(v string) string {
	if v == "" {
		return "(not set)"
	}
	return v
}
