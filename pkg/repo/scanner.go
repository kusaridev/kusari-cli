// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
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
	"github.com/kusaridev/kusari-cli/pkg/login"
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
	presignedURLGetter     func(apiEndpoint string, jwtToken string, filePath, workspace string, full bool, size int64) (string, error)
	defaultWorkspaceGetter func(platformUrl string, jwtToken string) ([]login.Workspace, map[string][]string, error)
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

	// Check if this is a monorepo - only for risk checks (full scans)
	// Diff scans can work fine on monorepos since they're analyzing changes
	if full {
		isMonoRepo, indicators, err := detectMonoRepo(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error checking for monorepo: %v\n", err)
		}
		if isMonoRepo {
			fmt.Fprintf(os.Stderr, "Error: Monorepo detected in %s\n", dir)
			fmt.Fprintf(os.Stderr, "\nMonorepo indicators found:\n")
			for _, indicator := range indicators {
				fmt.Fprintf(os.Stderr, "  - %s\n", indicator)
			}
			fmt.Fprintf(os.Stderr, "\nKusari Inspector works best when analyzing individual repositories.\n")
			fmt.Fprintf(os.Stderr, "Please run risk-check on each sub-project directory separately.\n")
			fmt.Fprintf(os.Stderr, "\nFor example:\n")
			fmt.Fprintf(os.Stderr, "  kusari repo risk-check ./packages/project1\n")
			fmt.Fprintf(os.Stderr, "  kusari repo risk-check ./packages/project2\n")
			os.Exit(1)
		}
	}

	fileUploader := uploadFileToS3
	presignedURLGetter := getPresignedURL
	defaultWorkspaceGetter := login.FetchWorkspaces
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

	meta, err := createMeta(rev, full)
	if err != nil {
		return fmt.Errorf("failed to create meta file: %w", err)
	}

	if !full {
		fmt.Fprint(os.Stderr, "Generating diff...\n")
		if err := generateDiff(rev); err != nil {
			return fmt.Errorf("failed to generate diff: %w", err)
		}
	}

	fmt.Fprint(os.Stderr, "Packaging directory...\n")

	size, err := packageDirectory(full)
	if err != nil {
		return fmt.Errorf("failed to package directory: %w", err)
	}

	var workspace string
	var workspaceDescription string

	// Load the stored workspace for the current platform
	// Pass empty string for authEndpoint as it's not available during scans and only validated during login
	storedWorkspace, err := auth.LoadWorkspace(platformUrl, "")
	if err != nil {
		// If no workspace is stored or platform changed, try to fetch and use first workspace
		workspaces, _, workspaceGetterErr := defaultWorkspaceGetter(platformUrl, accessToken)
		if workspaceGetterErr != nil {
			return fmt.Errorf("failed to get workspaces: %w. Please run `kusari auth login` to select a workspace", workspaceGetterErr)
		}

		// Use the first workspace as fallback (for CI/CD workflows)
		workspace = workspaces[0].ID
		workspaceDescription = workspaces[0].Description
		fmt.Fprintf(os.Stderr, "Using workspace: %s\n", workspaceDescription)
	} else {
		workspace = storedWorkspace.ID
		workspaceDescription = storedWorkspace.Description
		fmt.Fprintf(os.Stderr, "Using workspace: %s\n", workspaceDescription)
	}

	apiEndpoint, err := urlBuilder.Build(platformUrl, "inspector/presign/bundle-upload")
	if err != nil {
		return err
	}

	presignedUrl, err := presignedURLGetter(*apiEndpoint, accessToken, tarballName, workspace, full, size)
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

	var consoleFullUrl *string
	var consoleUrlErr error
	if !full {
		consoleFullUrl, consoleUrlErr = urlBuilder.Build(consoleUrl, "workspaces", workspaceID, "analysis", sortString, "result")
		if consoleUrlErr != nil {
			return consoleUrlErr
		}
	} else {
		// /workspaces/{{workspaceID}}/risk-check/{{repo}}/{{sortKey}}/result
		consoleFullUrl, consoleUrlErr = urlBuilder.Build(consoleUrl, "workspaces", workspaceID, "risk-check", meta.DirName, sortString, "result")
		if consoleUrlErr != nil {
			return consoleUrlErr
		}
	}

	fmt.Fprint(os.Stderr, "Upload successful, your scan is processing!\n")
	// We print the URL when it is completed, but that doesn't help if it fails
	// for some reason and the user needs to contact support.
	fmt.Fprintf(os.Stderr, "Once completed, you can see results at: %s\n", *consoleFullUrl)

	// Wait for results if the user wants, or exit immediately
	if wait {
		return queryForResult(platformUrl, epoch, accessToken, consoleFullUrl, workspace, outputFormat, full)
	}
	return nil
}

