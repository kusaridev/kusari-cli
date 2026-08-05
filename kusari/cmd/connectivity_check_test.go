// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kusaridev/kusari-cli/v2/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it
// wrote. The printing helpers write to stdout directly, matching the rest of
// this package.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	require.NoError(t, w.Close())
	out := <-done
	require.NoError(t, r.Close())
	return out
}

func TestAuthEndpointFromConfig(t *testing.T) {
	t.Run("falls back to the shared constant", func(t *testing.T) {
		resetViper(t)
		assert.Equal(t, constants.DefaultAuthURL, authEndpointFromConfig())
	})

	t.Run("honors the auth-endpoint key", func(t *testing.T) {
		resetViper(t)
		viper.Set("auth-endpoint", "https://auth.eu.kusari.cloud/")
		assert.Equal(t, "https://auth.eu.kusari.cloud/", authEndpointFromConfig())
	})

	t.Run("treats whitespace as unset", func(t *testing.T) {
		resetViper(t)
		viper.Set("auth-endpoint", "   ")
		assert.Equal(t, constants.DefaultAuthURL, authEndpointFromConfig())
	})
}

// Regression guard for a silent, high-impact failure mode.
//
// viper keeps ONE bound pflag per key. `kusari auth login` binds
// "auth-endpoint" (auth_login.go); the connectivity command must only READ that
// key, never bind a second flag to it. If someone adds a competing binding,
// init() ordering (filename order within package cmd) decides the winner, and
// viper.find would fall through to the wrong flag's default -- silently
// discarding `auth login --auth-endpoint=...` and logging the user into the
// wrong region.
func TestConnectivityDoesNotStealAuthEndpointBinding(t *testing.T) {
	resetViper(t)

	// Re-bind exactly as auth_login.go's init() does, then simulate the user
	// passing the flag.
	fresh := &cobra.Command{Use: "login"}
	fresh.Flags().String("auth-endpoint", constants.DefaultAuthURL, "authentication endpoint URL")
	require.NoError(t, viper.BindPFlag("auth-endpoint", fresh.Flags().Lookup("auth-endpoint")))
	require.NoError(t, fresh.Flags().Set("auth-endpoint", "https://auth.eu.kusari.cloud/"))

	// The user's override must survive, both through viper directly and through
	// the helper the connectivity command uses.
	assert.Equal(t, "https://auth.eu.kusari.cloud/", viper.GetString("auth-endpoint"))
	assert.Equal(t, "https://auth.eu.kusari.cloud/", authEndpointFromConfig())
}

// Same hazard for "tenant" and "tenant-endpoint", which platform.go owns.
func TestConnectivityDoesNotStealTenantBinding(t *testing.T) {
	resetViper(t)

	fresh := &cobra.Command{Use: "platform"}
	fresh.PersistentFlags().String("tenant", "", "Tenant name")
	require.NoError(t, viper.BindPFlag("tenant", fresh.PersistentFlags().Lookup("tenant")))
	require.NoError(t, fresh.PersistentFlags().Set("tenant", "acme"))

	assert.Equal(t, "acme", viper.GetString("tenant"))
	assert.Equal(t, "acme", tenantFromConfig())
}

func TestTenantFromConfig(t *testing.T) {
	t.Run("reads the tenant key", func(t *testing.T) {
		resetViper(t)
		viper.Set("tenant", "acme")
		assert.Equal(t, "acme", tenantFromConfig())
	})

	t.Run("trims whitespace", func(t *testing.T) {
		resetViper(t)
		viper.Set("tenant", "  acme  ")
		assert.Equal(t, "acme", tenantFromConfig())
	})

	// platform.go treats --tenant-endpoint as highest precedence, so a user who
	// sets only that must still get the tenant-specific bucket probed.
	t.Run("tenant-endpoint wins over tenant", func(t *testing.T) {
		resetViper(t)
		viper.Set("tenant", "acme")
		viper.Set("tenant-endpoint", "https://demo.api.dev.kusari.cloud")
		assert.Equal(t, "demo", tenantFromConfig())
	})
}

