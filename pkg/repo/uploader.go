// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/kusaridev/kusari-cli/pkg/auth"
	"github.com/kusaridev/kusari-cli/pkg/constants"
	"github.com/kusaridev/kusari-cli/pkg/login"
	"golang.org/x/sync/errgroup"
)

// Document describes the input for a processor to run. This input can
// come from a collector or from the processor itself (run recursively).
type Document struct {
	Blob              []byte            `json:"blob"`
	Type              DocumentType      `json:"type"`
	Format            FormatType        `json:"format"`
	Encoding          EncodingType      `json:"encoding"`
	SourceInformation SourceInformation `json:"source_information"`
}

// DocumentType describes the type of the document contents for schema checks
type DocumentType string

// Document* is the enumerables of DocumentType
const (
	DocumentSBOM    DocumentType = "SBOM"
	DocumentOpenVEX DocumentType = "OPEN_VEX"
)

// FormatType describes the document format for malform checks
type FormatType string

// Format* is the enumerables of FormatType
const (
	FormatJSON      FormatType = "JSON"
	FormatJSONLines FormatType = "JSON_LINES"
	FormatXML       FormatType = "XML"
	FormatUnknown   FormatType = "UNKNOWN"
)

type EncodingType string

const (
	EncodingBzip2   EncodingType = "BZIP2"
	EncodingZstd    EncodingType = "ZSTD"
	EncodingUnknown EncodingType = "UNKNOWN"
)

var EncodingExts = map[string]EncodingType{
	".bz2": EncodingBzip2,
	".zst": EncodingZstd,
}

// SourceInformation provides additional information about where the document comes from
type SourceInformation struct {
	// Collector describes the name of the collector providing this information
	Collector string `json:"collector"`
	// Source describes the source which the collector got this information
	Source string `json:"source"`
	// DocumentRef describes the location of the document in the blob store
	DocumentRef string `json:"document_ref"`
}

// DocumentWrapper holds extra fields without modifying processor.Document
type DocumentWrapper struct {
	*Document
	UploadMetaData *map[string]string `json:"upload_metadata,omitempty"`
}

type sbomSubjectAndURI struct {
	subject string
	uri     string
	docRef  string
}

type softwareIDAndSbomID struct {
	SoftwareID int64 `json:"software_id"`
	SbomID     int64 `json:"sbom_id"`
}

type blockedPackages struct {
	Blocked         bool     `json:"blocked"`
	BlockedPackages []string `json:"blocked_packages"`
}

type StatusMeta struct {
	Status       string `json:"status"`        // started, processing, success, failed
	UserMessage  string `json:"user_message"`  // customer-facing message
	InternalMeta string `json:"internal_meta"` // internal metadata/details
	UpdatedAt    string `json:"updated_at"`    // timestamp in milliseconds
}

// IngestionStatusItem represents an item in the pico-ingestion-status DynamoDB table
type IngestionStatusItem struct {
	Workspace    string     `json:"workspace"`     // partition key
	Sort         string     `json:"sort"`          // sort key
	DocumentType string     `json:"document_type"` // SBOM, VEX, etc.
	DocumentName string     `json:"document_name"` // Name of the ingested document
	TTL          int64      `json:"ttl"`           // TTL in Unix epoch seconds
	StatusMeta   StatusMeta `json:"statusMeta"`
}

type cdxSBOM struct {
	BOMFormat    string `json:"bomFormat"`
	SerialNumber string `json:"serialNumber"`
	Metadata     struct {
		Component struct {
			Name string `json:"name"`
		} `json:"component"`
	} `json:"metadata"`
}

type spdxSBOM struct {
	SPDXID            string `json:"SPDXID"`
	DocumentNamespace string `json:"documentNamespace"`
	Name              string `json:"name"`
}