func cleanupWorkingDirectory(tempDir string) {
	_ = os.RemoveAll(tempDir)
}

func queryForResult(platformUrl string, epoch string, accessToken string, consoleFullUrl *string, workspace, outputFormat string, full bool) error {
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

	scanType := "scan"
	if full {
		scanType = "risk-check"
	}
	// Build URL
	fullURL := fmt.Sprintf("%s/inspector/result/user?sortKey=%s&op=beginswith&scanType=%s",
		strings.TrimSuffix(platformUrl, "/"),
		epoch,
		scanType)

	for attempt < maxAttempts {
		attempt++

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

					if full {
						printFullScanResults(results[0].Analysis)
						return nil
					}

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

// detectMonoRepo checks if the directory appears to be a monorepo based on Kusari-relevant dependency files
// Returns true if monorepo indicators are found, along with detected patterns
func detectMonoRepo(path string) (bool, []string, error) {
	var indicators []string

	// Check for common monorepo configuration files in root
	monoRepoConfigFiles := []string{
		"lerna.json",
		"nx.json",
		"pnpm-workspace.yaml",
		"turbo.json",
		"rush.json",
		"lage.config.js",
		"workspace.json",
	}

	for _, configFile := range monoRepoConfigFiles {
		if _, err := os.Stat(filepath.Join(path, configFile)); err == nil {
			indicators = append(indicators, fmt.Sprintf("monorepo config: %s", configFile))
		}
	}

	// Check root package.json for workspaces field (npm/yarn/pnpm workspaces)
	packageJsonPath := filepath.Join(path, "package.json")
	if data, err := os.ReadFile(packageJsonPath); err == nil {
		if strings.Contains(string(data), "\"workspaces\"") {
			indicators = append(indicators, "package.json with workspaces")
		}
	}

	// Check root Cargo.toml for workspace field (Rust workspace)
	cargoTomlPath := filepath.Join(path, "Cargo.toml")
	if data, err := os.ReadFile(cargoTomlPath); err == nil {
		if strings.Contains(string(data), "[workspace]") {
			indicators = append(indicators, "Cargo.toml with [workspace]")
		}
	}

	// Check for multiple Kusari-relevant dependency files in subdirectories
	// These are the key project manifest files that indicate separate projects
	manifestFiles := []string{
		"go.mod",           // Go
		"package.json",     // JavaScript/TypeScript (not lock files)
		"pom.xml",          // Java/Maven
		"Cargo.toml",       // Rust (not Cargo.lock)
		"requirements.txt", // Python
		"pyproject.toml",   // Python
		"Gemfile",          // Ruby (not lock file)
		"build.gradle",     // Gradle
	}

	// Directories that commonly contain tooling/docs/generated code, not separate projects
	excludedDirs := map[string]bool{
		"docs":       true,
		"doc":        true,
		"website":    true,
		".github":    true,
		"scripts":    true,
		"tools":      true,
		"tool":       true,
		"util":       true,
		"utils":      true,
		"utilities":  true,
		"examples":   true,
		"example":    true,
		"test":       true,
		"tests":      true,
		"generated":  true,
		"gen":        true,
		".generated": true,
	}

	// Directory name patterns that indicate non-project directories (partial matches)
	excludedPatterns := []string{
		"test",      // matches: test, tests, integrationtest, unittest, etc.
		"mock",      // matches: mock, mocks, mockdata, etc.
		"fixture",   // matches: fixture, fixtures, etc.
		"generated", // matches: generated, .generated, codegen, etc.
	}

	manifestCounts := make(map[string]int)
	manifestLocations := make(map[string][]string) // Track locations for reporting
	totalManifests := 0

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		// Skip .git, node_modules, vendor, and other common directories
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == "target" || name == ".venv" || name == "venv" {
				return filepath.SkipDir
			}
		}

		// Check if this is a manifest file and not in the root
		for _, manifest := range manifestFiles {
			if info.Name() == manifest && p != filepath.Join(path, manifest) {
				// Get the relative path and check if it's in an excluded directory
				relPath, err := filepath.Rel(path, p)
				if err != nil {
					continue
				}

				// Check if the manifest is in an excluded directory
				pathParts := strings.Split(filepath.Dir(relPath), string(filepath.Separator))
				inExcludedDir := false
				for _, part := range pathParts {
					// Check exact match
					if excludedDirs[part] {
						inExcludedDir = true
						break
					}
					// Check pattern match (case-insensitive)
					lowerPart := strings.ToLower(part)
					for _, pattern := range excludedPatterns {
						if strings.Contains(lowerPart, pattern) {
							inExcludedDir = true
							break
						}
					}
					if inExcludedDir {
						break
					}
				}

				if !inExcludedDir {
					manifestCounts[manifest]++
					manifestLocations[manifest] = append(manifestLocations[manifest], relPath)
					totalManifests++
				}
			}
		}
		return nil
	})

	if err != nil {
		return false, nil, err
	}

	// Report which manifests were found multiple times (same type)
	for manifest, count := range manifestCounts {
		if count >= 2 {
			indicators = append(indicators, fmt.Sprintf("multiple %s files in subdirectories", manifest))
		}
	}

	// Check for polyglot monorepo: 2+ manifests of different types
	if totalManifests >= 2 && len(manifestCounts) >= 2 {
		var types []string
		for manifest := range manifestCounts {
			types = append(types, manifest)
		}
		indicators = append(indicators, fmt.Sprintf("multiple project types detected: %s", strings.Join(types, ", ")))
	}

	return len(indicators) > 0, indicators, nil
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

