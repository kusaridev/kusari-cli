// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// uploadToS3Options contains configuration for uploading data to S3
type uploadToS3Options struct {
	client       *http.Client
	presignedURL string
	data         []byte
	contentType  string
}

// uploadToS3WithOptions uploads data to S3 using a presigned URL
func uploadToS3WithOptions(opts uploadToS3Options) error {
	if len(opts.data) == 0 {
		return nil // Skip empty uploads
	}

	client := opts.client
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequest("PUT", opts.presignedURL, bytes.NewReader(opts.data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if opts.contentType != "" {
		req.Header.Set("Content-Type", opts.contentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UploadZipToS3 uploads a local file to S3 using a presigned URL.
func uploadFileToS3(presignedURL, filePath string) error {
	// check that the file is not empty
	checkFile, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get stats on filepath: %s, with error: %w", filePath, err)
	}
	// if file is empty, do not upload and return nil
	if checkFile.Size() == 0 {
		return nil
	}

	// Read the file content into memory
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	return uploadToS3WithOptions(uploadToS3Options{
		presignedURL: presignedURL,
		data:         fileBytes,
		contentType:  "application/x-bzip2",
	})
}

// presignedURLOptions contains configuration for obtaining a presigned URL
type presignedURLOptions struct {
	client      *http.Client
	apiEndpoint string
	jwtToken    string
	payload     map[string]any
	workspace   string
}

// getPresignedURLWithOptions is a flexible function to obtain presigned URLs
func getPresignedURLWithOptions(opts presignedURLOptions) (string, error) {
	payloadBytes, err := json.Marshal(opts.payload)
	if err != nil {
		return "", fmt.Errorf("error creating JSON payload: %w", err)
	}

	client := opts.client
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	req, err := http.NewRequest("POST", opts.apiEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to POST to %s, with error: %w", opts.apiEndpoint, err)
	}

	req.Header.Set("Authorization", "Bearer "+opts.jwtToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if opts.workspace != "" {
		req.Header.Set("X-Kusari-Workspace", opts.workspace)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to POST to %s, with error: %w", opts.apiEndpoint, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return "", fmt.Errorf("GetPresignedUrl failed with unauthorized request: %d", resp.StatusCode)
		case http.StatusForbidden:
			// Handle the HTTP 403 case by suggesting the user login
			return "", fmt.Errorf("GetPresignedUrl failed with forbidden (%d). Try `kusari auth login`", resp.StatusCode)
		case http.StatusBadRequest:
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("GetPresignedUrl failed with bad request (%d). Body was: %s", resp.StatusCode, string(body))
		default:
			return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body with error: %w", err)
	}

	type urlResponse struct {
		PresignedUrl string `json:"presignedUrl"`
	}

	var result urlResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal the results with body: %s with error: %w", string(body), err)
	}

	return result.PresignedUrl, nil
}

// GetPresignedUrl utilizes authorized client to obtain the presigned URL to upload to S3
func getPresignedURL(apiEndpoint string, jwtToken string, filePath, workspace string, full bool, size int64) (string, error) {
	scanType := "diff"
	if full {
		scanType = "full"
	}

	payload := map[string]any{
		"filename":        filePath,
		"type":            scanType,
		"file_size_bytes": size,
	}

	return getPresignedURLWithOptions(presignedURLOptions{
		apiEndpoint: apiEndpoint,
		jwtToken:    jwtToken,
		payload:     payload,
		workspace:   workspace,
	})
}