// Upload handles the upload of SBOM or OpenVEX files to the Kusari platform
func Upload(
	filePath string,
	tenantEndpoint string,
	platformUrl string,
	alias string,
	docType string,
	isOpenVex bool,
	tag string,
	softwareID string,
	sbomSubject string,
	componentName string,
	checkBlockedPackages bool,
) error {
	// Validate required configuration
	if filePath == "" {
		return fmt.Errorf("file-path is required")
	}

	if tenantEndpoint == "" {
		return fmt.Errorf("tenant configuration missing. Please provide --tenant flag (e.g., --tenant demo), or --tenant-endpoint if working in developement, or run 'kusari auth login'")
	}

	// Display the tenant endpoint being used
	fmt.Printf("Using tenant endpoint: %s\n", tenantEndpoint)

	if isOpenVex && (tag == "" || (softwareID == "" && sbomSubject == "")) {
		return fmt.Errorf("when using OpenVEX, tag must be specified, and so must software-id or sbom-subject")
	}

	// Load the auth token
	token, err := auth.LoadToken("kusari")
	if err != nil {
		return fmt.Errorf("failed to load auth token: %w (try running 'kusari auth login')", err)
	}

	// Check if token is expired
	if err := auth.CheckTokenExpiry(token); err != nil {
		return fmt.Errorf("auth token expired: %w (try running 'kusari auth login')", err)
	}

	accessToken := token.AccessToken

	// Set default platform URL if not provided
	if platformUrl == "" {
		platformUrl = constants.DefaultPlatformURL
	}

	// Get workspace
	var workspace string
	var workspaceDescription string
	storedWorkspace, err := auth.LoadWorkspace(platformUrl, "")
	if err != nil {
		// If no workspace is stored, try to fetch and use first workspace
		workspaces, _, workspaceGetterErr := login.FetchWorkspaces(platformUrl, accessToken)
		if workspaceGetterErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get workspaces: %v\n", workspaceGetterErr)
		} else if len(workspaces) > 0 {
			workspace = workspaces[0].ID
			workspaceDescription = workspaces[0].Description
			fmt.Fprintf(os.Stderr, "Using workspace: %s\n", workspaceDescription)
		}
	} else {
		workspace = storedWorkspace.ID
		workspaceDescription = storedWorkspace.Description
		fmt.Fprintf(os.Stderr, "Using workspace: %s\n", workspaceDescription)
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Check if path is a directory or file
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("error getting file info: %w", err)
	}

	if fileInfo.IsDir() && isOpenVex {
		return fmt.Errorf("OpenVEX can't be used with directories, only single files")
	}

	// Build upload metadata
	uploadMeta := map[string]string{}
	if alias != "" {
		uploadMeta["alias"] = alias
	}
	if docType != "" {
		uploadMeta["type"] = docType
	}
	if tag != "" {
		uploadMeta["tag"] = tag
	}
	if softwareID != "" {
		uploadMeta["software_id"] = softwareID
	}
	if sbomSubject != "" {
		uploadMeta["sbom_subject"] = sbomSubject
	}
	if componentName != "" {
		uploadMeta["component_name"] = componentName
	}

	var ssaus []sbomSubjectAndURI

	// Upload based on file type
	if fileInfo.IsDir() {
		fmt.Printf("Uploading directory: %s\n", filePath)
		ssaus, err = uploadDirectory(client, accessToken, tenantEndpoint, filePath, uploadMeta)
		if err != nil {
			return fmt.Errorf("directory upload failed: %w", err)
		}
	} else {
		fmt.Printf("Uploading file: %s\n", filePath)
		ssau, err := uploadSingleFile(client, accessToken, tenantEndpoint, filePath, isOpenVex, uploadMeta)
		if err != nil {
			return fmt.Errorf("single file upload failed: %w", err)
		}
		ssaus = []sbomSubjectAndURI{ssau}
	}

	// Extract tenant name from tenant endpoint
	tenantName := ""
	if parsedURL, err := url.Parse(tenantEndpoint); err == nil {
		hostname := parsedURL.Hostname()
		// Extract subdomain (e.g., "parth" from "parth.api.dev.kusari.cloud")
		if idx := strings.Index(hostname, "."); idx != -1 {
			tenantName = hostname[:idx]
		} else {
			tenantName = hostname
		}
	}

	// Query ingestion status for each uploaded document
	if workspace != "" && tenantName != "" {
		type ingestionResult struct {
			docRef       string
			documentName string
			status       string
			err          error
		}

		results := make([]ingestionResult, 0, len(ssaus))

		for _, ssau := range ssaus {
			if ssau.docRef == "" {
				continue
			}

			result, err := queryForIngestionStatus(platformUrl, tenantName, ssau.docRef, accessToken, workspace)
			if err != nil {
				results = append(results, ingestionResult{
					docRef: ssau.docRef,
					status: "failed",
					err:    err,
				})
				continue
			}

			if result != nil && result.DocumentName != "" {
				results = append(results, ingestionResult{
					docRef:       ssau.docRef,
					documentName: result.DocumentName,
					status:       result.StatusMeta.Status,
				})
			}
		}

		// Display results in a table if multiple documents, or simple output for single document
		if len(results) > 1 {
			fmt.Fprintf(os.Stderr, "\nIngestion Results:\n")
			w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "STATUS\tDOCUMENT NAME\tDOCUMENT REF")
			fmt.Fprintln(w, "------\t-------------\t------------")
			for _, r := range results {
				statusSymbol := "✓"
				if r.status == "failed" || r.err != nil {
					statusSymbol = "✗"
				}
				docName := r.documentName
				if docName == "" {
					docName = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", statusSymbol, docName, r.docRef)
			}
			w.Flush()
		} else if len(results) == 1 {
			r := results[0]
			if r.err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to check ingestion status for %s: %v\n", r.docRef, r.err)
			} else if r.documentName != "" {
				fmt.Fprintf(os.Stderr, "Successfully ingested: %s\n", r.documentName)
			}
		}
	}

	if checkBlockedPackages {
		blocked, err := checkSBOMsForBlockedPackages(context.Background(), client, accessToken, tenantEndpoint, ssaus)
		if err != nil {
			return fmt.Errorf("error checking for blocked packages: %w", err)
		}

		if blocked {
			return fmt.Errorf("blocked packages found in uploaded SBOMs")
		}
	}

	return nil
}

