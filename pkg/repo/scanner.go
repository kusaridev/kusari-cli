// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/glamour"
	"github.com/kusaridev/kusari-cli/api"
	"github.com/kusaridev/kusari-cli/pkg/auth"
	"github.com/kusaridev/kusari-cli/pkg/sarif"
	urlBuilder "github.com/kusaridev/kusari-cli/pkg/url"
)

const (
	patchFile               = "kusari-inspector.patch"
	metaFile                = "kusari-inspector.json"
	tarballNameUncompressed = "kusari-inspector.tar"
	tarballName             = tarballNameUncompressed + ".bz2"
	workingDirName          = "kusari-dir"
)

var (
	metaName   string
	patchName  string
	tarballDir string
	workingDir string
)

func Scan(dir string, rev string, platformUrl string, consoleUrl string, verbose bool, wait bool, outputFormat string) error {
	return scan(dir, rev, platformUrl, consoleUrl, verbose, wait, false, outputFormat, nil)
}

func RiskCheck(dir string, platformUrl string, consoleUrl string, verbose bool, wait bool) error {
	// default to outputformat "markdown" for now for risk check as it will link to console
	return scan(dir, "", platformUrl, consoleUrl, verbose, wait, true, "markdown", nil)
}

// scanMock facilitates use of mock values for testing
type scanMock struct {
	fileUploader           func(presignedURL, filePath string) error
	presignedURLGetter     func(apiEndpoint string, jwtToken string, filePath, workspace string, full bool) (string, error)
	defaultWorkspaceGetter func(apiEndpoint string, jwtToken string) (string, error)
	token                  string
}

func scan(dir string, rev string, platformUrl string, consoleUrl string, verbose bool, wait bool, full bool, outputFormat string,
	mock *scanMock) error {
	if verbose {
		fmt.Fprintf(os.Stderr, " dir: %s\n", dir)
		fmt.Fprintf(os.Stderr, " rev: %s\n", rev)
		fmt.Fprintf(os.Stderr, " platformUrl: %s\n", platformUrl)
		fmt.Fprintf(os.Stderr, " consoleUrl: %s\n", consoleUrl)
		fmt.Fprintf(os.Stderr, " outputFormat: %s\n", outputFormat)
	}

	// Check to see if the directory has a .git directory. If it does not, it is not the root of
	// the repo and the scan will probably fail during analysis.
	_, err := os.Stat(filepath.Join(dir, ".git"))
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No .git directory found in %s\n  Directory must be root of repo\n", dir)
		os.Exit(1)
	}

	fileUploader := uploadFileToS3
	presignedURLGetter := getPresignedURL
	defaultWorkspaceGetter := getAPIDefaultWorkspace
	var accessToken string
	if mock != nil {
		fileUploader = mock.fileUploader
		presignedURLGetter = mock.presignedURLGetter
		defaultWorkspaceGetter = mock.defaultWorkspaceGetter
		accessToken = mock.token
	} else {
		token, err := auth.LoadToken("kusari")
		if err != nil {
			return fmt.Errorf("failed to load auth token: %w", err)
		}

		if err := auth.CheckTokenExpiry(token); err != nil {
			return err
		}
		accessToken = token.AccessToken
	}

	if err := validateDirectory(dir); err != nil {
		return fmt.Errorf("failed to validate directory: %w", err)
	}

	// Create a temporary working directory
	tempDir, err := os.MkdirTemp(os.TempDir(), "kusari-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	// Create the path inside it for our metadata and patch files
	workingDir = filepath.Join(tempDir, workingDirName)
	err = os.Mkdir(workingDir, os.FileMode(0700))
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	tarballDir = tempDir
	metaName = filepath.Join(tarballDir, workingDirName, metaFile)
	patchName = filepath.Join(tarballDir, workingDirName, patchFile)

	// Set up signal handling to clean up after ourselves
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		cleanupWorkingDirectory(tempDir)
		os.Exit(1)
	}()

	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}
	defer func() {
		cleanupWorkingDirectory(tempDir)
	}()

	if err := createMeta(rev, full); err != nil {
		return fmt.Errorf("failed to create meta file: %w", err)
	}

	if !full {
		fmt.Fprint(os.Stderr, "Generating diff...\n")
		if err := generateDiff(rev); err != nil {
			return fmt.Errorf("failed to generate diff: %w", err)
		}
	}

	fmt.Fprint(os.Stderr, "Packaging directory...\n")

	if err := packageDirectory(full); err != nil {
		return fmt.Errorf("failed to package directory: %w", err)
	}

	var workspace string
	// if running in a pipeline/workflow we need to get the workspace associated with the API key
	userEndpoint, err := urlBuilder.Build(platformUrl, "/user")
	if err != nil {
		return err
	}
	var workspaceGetterErr error
	workspace, workspaceGetterErr = defaultWorkspaceGetter(*userEndpoint, accessToken)
	if workspaceGetterErr != nil {
		return fmt.Errorf("failed defaultWorkspaceGetter: %w", workspaceGetterErr)
	}

	apiEndpoint, err := urlBuilder.Build(platformUrl, "inspector/presign/bundle-upload")
	if err != nil {
		return err
	}

	presignedUrl, err := presignedURLGetter(*apiEndpoint, accessToken, tarballName, workspace, full)
	if err != nil {
		return fmt.Errorf("failed to get presigned URL: %w", err)
	}

	fmt.Fprint(os.Stderr, "Uploading package repo...\n")

	if err := fileUploader(presignedUrl, filepath.Join(tarballDir, tarballName)); err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	workspaceID, userID, epoch, isMachine, err := urlBuilder.GetIDsFromUrl(presignedUrl)
	if err != nil {
		return err
	}

	sortString := urlBuilder.CreateSortString(userID, epoch, full, isMachine)

	consoleFullUrl, err := urlBuilder.Build(consoleUrl, "workspaces", workspaceID, "analysis", sortString, "result")
	if err != nil {
		return err
	}

	fmt.Fprint(os.Stderr, "Upload successful, your scan is processing!\n")
	// We print the URL when it is completed, but that doesn't help if it fails
	// for some reason and the user needs to contact support.
	fmt.Fprintf(os.Stderr, "Once completed, you can see results at: %s\n", *consoleFullUrl)

	// Wait for results if the user wants, or exit immediately
	if wait {
		return queryForResult(platformUrl, epoch, accessToken, consoleFullUrl, workspace, outputFormat)
	}
	return nil
}

