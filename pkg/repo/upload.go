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

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// Read the file content into memory
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Create the HTTP PUT request
	req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader(fileBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set correct content type for zip
	req.Header.Set("Content-Type", "application/x-bzip2")

	// Perform the upload
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check S3 response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetPresignedUrl utilizes authorized client to obtain the presigned URL to upload to S3
func getPresignedURL(apiEndpoint string, jwtToken string, filePath string) (string, error) {

	// Prepare the payload for the presigned URL request
	payload := map[string]string{
		"filename": filePath,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error creating JSON payload: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	// Build request
	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to POST to %s, with error: %w", apiEndpoint, err)
	}

	// Add Authorization header with Bearer token
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/json")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to POST to %s, with error: %w", apiEndpoint, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf("GetPresignedUrl failed with unauthorized request: %d", resp.StatusCode)
		}
		// otherwise return an error
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body with error: %w", err)
	}

	type url struct {
		PresignedUrl string `json:"presignedUrl"`
	}

	var result url
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal the results with body: %s with error: %w", string(body), err)
	}

	presignedUrl := result.PresignedUrl

	return presignedUrl, nil
}