// uploadDirectory uses filepath.Walk to walk through the directory and upload the files that are found
func uploadDirectory(client *http.Client, accessToken, tenantEndpoint, dirPath string, uploadMeta map[string]string) ([]sbomSubjectAndURI, error) {
	var ssaus []sbomSubjectAndURI

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fmt.Printf("  Uploading: %s\n", path)
			ssau, err := uploadSingleFile(client, accessToken, tenantEndpoint, path, false, uploadMeta)
			if err != nil {
				return fmt.Errorf("uploadSingleFile failed with error: %w", err)
			}
			ssaus = append(ssaus, ssau)
		}
		return nil
	})

	return ssaus, err
}

// uploadSingleFile creates a presigned URL for the filepath and calls uploadBlob to upload the actual file
func uploadSingleFile(client *http.Client, accessToken, tenantEndpoint, filePath string, isOpenVex bool,
	uploadMeta map[string]string) (sbomSubjectAndURI, error) {
	// check that the file is not empty
	checkFile, err := os.Stat(filePath)
	if err != nil {
		return sbomSubjectAndURI{}, fmt.Errorf("failed to get stats on filepath: %s, with error: %w", filePath, err)
	}
	// if file is empty, do not upload and return nil
	if checkFile.Size() == 0 {
		fmt.Printf("  Skipping empty file: %s\n", filePath)
		return sbomSubjectAndURI{}, nil
	}

	blob, err := os.ReadFile(filePath)
	if err != nil {
		return sbomSubjectAndURI{}, fmt.Errorf("error reading file: %s, err: %w", filePath, err)
	}

	// Prepare the payload for the presigned URL request
	payload := map[string]string{
		"filename": getDocRef(blob),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return sbomSubjectAndURI{}, fmt.Errorf("error creating JSON payload: %w", err)
	}

	presignedUrl, err := getPresignedUrlForUpload(client, accessToken, tenantEndpoint, payloadBytes)
	if err != nil {
		return sbomSubjectAndURI{}, err
	}

	return uploadBlob(client, presignedUrl, filePath, blob, isOpenVex, uploadMeta)
}

// getPresignedUrlForUpload utilizes authorized client to obtain the presigned URL to upload to S3
func getPresignedUrlForUpload(client *http.Client, accessToken, tenantEndpoint string, payloadBytes []byte) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return getPresignedURLWithOptions(presignedURLOptions{
		client:      client,
		apiEndpoint: tenantEndpoint + "/presign",
		jwtToken:    accessToken,
		payload:     payload,
		workspace:   "", // No workspace header for SBOM uploads
	})
}