func printFullScanResults(a *api.Analysis) {
	sb := new(strings.Builder)

	fmt.Fprintf(sb, "## Overall Score: %d/5\n", a.Score)
	fmt.Fprintf(sb, "%s\n\n", a.Results)

	keys := slices.Collect(maps.Keys(a.Health))
	slices.Sort(keys)

	for _, key := range keys {
		fmt.Fprintf(sb, "### %s Score: %d/5\n", titleize(key), a.Health[key].Score)
		for _, datum := range a.Health[key].Summary.Data {
			fmt.Fprintf(sb, "#### %s:\n", datum.Label)
			for _, value := range datum.Values {
				fmt.Fprintln(sb, value)
				fmt.Fprintln(sb)
			}
			fmt.Fprintln(sb)
		}
		for _, check := range a.Health[key].Checks {
			fmt.Fprintf(sb, "#### %s: %v\n", check.Name, check.Pass)
			fmt.Fprintf(sb, "##### %s:\n", check.Data.Label)
			for _, value := range check.Data.Values {
				fmt.Fprintln(sb, value)
				fmt.Fprintln(sb)
			}
			fmt.Fprintln(sb)
		}
		fmt.Fprintln(sb)
	}

	cleanedContent := removeImageLines(sb.String())
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		fmt.Print(cleanedContent) // stdout
		return
	}

	rendered, err := r.Render(cleanedContent)
	if err != nil {
		fmt.Print(cleanedContent) // stdout
		return
	}

	fmt.Print(rendered) // stdout
}

func titleize(s string) string {
	return strings.ToUpper(s[0:1]) + s[1:]
}
