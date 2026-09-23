// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/kusaridev/kusari-cli/v2/pkg/auth"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The platform pre-run builds the tenant endpoint from the platform URL, so a dev platform URL gives
// a dev tenant host whether the tenant comes from --tenant or the stored workspace.
func TestPlatformPreRun_TenantEndpoint(t *testing.T) {
	const devPlatform = "https://platform.api.dev.kusari.cloud/"

	tests := []struct {
		name         string
		viper        map[string]string
		platformURL  string
		workspace    *auth.WorkspaceInfo
		wantEndpoint string
		wantErr      string
	}{
		{
			name:         "tenant flag on dev",
			viper:        map[string]string{"tenant": "demo"},
			platformURL:  devPlatform,
			wantEndpoint: "https://demo.api.dev.kusari.cloud",
		},
		{
			name:         "workspace tenant on dev",
			platformURL:  devPlatform,
			workspace:    &auth.WorkspaceInfo{ID: "w", PlatformUrl: devPlatform, Tenant: "stars"},
			wantEndpoint: "https://stars.api.dev.kusari.cloud",
		},
		{
			name:         "tenant-endpoint wins over tenant",
			viper:        map[string]string{"tenant-endpoint": "http://localhost:8080", "tenant": "demo"},
			platformURL:  devPlatform,
			wantEndpoint: "http://localhost:8080",
		},
		{
			name:        "tenant flag with unusable platform URL errors",
			viper:       map[string]string{"tenant": "demo"},
			platformURL: "http://localhost:9000/",
			wantErr:     `host must start with "platform."`,
		},
		{
			name:        "workspace tenant with unusable platform URL leaves endpoint unset",
			platformURL: "http://localhost:9000/",
			workspace:   &auth.WorkspaceInfo{ID: "w", PlatformUrl: "http://localhost:9000/", Tenant: "demo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)

			origPlatform, origEndpoint, origTenant := platformUrl, platformTenantEndpoint, platformTenant
			t.Cleanup(func() { platformUrl, platformTenantEndpoint, platformTenant = origPlatform, origEndpoint, origTenant })

			for k, v := range tt.viper {
				viper.Set(k, v)
			}
			platformUrl = tt.platformURL
			platformTenantEndpoint = ""
			if tt.workspace != nil {
				require.NoError(t, auth.SaveWorkspace(*tt.workspace))
			}

			err := platformCmd.PersistentPreRunE(platformCmd, nil)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantEndpoint, platformTenantEndpoint)
		})
	}
}