// uploadBlob takes the file and creates a Document blob which is uploaded to S3
func uploadBlob(client *http.Client, presignedUrl, filePath string, readFile []byte, isOpenVex bool,
	uploadMeta map[string]string) (sbomSubjectAndURI, error) {

	doctype := DocumentSBOM
	if isOpenVex {
		doctype = DocumentOpenVEX
	}

	docRef := getDocRef(readFile)

	baseDoc := &Document{
		Blob:   readFile,
		Type:   doctype,
		Format: FormatUnknown,
		SourceInformation: SourceInformation{
			Collector:   "Kusari-CLI",
			Source:      fmt.Sprintf("file:///%s", filePath),
			DocumentRef: docRef,
		},
	}

	var docByte []byte
	var err error

	if len(uploadMeta) != 0 {
		// Wrap it with additional metadata about the project
		docWrapper := DocumentWrapper{
			Document:       baseDoc,
			UploadMetaData: &uploadMeta,
		}

		docByte, err = json.Marshal(docWrapper)
		if err != nil {
			return sbomSubjectAndURI{}, fmt.Errorf("failed marshal of document: %w", err)
		}
	} else {
		docByte, err = json.Marshal(baseDoc)
		if err != nil {
			return sbomSubjectAndURI{}, fmt.Errorf("failed marshal of document: %w", err)
		}
	}

	// Upload using the shared function
	err = uploadToS3WithOptions(uploadToS3Options{
		client:       client,
		presignedURL: presignedUrl,
		data:         docByte,
		contentType:  "multipart/form-data",
	})
	if err != nil {
		return sbomSubjectAndURI{}, err
	}

	// Get SBOM subjects and URIs for checking against the blocked package list.
	var cdx cdxSBOM
	if err := json.Unmarshal(readFile, &cdx); err == nil { // inverted error check
		if cdx.BOMFormat == "CycloneDX" && cdx.Metadata.Component.Name != "" && cdx.SerialNumber != "" {
			return sbomSubjectAndURI{subject: cdx.Metadata.Component.Name, uri: cdx.SerialNumber, docRef: docRef}, nil
		}
	}

	var spdx spdxSBOM
	if err := json.Unmarshal(readFile, &spdx); err == nil { // inverted error check
		if spdx.SPDXID == "SPDXRef-DOCUMENT" && spdx.Name != "" && spdx.DocumentNamespace != "" {
			return sbomSubjectAndURI{subject: spdx.Name, uri: spdx.DocumentNamespace + "#DOCUMENT", docRef: docRef}, nil
		}
	}

	return sbomSubjectAndURI{docRef: docRef}, nil
}

