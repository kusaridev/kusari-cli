// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/kusaridev/kusari-cli/pkg/repo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	uploadFilePath      string
	uploadAlias         string
	uploadDocumentType  string
	uploadOpenVex       bool
	uploadTag           string
	uploadSoftwareID    string
	uploadSbomSubject   string
	uploadComponentName string
	uploadCheckBlocked  bool
)

func init() {
	uploadcmd.Flags().StringVarP(&uploadFilePath, "file-path", "f", "", "Path to file or directory to upload (required)")
	uploadcmd.Flags().StringVarP(&uploadAlias, "alias", "a", "", "Alias that supersedes the subject in Kusari platform (optional)")
	uploadcmd.Flags().StringVarP(&uploadDocumentType, "document-type", "d", "", "Type of the document (image or build) sbom (optional)")
	uploadcmd.Flags().BoolVar(&uploadOpenVex, "open-vex", false, "Indicate that this is an OpenVEX document (optional, only works with files)")
	uploadcmd.Flags().StringVar(&uploadTag, "tag", "", "Tag value to set in the document wrapper upload meta (optional, e.g. govulncheck)")
	uploadcmd.Flags().StringVar(&uploadSoftwareID, "software-id", "", "Kusari Platform Software ID value to set in the document wrapper upload meta (optional)")
	uploadcmd.Flags().StringVar(&uploadSbomSubject, "sbom-subject", "", "Kusari Platform Software sbom subject substring value to set in the document wrapper upload meta (optional)")
	uploadcmd.Flags().StringVar(&uploadComponentName, "component-name", "", "Kusari Platform component name (optional)")
	uploadcmd.Flags().BoolVar(&uploadCheckBlocked, "check-blocked-packages", false, "Check if any of the SBOMs uses a package contained in the blocked package list")

	// Bind flags to viper
	mustBindPFlag("file-path", uploadcmd.Flags().Lookup("file-path"))
	mustBindPFlag("alias", uploadcmd.Flags().Lookup("alias"))
	mustBindPFlag("document-type", uploadcmd.Flags().Lookup("document-type"))
	mustBindPFlag("open-vex", uploadcmd.Flags().Lookup("open-vex"))
	mustBindPFlag("tag", uploadcmd.Flags().Lookup("tag"))
	mustBindPFlag("software-id", uploadcmd.Flags().Lookup("software-id"))
	mustBindPFlag("sbom-subject", uploadcmd.Flags().Lookup("sbom-subject"))
	mustBindPFlag("component-name", uploadcmd.Flags().Lookup("component-name"))
	mustBindPFlag("check-blocked-packages", uploadcmd.Flags().Lookup("check-blocked-packages"))
}

func upload() *cobra.Command {
	uploadcmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		return repo.Upload(
			uploadFilePath,
			platformTenantEndpoint,
			uploadAlias,
			uploadDocumentType,
			uploadOpenVex,
			uploadTag,
			uploadSoftwareID,
			uploadSbomSubject,
			uploadComponentName,
			uploadCheckBlocked,
			verbose,
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
    --open-vex --tag govulncheck --software-id 12345

  # CI/CD: Upload with blocked package checking
  kusari platform upload --file-path sbom.json --tenant demo \
    --check-blocked-packages

  # Dev/Testing: Upload using full tenant endpoint (overrides --tenant)
  kusari platform upload --file-path sbom.json --tenant-endpoint https://demo.api.dev.kusari.cloud`,
	Args: cobra.NoArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Update from viper (this gets env vars + config + flags)
		uploadFilePath = viper.GetString("file-path")
		uploadAlias = viper.GetString("alias")
		uploadDocumentType = viper.GetString("document-type")
		uploadOpenVex = viper.GetBool("open-vex")
		uploadTag = viper.GetString("tag")
		uploadSoftwareID = viper.GetString("software-id")
		uploadSbomSubject = viper.GetString("sbom-subject")
		uploadComponentName = viper.GetString("component-name")
		uploadCheckBlocked = viper.GetBool("check-blocked-packages")
	},
}
