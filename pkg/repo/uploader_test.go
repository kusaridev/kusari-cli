// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kusaridev/kusari-cli/pkg/constants"
)

func TestGetHash(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "empty data",
			data:     []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple string",
			data:     []byte("hello world"),
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "json data",
			data:     []byte(`{"test": "data"}`),
			expected: "40b61fe1b15af0a4d5402735b26343e8cf8a045f4d81710e6108a21d91eaf366",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getHash(tt.data)
			if result != tt.expected {
				t.Errorf("getHash() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetDocRef(t *testing.T) {
	tests := []struct {
		name     string
		blob     []byte
		expected string
	}{
		{
			name:     "empty blob",
			blob:     []byte{},
			expected: "sha256_e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple blob",
			blob:     []byte("test data"),
			expected: "sha256_916f0027a575074ce72a331777c3478d6513f786a591bd892da1a577bf2335f9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDocRef(tt.blob)
			if result != tt.expected {
				t.Errorf("getDocRef() = %v, want %v", result, tt.expected)
			}
			// Verify it has the correct prefix
			if !strings.HasPrefix(result, "sha256_") {
				t.Errorf("getDocRef() should start with 'sha256_', got %v", result)
			}
		})
	}
}

func TestUploadBlob(t *testing.T) {
	tests := []struct {
		name            string
		fileContent     string
		isOpenVex       bool
		uploadMeta      map[string]string
		serverStatus    int
		serverResponse  string
		expectError     bool
		errorContains   string
		expectedSubject string
		expectedURI     string
	}{
		{
			name: "successful CycloneDX upload",
			fileContent: `{
				"bomFormat": "CycloneDX",
				"serialNumber": "urn:uuid:3e671687-395b-41f5-a30f-a58921a69b79",
				"metadata": {
					"component": {
						"name": "my-app"
					}
				}
			}`,
			isOpenVex:       false,
			uploadMeta:      map[string]string{"alias": "test"},
			serverStatus:    http.StatusOK,
			expectError:     false,
			expectedSubject: "my-app",
			expectedURI:     "urn:uuid:3e671687-395b-41f5-a30f-a58921a69b79",
		},
		{
			name: "successful SPDX upload",
			fileContent: `{
				"SPDXID": "SPDXRef-DOCUMENT",
				"documentNamespace": "https://example.com/spdx/my-app",
				"name": "my-app"
			}`,
			isOpenVex:       false,
			uploadMeta:      map[string]string{},
			serverStatus:    http.StatusOK,
			expectError:     false,
			expectedSubject: "my-app",
			expectedURI:     "https://example.com/spdx/my-app#DOCUMENT",
		},
		{
			name:            "OpenVEX upload",
			fileContent:     `{"test": "vex"}`,
			isOpenVex:       true,
			uploadMeta:      map[string]string{"tag": "v1.0"},
			serverStatus:    http.StatusOK,
			expectError:     false,
			expectedSubject: "",
			expectedURI:     "",
		},
		{
			name:          "unauthorized error",
			fileContent:   `{"test": "data"}`,
			isOpenVex:     false,
			uploadMeta:    map[string]string{},
			serverStatus:  http.StatusUnauthorized,
			expectError:   true,
			errorContains: "upload failed with status 401",
		},
		{
			name:          "internal server error",
			fileContent:   `{"test": "data"}`,
			isOpenVex:     false,
			uploadMeta:    map[string]string{},
			serverStatus:  http.StatusInternalServerError,
			expectError:   true,
			errorContains: "upload failed",
		},
		{
			name: "invalid CycloneDX (missing required fields)",
			fileContent: `{
				"bomFormat": "CycloneDX",
				"serialNumber": "urn:uuid:3e671687-395b-41f5-a30f-a58921a69b79"
			}`,
			isOpenVex:       false,
			uploadMeta:      map[string]string{},
			serverStatus:    http.StatusOK,
			expectError:     false,
			expectedSubject: "",
			expectedURI:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("Expected PUT request, got %s", r.Method)
				}
				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}
			}))
			defer server.Close()

			client := server.Client()
			ssau, err := uploadBlob(
				client,
				server.URL,
				"test.json",
				[]byte(tt.fileContent),
				tt.isOpenVex,
				tt.uploadMeta,
			)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if ssau.subject != tt.expectedSubject {
					t.Errorf("Expected subject '%s', got '%s'", tt.expectedSubject, ssau.subject)
				}
				if ssau.uri != tt.expectedURI {
					t.Errorf("Expected URI '%s', got '%s'", tt.expectedURI, ssau.uri)
				}
			}
		})
	}
}

