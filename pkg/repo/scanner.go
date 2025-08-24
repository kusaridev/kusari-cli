// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/glamour"
	"github.com/kusaridev/kusari-cli/api"
	"github.com/kusaridev/kusari-cli/pkg/auth"
	urlBuilder "github.com/kusaridev/kusari-cli/pkg/url"
)

const (
	patchName   = "kusari-inspector.patch"
	metaName    = "kusari-inspector.json"
	tarballName = "kusari-inspector.tar.bz2"
	tarballDir  = "kusari-dir"
)

func Scan(dir string, diffCmd []string, platformUrl string, consoleUrl string, verbose bool) error {
	if verbose {
		fmt.Fprintf(os.Stderr, " dir: %s\n", dir)
		fmt.Fprintf(os.Stderr, " diffCmd: %s\n", strings.Join(diffCmd, " "))
		fmt.Fprintf(os.Stderr, " platformUrl: %s\n", platformUrl)
		fmt.Fprintf(os.Stderr, " consoleUrl: %s\n", consoleUrl)
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
		// If something else is in tarballDir, this will fail
		_ = os.Remove(tarballDir)
		_ = os.Chdir(wd)
	}()

	if err := createMeta(diffCmd); err != nil {
		return fmt.Errorf("failed to package directory: %w", err)
	}

	fmt.Fprint(os.Stderr, "Generating diff...\n")

	if err := generateDiff(dir, diffCmd); err != nil {
		return fmt.Errorf("failed to generate diff: %w", err)
	}

	fmt.Fprint(os.Stderr, "Packaging directory...\n")

	if err := packageDirectory(); err != nil {
		return fmt.Errorf("failed to package directory: %w", err)
	}

	token, err := auth.LoadToken("kusari")
	if err != nil {
		return fmt.Errorf("failed to load auth token: %w", err)
	}

	apiEndpoint, err := urlBuilder.Build(platformUrl, "inspector/presign/bundle-upload")
	if err != nil {
		return err
	}

	presignedUrl, err := getPresignedURL(*apiEndpoint, token.AccessToken, tarballName)
	if err != nil {
		return fmt.Errorf("failed to get presigned URL: %w", err)
	}

	fmt.Fprint(os.Stderr, "Uploading package repo...\n")

	if err := uploadFileToS3(presignedUrl, filepath.Join(tarballDir, tarballName)); err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	epoch, err := urlBuilder.GetEpochFromUrl(presignedUrl)
	if err != nil {
		return err
	}

	consoleFullUrl, err := urlBuilder.Build(consoleUrl, "analysis/users", *epoch, "result")
	if err != nil {
		return err
	}

	fmt.Fprint(os.Stderr, "Upload successful, your scan is processing!\n")
	// We print the URL when it is completed, but that doesn't help if it fails
	// for some reason and the user needs to contact support.
	fmt.Fprintf(os.Stderr, "Once completed, you can see results at: %s\n", *consoleFullUrl)
	return queryForResult(platformUrl, epoch, token.AccessToken, consoleFullUrl)
}

func queryForResult(platformUrl string, epoch *string, accessToken string, consoleFullUrl *string) error {
	maxAttempts := 50
	attempt := 0
	sleepDuration := 15 * time.Second

	// Create spinner for stderr
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = os.Stderr // Send spinner to stderr
	s.Prefix = "Analysis in progress... "
	s.FinalMSG = "✓ Results found!\n"
	s.Start()

	// Ensure spinner stops no matter what
	defer s.Stop()

	for attempt < maxAttempts {
		attempt++

		// Build URL
		fullURL := fmt.Sprintf("%s/inspector/result/user?sortKey=%s",
			strings.TrimSuffix(platformUrl, "/"),
			*epoch)

		// Create HTTP client
		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(sleepDuration)
			continue
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				time.Sleep(sleepDuration)
				continue
			}

			var results []api.UserInspectorResult
			if err := json.Unmarshal(body, &results); err != nil {
				time.Sleep(sleepDuration)
				continue
			}

			if len(results) > 0 {
				// Stop spinner before outputting results
				s.FinalMSG = "✓ Analysis complete!\n"
				s.Stop()

				// Clean and format results for stdout
				rawContent := results[0].Analysis.Results
				cleanedContent := removeImageLines(rawContent)

				// Render with glamour to stdout
				r, err := glamour.NewTermRenderer(
					glamour.WithAutoStyle(),
					glamour.WithWordWrap(100),
				)
				if err != nil {
					fmt.Print(cleanedContent) // stdout
					return nil
				}

				rendered, err := r.Render(cleanedContent)
				if err != nil {
					fmt.Print(cleanedContent) // stdout
					return nil
				}

				fmt.Fprintf(os.Stderr, "You can also view your results here: %s\n", *consoleFullUrl)

				fmt.Print(rendered) // stdout
				return nil
			}
		}

		time.Sleep(sleepDuration)
	}

	// If we get here, we failed
	s.FinalMSG = "✗ No results found after maximum attempts\n"
	s.Stop()
	return fmt.Errorf("no results found after %d attempts", maxAttempts)
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

func removeImageLines(content string) string {
	// Split into lines
	lines := strings.Split(content, "\n")
	var filteredLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip lines that start with "Image:" and contain "→"
		if strings.HasPrefix(trimmed, "Image:") && strings.Contains(trimmed, "→") {
			continue
		}
		filteredLines = append(filteredLines, line)
	}

	result := strings.Join(filteredLines, "\n")

	// Also remove any remaining markdown images
	imagePattern := regexp.MustCompile(`!\[.*?\]\(.*?\)`)
	result = imagePattern.ReplaceAllString(result, "")

	// Clean up multiple newlines
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result)
}
