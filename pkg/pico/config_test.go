// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		platformURL string
		tenant      string
		want        string
		wantErr     string
	}{
		{name: "us", platformURL: "https://platform.api.us.kusari.cloud/", tenant: "demo", want: "https://demo.api.us.kusari.cloud"},
		{name: "dev", platformURL: "https://platform.api.dev.kusari.cloud/", tenant: "demo", want: "https://demo.api.dev.kusari.cloud"},
		{name: "preview", platformURL: "https://platform.api.preview.kusari.cloud/", tenant: "testbed", want: "https://testbed.api.preview.kusari.cloud"},
		{name: "no trailing slash", platformURL: "https://platform.api.dev.kusari.cloud", tenant: "demo", want: "https://demo.api.dev.kusari.cloud"},
		{name: "path dropped", platformURL: "https://platform.api.dev.kusari.cloud/some/path", tenant: "demo", want: "https://demo.api.dev.kusari.cloud"},
		{name: "port kept", platformURL: "http://platform.localtest.me:8080/", tenant: "demo", want: "http://demo.localtest.me:8080"},
		{name: "hyphenated tenant", platformURL: "https://platform.api.dev.kusari.cloud/", tenant: "tenant-alpha", want: "https://tenant-alpha.api.dev.kusari.cloud"},
		{name: "tenant lowercased", platformURL: "https://platform.api.dev.kusari.cloud/", tenant: "Demo", want: "https://demo.api.dev.kusari.cloud"},

		{name: "host not platform", platformURL: "https://custom-platform.example.com/", tenant: "demo", wantErr: `host must start with "platform."`},
		{name: "localhost", platformURL: "http://localhost:8080/", tenant: "demo", wantErr: `host must start with "platform."`},
		{name: "bare platform host", platformURL: "https://platform/", tenant: "demo", wantErr: `host must start with "platform."`},
		{name: "no scheme", platformURL: "platform.api.dev.kusari.cloud", tenant: "demo", wantErr: "expected a URL like"},
		{name: "empty platform URL", platformURL: "", tenant: "demo", wantErr: "expected a URL like"},
		{name: "empty tenant", platformURL: "https://platform.api.dev.kusari.cloud/", tenant: "", wantErr: "invalid tenant name"},
		{name: "tenant with dot", platformURL: "https://platform.api.dev.kusari.cloud/", tenant: "x.evil.com", wantErr: "invalid tenant name"},
		{name: "tenant with slash", platformURL: "https://platform.api.dev.kusari.cloud/", tenant: "x/", wantErr: "invalid tenant name"},
		{name: "tenant leading hyphen", platformURL: "https://platform.api.dev.kusari.cloud/", tenant: "-demo", wantErr: "invalid tenant name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TenantEndpoint(tt.platformURL, tt.tenant)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