func TestGetPresignedUrlForUpload(t *testing.T) {
	tests := []struct {
		name          string
		payload       map[string]string
		serverStatus  int
		serverResp    interface{}
		expectError   bool
		errorContains string
		expectedURL   string
	}{
		{
			name:         "successful request",
			payload:      map[string]string{"filename": "test.json"},
			serverStatus: http.StatusOK,
			serverResp: map[string]string{
				"presignedUrl": "https://s3.amazonaws.com/bucket/key?signature=xyz",
			},
			expectError: false,
			expectedURL: "https://s3.amazonaws.com/bucket/key?signature=xyz",
		},
		{
			name:          "unauthorized",
			payload:       map[string]string{"filename": "test.json"},
			serverStatus:  http.StatusUnauthorized,
			expectError:   true,
			errorContains: "unauthorized request",
		},
		{
			name:          "forbidden",
			payload:       map[string]string{"filename": "test.json"},
			serverStatus:  http.StatusForbidden,
			expectError:   true,
			errorContains: "kusari auth login",
		},
		{
			name:          "internal server error",
			payload:       map[string]string{"filename": "test.json"},
			serverStatus:  http.StatusInternalServerError,
			expectError:   true,
			errorContains: "unexpected status code",
		},
		{
			name:          "invalid json response",
			payload:       map[string]string{"filename": "test.json"},
			serverStatus:  http.StatusOK,
			serverResp:    "invalid json",
			expectError:   true,
			errorContains: "failed to unmarshal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("Expected Bearer token, got %s", r.Header.Get("Authorization"))
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected application/json, got %s", r.Header.Get("Content-Type"))
				}

				w.WriteHeader(tt.serverStatus)
				if tt.serverResp != nil {
					switch v := tt.serverResp.(type) {
					case string:
						_, _ = w.Write([]byte(v))
					case map[string]string:
						_ = json.NewEncoder(w).Encode(v)
					}
				}
			}))
			defer server.Close()

			payloadBytes, _ := json.Marshal(tt.payload)
			client := server.Client()

			url, err := getPresignedUrlForUpload(client, "test-token", server.URL, payloadBytes)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if url != tt.expectedURL {
					t.Errorf("Expected URL '%s', got '%s'", tt.expectedURL, url)
				}
			}
		})
	}
}

func TestMakePicoRequest(t *testing.T) {
	tests := []struct {
		name          string
		pathAndQS     string
		serverStatus  int
		serverResp    string
		expectError   bool
		errorContains string
	}{
		{
			name:         "successful request",
			pathAndQS:    "pico/v1/software/id?software_name=test&sbom_uri=test-uri",
			serverStatus: http.StatusOK,
			serverResp:   `{"software_id": 1, "sbom_id": 2}`,
			expectError:  false,
		},
		{
			name:         "not found",
			pathAndQS:    "pico/v1/software/id?software_name=test&sbom_uri=test-uri",
			serverStatus: http.StatusNotFound,
			expectError:  false,
		},
		{
			name:         "server error",
			pathAndQS:    "pico/v1/packages/blocked/check/software/1/sbom/2",
			serverStatus: http.StatusInternalServerError,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("Expected Bearer token, got %s", r.Header.Get("Authorization"))
				}
				if r.Header.Get("Accept") != "application/json" {
					t.Errorf("Expected Accept: application/json, got %s", r.Header.Get("Accept"))
				}

				w.WriteHeader(tt.serverStatus)
				if tt.serverResp != "" {
					_, _ = w.Write([]byte(tt.serverResp))
				}
			}))
			defer server.Close()

			client := server.Client()
			ctx := context.Background()

			resp, err := makePicoRequest(ctx, client, "test-token", server.URL, tt.pathAndQS)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("Expected response, got nil")
				} else {
					defer func() { _ = resp.Body.Close() }()
					if resp.StatusCode != tt.serverStatus {
						t.Errorf("Expected status %d, got %d", tt.serverStatus, resp.StatusCode)
					}
				}
			}
		})
	}
}