func cleanupWorkingDirectory(tempDir string) {
	_ = os.RemoveAll(tempDir)
}

func queryForResult(platformUrl string, epoch string, accessToken string, consoleFullUrl *string, workspace, outputFormat string) error {
	maxAttempts := 750
	attempt := 0
	sleepDuration := time.Second

	// Create spinner for stderr
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = os.Stderr // Send spinner to stderr
	s.Prefix = "Analysis in progress... "
	s.FinalMSG = "✓ Results found!\n"
	s.Start()

	// Ensure spinner stops no matter what
	defer s.Stop()

	client := &http.Client{Timeout: 10 * time.Second}

	for attempt < maxAttempts {
		attempt++

		// Build URL
		fullURL := fmt.Sprintf("%s/inspector/result/user?sortKey=%s&op=beginswith",
			strings.TrimSuffix(platformUrl, "/"),
			epoch)

		// Create HTTP client
		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
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
				if results[0].Analysis != nil {
					// Stop spinner before outputting results
					s.FinalMSG = "✓ Analysis complete!\n"
					s.Stop()

					// Check output format
					if outputFormat == "sarif" {
						// Output sarif format
						sarifOutput, err := sarif.ConvertToSARIF(results[0].Analysis.RawLLMAnalysis, *consoleFullUrl)
						if err != nil {
							return fmt.Errorf("failed to convert to SARIF: %w", err)
						}

						fmt.Fprintf(os.Stderr, "You can also view your results here: %s\n", *consoleFullUrl)
						fmt.Print(sarifOutput) // stdout
						return nil
					}

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

				slices.SortFunc(results, func(a, b api.UserInspectorResult) int {
					if a.StatusMeta.UpdatedAt < b.StatusMeta.UpdatedAt {
						return 1
					}

					if a.StatusMeta.UpdatedAt == b.StatusMeta.UpdatedAt {
						return 0
					}

					return -1
				})

				status := results[0].StatusMeta.Status

				prefix := status
				if len(status) >= 1 {
					prefix = strings.ToUpper(results[0].StatusMeta.Status[:1]) + results[0].StatusMeta.Status[1:]
				}

				s.Prefix = prefix + " "

				if status == "failed" {
					s.FinalMSG = prefix
					s.Stop()
					fmt.Fprintln(os.Stderr)
					if results[0].StatusMeta.Details != "" {
						fmt.Fprintf(os.Stderr, "Error: %s\n", results[0].StatusMeta.Details)
					}
					return errors.New("processing failed after uploading")
				}
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
