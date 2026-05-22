// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"strings"

	"github.com/kusaridev/kusari-cli/v2/pkg/mikebom"
	"github.com/kusaridev/kusari-cli/v2/pkg/repo"
	"github.com/spf13/cobra"
)

var generateUpload bool

func generate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [--upload [upload-flags]] [-- mikebom-flags...]",
		Short: "Generate an SBOM (runs mikebom sbom scan)",
		Long: `Generate an SBOM by invoking "mikebom sbom scan". MikeBOM is
downloaded and verified on first use to ~/.kusari/bin/mikebom-<version>.

Defaults "--offline" and "--output project.cdx.json" are supplied
automatically. Pass --output or --offline=false after "--" to override.

Anything after "--" is passed verbatim to mikebom as flags to "sbom scan".

When --upload is set, the generated SBOM is uploaded to the Kusari
platform after generation. All "kusari platform upload" flags except
--file-path are accepted; the file path is taken from --output.

Environment variables:
  KUSARI_MIKEBOM_BIN     Use this binary instead of downloading.
  KUSARI_NO_AUTO_INSTALL If "1", fail rather than download on first run.

Examples:
  kusari platform generate -- --path .
  kusari platform generate --upload --tenant demo -- --path .
  kusari platform generate --upload --tag govulncheck --forge github.com \
    --org myorg --repo myrepo -- --path .`,
		Args: cobra.ArbitraryArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			if !generateUpload {
				return
			}
			uploadPreRun(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			scanArgs := []string{"sbom", "scan"}
			if !hasFlag(args, "--offline") {
				scanArgs = append(scanArgs, "--offline")
			}
			if !hasFlag(args, "--output") {
				scanArgs = append(scanArgs, "--output", "project.cdx.json")
			}
			scanArgs = append(scanArgs, args...)
			if err := mikebom.Run(cmd.Context(), scanArgs, os.Stdin, os.Stdout, os.Stderr); err != nil {
				return err
			}

			if !generateUpload {
				return nil
			}
			warnIfDeprecatedComponentName(cmd)
			return repo.Upload(
				sbomOutputPath(args),
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
		},
	}
	cmd.Flags().BoolVar(&generateUpload, "upload", false, "After generating, upload the SBOM to the Kusari platform")
	addUploadFlags(cmd, false)
	// --openvex doesn't make sense for an SBOM produced by "mikebom sbom scan";
	// hide it from help to avoid the confusing "tag must be specified" error a
	// user would hit downstream in repo.Upload's OpenVEX validation.
	if err := cmd.Flags().MarkHidden("openvex"); err != nil {
		panic(err)
	}
	return cmd
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name || strings.HasPrefix(a, name+"=") {
			return true
		}
	}
	return false
}

// sbomOutputPath returns the path that mikebom will write the SBOM to,
// derived from any user-supplied --output flag (handles "--output PATH",
// "--output=PATH", and the per-format "--output FMT=PATH" form).
func sbomOutputPath(args []string) string {
	raw := flagValue(args, "--output")
	if raw == "" {
		return "project.cdx.json"
	}
	if _, path, ok := strings.Cut(raw, "="); ok {
		return path
	}
	return raw
}

func flagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, name+"=") {
			return a[len(name)+1:]
		}
	}
	return ""
}
