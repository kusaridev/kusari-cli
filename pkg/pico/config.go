// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// tenantLabel is a single DNS label: letters, digits, and inner hyphens.
var tenantLabel = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// TenantEndpoint builds a tenant's Pico API endpoint from the platform URL by swapping the leading
// "platform" host label for the tenant name, so every environment gets its own tenant host:
// https://platform.api.dev.kusari.cloud/ with tenant "demo" gives https://demo.api.dev.kusari.cloud.
// It keeps the scheme and port, drops any path, and errors when the platform host doesn't start
// with "platform." rather than guessing.
func TenantEndpoint(platformURL, tenant string) (string, error) {
	if !tenantLabel.MatchString(tenant) {
		return "", fmt.Errorf("invalid tenant name %q: must be a single DNS label (letters, digits, hyphens)", tenant)
	}

	u, err := url.Parse(platformURL)
	if err != nil || u.Scheme == "" || u.Hostname() == "" {
		return "", fmt.Errorf("cannot build tenant endpoint from platform URL %q: expected a URL like https://platform.api.us.kusari.cloud/", platformURL)
	}

	rest, ok := strings.CutPrefix(u.Hostname(), "platform.")
	if !ok || rest == "" {
		return "", fmt.Errorf("cannot build tenant endpoint from platform URL %q: host must start with \"platform.\"; use --tenant-endpoint instead", platformURL)
	}

	host := strings.ToLower(tenant) + "." + rest
	if port := u.Port(); port != "" {
		host = net.JoinHostPort(host, port)
	}
	return u.Scheme + "://" + host, nil
}
