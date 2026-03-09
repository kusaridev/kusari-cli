// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/auth"
)

const (
	defaultAuthEndpoint = "https://auth.us.kusari.cloud/"
	defaultPlatformURL  = "https://api.us.kusari.cloud"
)

// NewClientFromWorkspace creates a Pico client using the stored workspace configuration.
// It loads the workspace and extracts the tenant to initialize the client.
func NewClientFromWorkspace() (*Client, error) {
	// Load workspace to get tenant
	workspace, err := auth.LoadWorkspace(defaultPlatformURL, defaultAuthEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace: %w. Run `kusari auth login` to authenticate", err)
	}

	if workspace.Tenant == "" {
		return nil, fmt.Errorf("no tenant configured. Run `kusari auth login` to select a tenant")
	}

	return NewClient(workspace.Tenant), nil
}
