// =============================================================================
// pkg/repo/scanner.go
// =============================================================================
package repo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kusaridev/iac/app-code/kusari-cli/pkg/auth"
)

const (
	patchName   = "kusari-inspector.patch"
	metaName    = "kusari-inspector.json"
	tarballName = "kusari-inspector.tar.bz2"
	tarballDir  = "kusari-dir"
)

func Scan(dir string, diffCmd []string, platformUrl string) error {
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

	apiEndpoint := fmt.Sprintf("%s/inspector/presign/bundle-upload", platformUrl)
	presignedUrl, err := getPresignedURL(apiEndpoint, token.AccessToken, tarballName)
	if err != nil {
		return fmt.Errorf("failed to get presigned URL: %w", err)
	}
	fmt.Printf("Presigned URL: %s\n", presignedUrl)

	if err := uploadFileToS3(presignedUrl, filepath.Join(tarballDir, tarballName)); err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return nil
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
