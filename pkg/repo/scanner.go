// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kusaridev/kusari-cli/pkg/auth"
)

const (
	patchName   = "kusari-inspector.patch"
	metaName    = "kusari-inspector.json"
	tarballName = "kusari-inspector.tar.bz2"
	tarballDir  = "kusari-dir"
)

func Scan(dir string, diffCmd []string, platformUrl string, consoleUrl string, verbose bool) error {
	if verbose {
		fmt.Printf(" dir: %s\n", dir)
		fmt.Printf(" diffCmd: %s\n", strings.Join(diffCmd, " "))
		fmt.Printf(" platformUrl: %s\n", platformUrl)
		fmt.Printf(" consoleUrl: %s\n", consoleUrl)
	}

	if err := validateDirectory(dir); err != nil {
		return fmt.Errorf("failed to validate directory: %w", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}
	defer func() {
		// If these haven't been created yet, they will error.
		_ = os.Remove(patchName)
		_ = os.Remove(metaName)
		_ = os.Remove(filepath.Join(tarballDir, tarballName))
		_ = os.Remove(tarballDir)
		_ = os.Chdir(wd)
	}()

	if err := createMeta(diffCmd); err != nil {
		return fmt.Errorf("failed to package directory: %w", err)
	}

	if err := generateDiff(dir, diffCmd); err != nil {
		return fmt.Errorf("failed to generate diff: %w", err)
	}

	if err := packageDirectory(); err != nil {
		return fmt.Errorf("failed to package directory: %w", err)
	}

	token, err := auth.LoadToken("kusari")
	if err != nil {
		return fmt.Errorf("failed to load auth token: %w", err)
	}

	baseURL, err := url.Parse(platformUrl)
	if err != nil {
		return err
	}
	apiEndpoint := baseURL.JoinPath("inspector/presign/bundle-upload").String()

	presignedUrl, err := getPresignedURL(apiEndpoint, token.AccessToken, tarballName)
	if err != nil {
		return fmt.Errorf("failed to get presigned URL: %w", err)
	}

	if err := uploadFileToS3(presignedUrl, filepath.Join(tarballDir, tarballName)); err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	epoch, err := getEpochFromUrl(presignedUrl)
	if err != nil {
		return err
	}

	baseConsoleURL, err := url.Parse(consoleUrl)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("analysis/users/%s/result", *epoch)
	consoleFullUrl := baseConsoleURL.JoinPath(path).String()

	fmt.Printf("Success, your scan is processing! Once completed, you can see results here: %s\n", consoleFullUrl)

	return nil
}

func getEpochFromUrl(presignUrl string) (*string, error) {
	u, err := url.Parse(presignUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}
	epoch := path.Base(u.Path)
	return &epoch, nil
}

// ValidateDirectory checks if a directory exists and is readable
func validateDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", path)
		}
		return fmt.Errorf("cannot access directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	return nil
}