// checkSBOMsForBlockedPackages checks if uploaded SBOMs contain any blocked packages
func checkSBOMsForBlockedPackages(ctx context.Context, client *http.Client, accessToken, tenantEndpoint string, ssaus []sbomSubjectAndURI) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(5)

	blocked := make([]bool, len(ssaus))
	blockedPurls := make([][]string, len(ssaus))

	for i, ssau := range ssaus {
		if ssau.subject == "" && ssau.uri == "" {
			continue
		}

		g.Go(func() error {
			var ids softwareIDAndSbomID

			// Poll for software/SBOM IDs until available
			for {
				res, err := makePicoRequest(ctx, client, accessToken, tenantEndpoint, fmt.Sprintf("pico/v1/software/id?software_name=%s&sbom_uri=%s",
					url.QueryEscape(ssau.subject), url.QueryEscape(ssau.uri)))
				if err != nil {
					return fmt.Errorf("error making request for IDs: %w", err)
				}
				defer res.Body.Close() //nolint:errcheck

				if res.StatusCode == 200 {
					body, err := io.ReadAll(res.Body)
					if err != nil {
						return fmt.Errorf("error reading response body for IDs: %w", err)
					}

					if err := json.Unmarshal(body, &ids); err != nil {
						return fmt.Errorf("error unmarshaling response body for IDs: %w", err)
					}

					break
				} else if res.StatusCode == 404 {
					fmt.Printf("  Waiting for SBOM to be ingested (subject: %s)...\n", ssau.subject)
					time.Sleep(time.Second)
				} else {
					return fmt.Errorf("unexpected response status code for IDs: %d", res.StatusCode)
				}
			}

			// Check for blocked packages
			res, err := makePicoRequest(ctx, client, accessToken, tenantEndpoint, fmt.Sprintf("pico/v1/packages/blocked/check/software/%d/sbom/%d",
				ids.SoftwareID, ids.SbomID))
			if err != nil {
				return fmt.Errorf("error making request for check: %w", err)
			}
			defer res.Body.Close() //nolint:errcheck

			if res.StatusCode == 200 {
				body, err := io.ReadAll(res.Body)
				if err != nil {
					return fmt.Errorf("error reading response body for check: %w", err)
				}

				var bps blockedPackages
				if err := json.Unmarshal(body, &bps); err != nil {
					return fmt.Errorf("error unmarshaling response body for check: %w", err)
				}

				if bps.Blocked {
					blocked[i] = true
					blockedPurls[i] = bps.BlockedPackages
				}
			} else {
				return fmt.Errorf("unexpected response status code for check: %d", res.StatusCode)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return false, err
	}

	// Report blocked packages
	for i, v := range blocked {
		if v {
			fmt.Printf("\nBlocked packages found for SBOM subject %s with URI %s:\n", ssaus[i].subject, ssaus[i].uri)
			for _, bp := range blockedPurls[i] {
				fmt.Printf("  - %s\n", bp)
			}
		}
	}

	return slices.Contains(blocked, true), nil
}

// makePicoRequest makes an HTTP GET request to the Pico API with authentication
func makePicoRequest(ctx context.Context, client *http.Client, accessToken, tenantURL, pathAndQS string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", tenantURL, pathAndQS), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// queryForIngestionStatus polls the ingestion status endpoint until a result is found or timeout
func queryForIngestionStatus(platformUrl, tenantName, docRef, accessToken, workspace string) (*IngestionStatusItem, error) {
	maxAttempts := 150 // 150 attempts * 2 seconds = 5 minutes max
	attempt := 0
	sleepDuration := 2 * time.Second

	// Create spinner for stderr
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = os.Stderr
	s.Prefix = "Checking ingestion status... "
	s.FinalMSG = "✓ Ingestion complete!\n"
	s.Start()

	// Ensure spinner stops no matter what
	defer s.Stop()

	client := &http.Client{Timeout: 10 * time.Second}

	for attempt < maxAttempts {
		attempt++

		fullURL := fmt.Sprintf("%s/ingestion/status?tenantName=%s&docRef=%s",
			strings.TrimSuffix(platformUrl, "/"),
			url.QueryEscape(tenantName),
			url.QueryEscape(docRef))

		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			time.Sleep(sleepDuration)
			continue
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Kusari-Workspace", workspace)

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(sleepDuration)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				time.Sleep(sleepDuration)
				continue
			}

			var results []IngestionStatusItem
			if err := json.Unmarshal(body, &results); err != nil {
				time.Sleep(sleepDuration)
				continue
			}

			if len(results) > 0 {
				status := results[0].StatusMeta.Status
				userMsg := results[0].StatusMeta.UserMessage

				switch status {
				case "success":
					s.FinalMSG = "✓ Ingestion successful!\n"
					s.Stop()
					if userMsg != "" {
						fmt.Fprintf(os.Stderr, "%s\n", userMsg)
					}
					return &results[0], nil
				case "failed":
					s.FinalMSG = "✗ Ingestion failed!\n"
					s.Stop()
					if userMsg != "" {
						fmt.Fprintf(os.Stderr, "Error: %s\n", userMsg)
					}
					return nil, fmt.Errorf("ingestion failed: %s", userMsg)
				default:
					// Update spinner prefix with current status
					prefix := status
					if len(status) >= 1 {
						prefix = strings.ToUpper(status[:1]) + status[1:]
					}
					if userMsg != "" {
						s.Prefix = prefix + ": " + userMsg + "... "
					} else {
						s.Prefix = prefix + "... "
					}
				}
			}
		}
		resp.Body.Close()

		time.Sleep(sleepDuration)
	}

	// If we get here, we timed out
	s.FinalMSG = "✗ Ingestion status check timed out\n"
	s.Stop()
	return nil, fmt.Errorf("ingestion status not found after %d attempts", maxAttempts)
}

// getDocRef returns the Document Reference of a blob; i.e. the blob store key for this blob.
func getDocRef(blob []byte) string {
	generatedHash := getHash(blob)
	return fmt.Sprintf("sha256_%s", generatedHash)
}

// getHash returns the SHA256 hash of data as a hex string
func getHash(data []byte) string {
	sha256sum := sha256.Sum256(data)
	return hex.EncodeToString(sha256sum[:])
}
