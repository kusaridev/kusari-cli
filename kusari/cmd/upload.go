// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"

	"github.com/kusaridev/kusari-cli/v2/pkg/repo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	uploadFilePath                   string
	uploadAlias                      string
	uploadDocumentType               string
	uploadOpenVex                    bool
	uploadTag                        string
	uploadSoftwareID                 string
	uploadSbomSubject                string
	uploadComponentName              string
	uploadCheckBlocked               bool
	uploadSbomSubjectNameOverride    string
	uploadSbomSubjectVersionOverride string
	uploadWait                       bool
	uploadForge                      string
	uploadOrg                        string
	uploadRepo                       string
	uploadSubrepoPath                string
	uploadCommitSha                  string
)

// addUploadFlags registers the upload-related flags on a cobra command.
// When includeFilePath is true, --file-path is registered too; generate
// derives the file path from --output instead.
func addUploadFlags(cmd *cobra.Command, includeFilePath bool) {
	if includeFilePath {
		cmd.Flags().StringVarP(&uploadFilePath, "file-path", "f", "", "Path to file or directory to upload (required)")
	}
	cmd.Flags().StringVarP(&uploadAlias, "alias", "a", "", "Alias that supersedes the subject in Kusari platform (optional)")
	cmd.Flags().StringVarP(&uploadDocumentType, "document-type", "d", "", "Type of the document (image or build) sbom (optional)")
	cmd.Flags().BoolVar(&uploadOpenVex, "openvex", false, "Indicate that this is an OpenVEX document (optional, only works with files)")
	cmd.Flags().StringVar(&uploadTag, "tag", "", "Tag value to set in the document wrapper upload meta (optional, e.g. govulncheck)")
	cmd.Flags().StringVar(&uploadSoftwareID, "software-id", "", "Kusari Platform Software ID value to set in the document wrapper upload meta (optional)")
	cmd.Flags().StringVar(&uploadSbomSubject, "sbom-subject", "", "Kusari Platform Software sbom subject substring value to set in the document wrapper upload meta (optional, for OpenVEX docs only)")
	cmd.Flags().StringVar(&uploadComponentName, "component-name", "", "Kusari Platform component name (optional)")
	if err := cmd.Flags().MarkDeprecated("component-name", "see https://docs.us.kusari.cloud/software/components for info on how to assign and use Components"); err != nil {
		panic(err)
	}
	cmd.Flags().BoolVar(&uploadCheckBlocked, "check-blocked-packages", false, "Check if any of the SBOMs uses a package contained in the blocked package list")
	cmd.Flags().StringVar(&uploadSbomSubjectNameOverride, "sbom-subject-name-override", "", "SBOM Subject Name override (optional, for SBOMs only)")
	cmd.Flags().StringVar(&uploadSbomSubjectVersionOverride, "sbom-subject-version-override", "", "SBOM Subject Version override (optional, from SBOMs only)")
	cmd.Flags().BoolVar(&uploadWait, "wait", true, "Wait for ingestion status (default: true)")
	cmd.Flags().StringVar(&uploadForge, "forge", "", "Source forge for the SBOM (e.g., github.com, gitlab.com)")
	cmd.Flags().StringVar(&uploadOrg, "org", "", "Organization/owner name in the forge")
	cmd.Flags().StringVar(&uploadRepo, "repo", "", "Repository name in the forge")
	cmd.Flags().StringVar(&uploadSubrepoPath, "subrepo-path", "", "Path to subrepo within the repository (e.g., app/frontend)")
	cmd.Flags().StringVar(&uploadCommitSha, "commit-sha", "", "Commit SHA (from git) (optional, for SBOMs only)")
}

// uploadFlagKeys is the canonical list of upload-related viper keys, kept
// centralized so bindUploadFlagsToViper and loadUploadFromViper stay in
// sync with addUploadFlags.
var uploadFlagKeys = []string{
	"file-path", "alias", "document-type", "openvex", "tag",
	"software-id", "sbom-subject", "component-name", "check-blocked-packages",
	"sbom-subject-name-override", "sbom-subject-version-override",
	"wait", "forge", "org", "repo", "subrepo-path", "commit-sha",
}