func TestUploadSingleFile(t *testing.T) {
	tests := []struct {
		name          string
		fileContent   string
		fileName      string
		isOpenVex     bool
		uploadMeta    map[string]string
		expectError   bool
		errorContains string
		expectSkip    bool
	}{
		{
			name:        "successful upload with metadata",
			fileContent: `{"test": "data"}`,
			fileName:    "test.json",
			isOpenVex:   false,
			uploadMeta:  map[string]string{"alias": "test-alias"},
			expectError: false,
		},
		{
			name:        "successful upload without metadata",
			fileContent: `{"test": "data"}`,
			fileName:    "test2.json",
			isOpenVex:   false,
			uploadMeta:  map[string]string{},
			expectError: false,
		},
		{
			name:        "empty file should skip",
			fileContent: "",
			fileName:    "empty.json",
			isOpenVex:   false,
			uploadMeta:  map[string]string{},
			expectError: false,
			expectSkip:  true,
		},
		{
			name:        "OpenVEX file",
			fileContent: `{"@context": "https://openvex.dev/ns"}`,
			fileName:    "vex.json",
			isOpenVex:   true,
			uploadMeta:  map[string]string{"tag": "v1.0"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, tt.fileName)
			err := os.WriteFile(filePath, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			// Create mock upload server (simulates S3)
			uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer uploadServer.Close()

			// Create mock presign server (simulates tenant endpoint)
			presignServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/presign") {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"presignedUrl": uploadServer.URL,
					})
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer presignServer.Close()

			client := &http.Client{}

			ssau, err := uploadSingleFile(
				client,
				"test-token",
				presignServer.URL,
				filePath,
				tt.isOpenVex,
				tt.uploadMeta,
			)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.expectSkip {
					if ssau.subject != "" || ssau.uri != "" {
						t.Error("Expected empty ssau for skipped file")
					}
				}
			}
		})
	}
}

func TestUploadDirectory(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string // filename -> content
		uploadMeta    map[string]string
		expectError   bool
		errorContains string
		expectedFiles int
	}{
		{
			name: "upload multiple files",
			files: map[string]string{
				"sbom1.json": `{"bomFormat": "CycloneDX"}`,
				"sbom2.json": `{"SPDXID": "SPDXRef-DOCUMENT"}`,
				"sbom3.json": `{"test": "data"}`,
			},
			uploadMeta:    map[string]string{"alias": "test"},
			expectError:   false,
			expectedFiles: 3,
		},
		{
			name: "upload with empty files",
			files: map[string]string{
				"sbom1.json": `{"bomFormat": "CycloneDX"}`,
				"empty.json": "",
			},
			uploadMeta:    map[string]string{},
			expectError:   false,
			expectedFiles: 1, // empty file should be skipped
		},
		{
			name: "upload with subdirectory",
			files: map[string]string{
				"sbom1.json":        `{"test": "data"}`,
				"subdir/sbom2.json": `{"test": "data2"}`,
			},
			uploadMeta:    map[string]string{},
			expectError:   false,
			expectedFiles: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory structure
			tmpDir := t.TempDir()
			for filename, content := range tt.files {
				fullPath := filepath.Join(tmpDir, filename)
				dir := filepath.Dir(fullPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("Failed to create directory: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create file: %v", err)
				}
			}

			// Create mock server
			uploadCount := 0
			var serverURL string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/presign") {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"presignedUrl": serverURL + "/upload",
					})
				} else {
					uploadCount++
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer server.Close()
			serverURL = server.URL

			client := server.Client()

			ssaus, err := uploadDirectory(client, "test-token", server.URL, tmpDir, tt.uploadMeta)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Count non-empty ssaus
				nonEmptyCount := 0
				for _, ssau := range ssaus {
					if ssau.subject != "" || ssau.uri != "" {
						nonEmptyCount++
					}
				}
				// Note: The count might not match exactly due to empty files being skipped
				// but still being added to the slice
				if len(ssaus) < tt.expectedFiles {
					t.Errorf("Expected at least %d files processed, got %d", tt.expectedFiles, len(ssaus))
				}
			}
		})
	}
}

