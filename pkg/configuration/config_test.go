// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package configuration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test generating a new file when none exists
func TestGenerate(t *testing.T) {
	// Get the current directory so that we can change back to it later
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// Create a temporary test directory
	testDir := t.TempDir()
	sourceFile := filepath.Join(cwd, "testdata", "config-default.yaml")
	destFile := "kusari.yaml"

	require.NoError(t, os.Chdir(testDir))
	require.NoFileExists(t, destFile)

	require.NoError(t, GenerateConfig(false))
	require.True(t, compareHashes(sourceFile, destFile))

	require.NoError(t, os.Chdir(cwd))
}

// Test generating a new file when one exists
func TestGenerateWithExisting(t *testing.T) {
	// Get the current directory so that we can change back to it later
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// Create a temporary test directory
	testDir := t.TempDir()
	// Copy the test file
	sourceFile := filepath.Join(cwd, "testdata", "config-default.yaml")
	destFile := filepath.Join(testDir, "kusari.yaml")
	require.NoError(t, runCommand("cp", sourceFile, destFile))
	require.NoError(t, os.Chdir(testDir))

	// Try to generate a new file when one exists. This should fail.
	require.ErrorContains(t, GenerateConfig(false), "not overwriting")
	// Make sure they match!
	require.True(t, compareHashes(sourceFile, destFile))

	// Try to force overwriting the existing file. This should succeed.
	require.NoError(t, GenerateConfig(true))
	// Make sure they match!
	require.True(t, compareHashes(sourceFile, destFile))

	require.NoError(t, os.Chdir(cwd))
}

// Test that update-config produces a default config file when none already exists
func TestUpdateWithNoFile(t *testing.T) {
	// Get the current directory so that we can change back to it later
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// Create a temporary test directory
	testDir := t.TempDir()
	sourceFile := filepath.Join(cwd, "testdata", "config-default.yaml")
	destFile := "kusari.yaml"

	// Make sure there's no file
	require.NoError(t, os.Chdir(testDir))
	require.NoFileExists(t, destFile)

	// Write the file
	require.NoError(t, UpdateConfig())

	// Make sure the new file matches the test data
	require.True(t, compareHashes(sourceFile, destFile))

	require.NoError(t, os.Chdir(cwd))
}

// Test that the update function doesn't change user configs
func TestUpdateWithChanges(t *testing.T) {
	// Get the current directory so that we can change back to it later
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// Create a temporary test directory
	testDir := t.TempDir()
	// Copy the test file
	sourceFile := filepath.Join(cwd, "testdata", "config-changed.yaml")
	destFile := filepath.Join(testDir, "kusari.yaml")
	require.NoError(t, runCommand("cp", sourceFile, destFile))
	require.NoError(t, os.Chdir(testDir))

	// Write the file
	require.NoError(t, UpdateConfig())

	// Make sure the new file kept the user change
	expectedConfig := "github_action_version_pinning_check_enabled: false"
	readContent, err := os.ReadFile(destFile)
	require.NoError(t, err)
	require.Contains(t, string(readContent), expectedConfig)

	require.NoError(t, os.Chdir(cwd))
}

// Test that the update function adds missing configs
func TestUpdateAddMissing(t *testing.T) {
	// Get the current directory so that we can change back to it later
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// Create a temporary test directory
	testDir := t.TempDir()
	// Copy the test file
	sourceFile := filepath.Join(cwd, "testdata", "config-missing.yaml")
	destFile := filepath.Join(testDir, "kusari.yaml")
	desiredFile := filepath.Join(cwd, "testdata", "config-default.yaml")
	require.NoError(t, runCommand("cp", sourceFile, destFile))
	require.NoError(t, os.Chdir(testDir))

	// Write the file
	require.NoError(t, UpdateConfig())

	// Check to make sure all of the missing configs were added
	require.True(t, compareHashes(desiredFile, destFile))

	require.NoError(t, os.Chdir(cwd))
}

// Test that SBOM configuration fields are properly preserved during update
func TestUpdatePreservesSBOMConfig(t *testing.T) {
	// Get the current directory so that we can change back to it later
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// Create a temporary test directory
	testDir := t.TempDir()
	// Copy the test file
	sourceFile := filepath.Join(cwd, "testdata", "config-sbom-enabled.yaml")
	destFile := filepath.Join(testDir, "kusari.yaml")
	require.NoError(t, runCommand("cp", sourceFile, destFile))
	require.NoError(t, os.Chdir(testDir))

	// Write the file
	require.NoError(t, UpdateConfig())

	// Make sure SBOM configs were preserved
	readContent, err := os.ReadFile(destFile)
	require.NoError(t, err)
	content := string(readContent)

	// Verify SBOM fields are preserved
	require.Contains(t, content, "sbom_generation_enabled: true")
	require.Contains(t, content, "sbom_component_name: my-custom-component")
	require.Contains(t, content, "sbom_subject_name_override: custom-subject")
	require.Contains(t, content, "sbom_subject_version_override: v1.0.0")

	require.NoError(t, os.Chdir(cwd))
}

// Test that default SBOM config has generation disabled
func TestDefaultSBOMConfigDisabled(t *testing.T) {
	require.False(t, DefaultConfig.SBOMGenerationEnabled, "SBOM generation should be disabled by default")
	require.Empty(t, DefaultConfig.SBOMComponentName, "SBOM component name should be empty by default")
	require.Empty(t, DefaultConfig.SBOMSubjectNameOverride, "SBOM subject name override should be empty by default")
	require.Empty(t, DefaultConfig.SBOMSubjectVersionOverride, "SBOM subject version override should be empty by default")
}

// Test that generated config omits empty SBOM string fields
func TestGeneratedConfigOmitsEmptySBOMFields(t *testing.T) {
	// Get the current directory so that we can change back to it later
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// Create a temporary test directory
	testDir := t.TempDir()

	require.NoError(t, os.Chdir(testDir))

	// Generate a new config file
	require.NoError(t, GenerateConfig(false))

	// Read the generated file
	readContent, err := os.ReadFile("kusari.yaml")
	require.NoError(t, err)
	content := string(readContent)

	// SBOM generation enabled should be present (it's a bool, not omitempty)
	require.Contains(t, content, "sbom_generation_enabled: false")

	// Empty string fields should NOT be present (they have omitempty)
	require.NotContains(t, content, "sbom_component_name")
	require.NotContains(t, content, "sbom_subject_name_override")
	require.NotContains(t, content, "sbom_subject_version_override")

	require.NoError(t, os.Chdir(cwd))
}

//
// Some helper functions along the way
//

// Run shell commands
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Compute a file's SHA256 hash
func computeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close() //nolint:errcheck

	hasher := sha256.New()

	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to copy file content to hasher: %w", err)
	}

	// Get the final hash sum and convert it to a hexadecimal string.
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Check that two files have the same SHA256 hash
func compareHashes(fileOne string, fileTwo string) bool {
	hashOne, _ := computeFileHash(fileOne)
	hashTwo, _ := computeFileHash(fileTwo)

	//DEBUG
	fmt.Fprintf(os.Stderr, "Comparing %s to %s", fileOne, fileTwo)
	return hashOne == hashTwo
}