// bindUploadFlagsToViper points viper at the upload-related flags on the
// given command. Called at PreRun time (not init) because viper holds one
// *pflag.Flag per key — only the active command's flag instances can be
// bound at once. Flags absent on cmd (e.g. --file-path on generate) are
// skipped.
func bindUploadFlagsToViper(cmd *cobra.Command) {
	for _, key := range uploadFlagKeys {
		if f := cmd.Flags().Lookup(key); f != nil {
			mustBindPFlag(key, f)
		}
	}
}

// loadUploadFromViper materializes env-var/config/CLI values into the
// package-level upload* vars in viper's precedence order.
func loadUploadFromViper() {
	uploadFilePath = viper.GetString("file-path")
	uploadAlias = viper.GetString("alias")
	uploadDocumentType = viper.GetString("document-type")
	uploadOpenVex = viper.GetBool("openvex")
	uploadTag = viper.GetString("tag")
	uploadSoftwareID = viper.GetString("software-id")
	uploadSbomSubject = viper.GetString("sbom-subject")
	uploadComponentName = viper.GetString("component-name")
	uploadCheckBlocked = viper.GetBool("check-blocked-packages")
	uploadSbomSubjectNameOverride = viper.GetString("sbom-subject-name-override")
	uploadSbomSubjectVersionOverride = viper.GetString("sbom-subject-version-override")
	uploadWait = viper.GetBool("wait")
	uploadForge = viper.GetString("forge")
	uploadOrg = viper.GetString("org")
	uploadRepo = viper.GetString("repo")
	uploadSubrepoPath = viper.GetString("subrepo-path")
	uploadCommitSha = viper.GetString("commit-sha")
}

// uploadPreRun wires both the rebind and the load. Reused by upload and
// "platform generate" so .env file and env-var values reach generate too.
func uploadPreRun(cmd *cobra.Command, _ []string) {
	bindUploadFlagsToViper(cmd)
	loadUploadFromViper()
}

func init() {
	addUploadFlags(uploadcmd, true)
}

func upload() *cobra.Command {
	uploadcmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// CLI uses of --component-name are warned about by cobra's MarkDeprecated.
		// Cover the config-file and env-var paths here.
		if uploadComponentName != "" && !cmd.Flags().Changed("component-name") {
			fmt.Fprintln(os.Stderr, "The component-name config value is no longer supported. "+
				"See https://docs.us.kusari.cloud/software/components for info on how to assign and use Components.")
		}

		return repo.Upload(
			uploadFilePath,
			platformTenantEndpoint,
			platformUrl,
			uploadAlias,
			uploadDocumentType,
			uploadOpenVex,
			uploadTag,
			uploadSoftwareID,
			uploadSbomSubject,
			uploadSbomSubjectNameOverride,
			uploadSbomSubjectVersionOverride,
			uploadCheckBlocked,
			uploadWait,
			uploadForge,
			uploadOrg,
			uploadRepo,
			uploadSubrepoPath,
			uploadCommitSha,
		)
	}

	return uploadcmd
}

var uploadcmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload SBOM or OpenVEX files to Kusari platform",
	Long: `Upload SBOM or OpenVEX files to Kusari platform using presigned S3 URLs.
Can upload individual files or entire directories.

Examples:
  # CI/CD: Upload using tenant name with API key (required in CI/CD)
  kusari platform upload --file-path sbom.json --tenant demo

  # Interactive user: Upload using stored tenant from login
  kusari platform upload --file-path sbom.json

  # CI/CD: Upload a directory of SBOMs
  kusari platform upload --file-path ./sboms/ --tenant demo

  # CI/CD: Upload an OpenVEX document with metadata
  kusari platform upload --file-path report.json --tenant demo \
    --openvex --tag govulncheck --software-id 12345

  # CI/CD: Upload with blocked package checking
  kusari platform upload --file-path sbom.json --tenant demo \
    --check-blocked-packages

  # CI/CD: Upload with repository traceability metadata
  kusari platform upload --file-path sbom.json --tenant demo \
    --forge github.com --org myorg --repo myrepo --subrepo-path app/frontend

  # Dev/Testing: Upload using full tenant endpoint (overrides --tenant)
  kusari platform upload --file-path sbom.json --tenant-endpoint https://demo.api.dev.kusari.cloud`,
	Args:   cobra.NoArgs,
	PreRun: uploadPreRun,
}