func TestTenantFromEndpoint(t *testing.T) {
	cases := map[string]string{
		"https://demo.api.dev.kusari.cloud":     "demo",
		"https://demo.api.us.kusari.cloud/":     "demo",
		"demo.api.us.kusari.cloud":              "demo",
		"https://demo.api.us.kusari.cloud:8443": "demo",
		// No dot means no tenant label to take; guessing would be worse.
		"https://localhost": "",
		"":                  "",
	}
	for in, want := range cases {
		assert.Equal(t, want, tenantFromEndpoint(in), "input %q", in)
	}
}

// The connectivity command must not register any flag whose name collides with
// a key another command has already bound to viper.
func TestConnectivityCheckFlags(t *testing.T) {
	cmd := connectivityCheck()

	require.NotNil(t, cmd.Flags().Lookup("timeout"))
	require.NotNil(t, cmd.Flags().Lookup("report"))

	for _, owned := range []string{"auth-endpoint", "tenant", "tenant-endpoint", "client-id", "client-secret"} {
		assert.Nil(t, cmd.Flags().Lookup(owned),
			"%q is bound to viper by another command; declaring it here would hijack that binding", owned)
	}

	// No --proxy: proxy configuration comes from the environment so a passing
	// check reflects what every other kusari command will do.
	assert.Nil(t, cmd.Flags().Lookup("proxy"))
	// No escape hatches that would weaken TLS verification.
	assert.Nil(t, cmd.Flags().Lookup("insecure"))
	assert.Nil(t, cmd.Flags().Lookup("ca-cert"))
}

func TestConnectivityCommandShape(t *testing.T) {
	cmd := Connectivity()

	assert.Equal(t, "connectivity", cmd.Name())
	assert.ElementsMatch(t, []string{"conn"}, cmd.Aliases)

	sub := cmd.Commands()
	require.Len(t, sub, 1)
	assert.Equal(t, "check", sub[0].Name())
	assert.NotEmpty(t, sub[0].Long)
	assert.NotEmpty(t, sub[0].Example)
	assert.NotNil(t, sub[0].Args, "Args must be set explicitly")
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{266 * time.Millisecond, "266ms"},
		{0, "0ms"},
		{999 * time.Millisecond, "999ms"},
		{time.Second, "1.000s"},
		{1204 * time.Millisecond, "1.204s"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, formatDuration(c.in))
	}
}

// A redirect Location can carry an OAuth state token. --verbose output gets
// pasted into support tickets, so the query must be elided.
func TestLocationForDisplayDropsQuery(t *testing.T) {
	got := locationForDisplay("https://auth.us.kusari.cloud/oauth2/authorize?client_id=abc&state=SECRETSTATE")
	assert.Equal(t, "https://auth.us.kusari.cloud/oauth2/authorize?…", got)
	assert.NotContains(t, got, "SECRETSTATE")

	assert.Equal(t, "https://aws.amazon.com/s3/", locationForDisplay("https://aws.amazon.com/s3/"))
	// A relative or unparseable Location is passed through unchanged.
	assert.Equal(t, "/login", locationForDisplay("/login"))
}

func TestOrNotSet(t *testing.T) {
	assert.Equal(t, "(not set)", orNotSet(""))
	assert.Equal(t, "http://p:8080", orNotSet("http://p:8080"))
}

func TestPrintWrappedRespectsWidth(t *testing.T) {
	long := strings.Repeat("word ", 40)
	out := captureStdout(t, func() { printWrapped("    1. ", long) })

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Greater(t, len(lines), 1, "long text must wrap")
	assert.True(t, strings.HasPrefix(lines[0], "    1. "))
	for _, l := range lines[1:] {
		assert.True(t, strings.HasPrefix(l, strings.Repeat(" ", len("    1. "))),
			"continuation lines align under the prefix: %q", l)
	}
	for _, l := range lines {
		assert.LessOrEqual(t, len(l), ruleWidth+len("    1. "))
	}
}