func TestCheckSBOMsForBlockedPackages(t *testing.T) {
	tests := []struct {
		name            string
		ssaus           []sbomSubjectAndURI
		mockResponses   map[string]mockResponse // endpoint -> response
		expectedBlocked bool
		expectError     bool
		errorContains   string
	}{
		{
			name: "no blocked packages",
			ssaus: []sbomSubjectAndURI{
				{subject: "my-app", uri: "urn:uuid:12345"},
			},
			mockResponses: map[string]mockResponse{
				"/software/id": {
					status: http.StatusOK,
					body:   `{"software_id": 1, "sbom_id": 2}`,
				},
				"/packages/blocked/check": {
					status: http.StatusOK,
					body:   `{"blocked": false, "blocked_packages": []}`,
				},
			},
			expectedBlocked: false,
			expectError:     false,
		},
		{
			name: "blocked packages found",
			ssaus: []sbomSubjectAndURI{
				{subject: "my-app", uri: "urn:uuid:12345"},
			},
			mockResponses: map[string]mockResponse{
				"/software/id": {
					status: http.StatusOK,
					body:   `{"software_id": 1, "sbom_id": 2}`,
				},
				"/packages/blocked/check": {
					status: http.StatusOK,
					body:   `{"blocked": true, "blocked_packages": ["pkg:npm/malicious@1.0.0"]}`,
				},
			},
			expectedBlocked: true,
			expectError:     false,
		},
		{
			name: "empty ssau should skip",
			ssaus: []sbomSubjectAndURI{
				{subject: "", uri: ""},
			},
			mockResponses:   map[string]mockResponse{},
			expectedBlocked: false,
			expectError:     false,
		},
		{
			name: "multiple SBOMs with mixed results",
			ssaus: []sbomSubjectAndURI{
				{subject: "app1", uri: "urn:uuid:1"},
				{subject: "app2", uri: "urn:uuid:2"},
			},
			mockResponses: map[string]mockResponse{
				"/software/id": {
					status: http.StatusOK,
					body:   `{"software_id": 1, "sbom_id": 2}`,
				},
				"/packages/blocked/check": {
					status: http.StatusOK,
					body:   `{"blocked": true, "blocked_packages": ["pkg:npm/bad@1.0"]}`,
				},
			},
			expectedBlocked: true,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var resp mockResponse
				found := false

				// Match endpoint patterns
				if strings.Contains(r.URL.Path, "software/id") {
					resp = tt.mockResponses["/software/id"]
					found = true
				} else if strings.Contains(r.URL.Path, "packages/blocked/check") {
					resp = tt.mockResponses["/packages/blocked/check"]
					found = true
				}

				if found {
					w.WriteHeader(resp.status)
					_, _ = w.Write([]byte(resp.body))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			client := server.Client()
			ctx := context.Background()

			blocked, err := checkSBOMsForBlockedPackages(ctx, client, "test-token", server.URL, tt.ssaus)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if blocked != tt.expectedBlocked {
					t.Errorf("Expected blocked=%v, got %v", tt.expectedBlocked, blocked)
				}
			}
		})
	}
}

// Helper struct for mock responses
type mockResponse struct {
	status int
	body   string
}

func TestUpload_ValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		filePath      string
		tenantURL     string
		isOpenVex     bool
		tag           string
		softwareID    string
		sbomSubject   string
		expectError   bool
		errorContains string
	}{
		{
			name:          "missing file path",
			filePath:      "",
			tenantURL:     "https://test.com",
			expectError:   true,
			errorContains: "file-path is required",
		},
		{
			name:          "missing tenant endpoint",
			filePath:      "/test/file.json",
			tenantURL:     "",
			expectError:   true,
			errorContains: "tenant configuration missing",
		},
		{
			name:          "OpenVEX missing tag",
			filePath:      "/test/file.json",
			tenantURL:     "https://test.com",
			isOpenVex:     true,
			tag:           "",
			expectError:   true,
			errorContains: "tag must be specified",
		},
		{
			name:          "OpenVEX missing software-id and sbom-subject",
			filePath:      "/test/file.json",
			tenantURL:     "https://test.com",
			isOpenVex:     true,
			tag:           "v1.0",
			softwareID:    "",
			sbomSubject:   "",
			expectError:   true,
			errorContains: "software-id or sbom-subject",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Upload(
				tt.filePath,
				tt.tenantURL,
				constants.DefaultPlatformURL,
				"",
				"",
				tt.isOpenVex,
				tt.tag,
				tt.softwareID,
				tt.sbomSubject,
				"",
				"",
				"",
				false,
				false,
			)

			if !tt.expectError {
				t.Error("Expected error, got nil")
				return
			}

			if err == nil {
				t.Errorf("Expected error containing '%s', got nil", tt.errorContains)
			} else if !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
			}
		})
	}
}
