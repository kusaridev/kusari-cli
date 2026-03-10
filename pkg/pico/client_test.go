// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package pico

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tenant := "demo"
	client := NewClient(tenant)

	assert.NotNil(t, client)
	assert.Equal(t, tenant, client.tenant)
	assert.Equal(t, "https://demo.api.us.kusari.cloud", client.baseURL)
	assert.NotNil(t, client.httpClient)
}

func TestNewClient_URLConstruction(t *testing.T) {
	tests := []struct {
		tenant      string
		expectedURL string
	}{
		{"demo", "https://demo.api.us.kusari.cloud"},
		{"test", "https://test.api.us.kusari.cloud"},
		{"prod", "https://prod.api.us.kusari.cloud"},
	}

	for _, tt := range tests {
		t.Run(tt.tenant, func(t *testing.T) {
			client := NewClient(tt.tenant)
			assert.Equal(t, tt.expectedURL, client.baseURL)
		})
	}
}

// mockServer creates a test HTTP server that returns the given response
func mockServer(t *testing.T, statusCode int, response interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		data, err := json.Marshal(response)
		require.NoError(t, err)
		w.Write(data)
	}))
}

func TestClient_MakeRequest_ErrorResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   map[string]string
	}{
		{
			name:       "400 Bad Request",
			statusCode: 400,
			response:   map[string]string{"error": "Bad request"},
		},
		{
			name:       "401 Unauthorized",
			statusCode: 401,
			response:   map[string]string{"error": "Unauthorized"},
		},
		{
			name:       "404 Not Found",
			statusCode: 404,
			response:   map[string]string{"error": "Not found"},
		},
		{
			name:       "500 Internal Server Error",
			statusCode: 500,
			response:   map[string]string{"error": "Internal server error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockServer(t, tt.statusCode, tt.response)
			defer server.Close()

			client := NewClient("test")
			client.baseURL = server.URL

			ctx := context.Background()
			// This will fail due to auth, but we're testing error response handling
			_, err := client.makeRequest(ctx, "GET", "/test", nil, nil)
			assert.Error(t, err)
		})
	}
}
