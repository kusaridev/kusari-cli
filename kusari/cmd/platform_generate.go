// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
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

Defaults "--warm-go-cache=per-workspace", "--no-deps-dev",
"--no-clearly-defined", and a format-appropriate "--output"
(project.cdx.json for CycloneDX, project.spdx.json for SPDX) are
supplied automatically. Pass any of these after "--" to override.

Anything after "--" is passed verbatim to mikebom as flags to "sbom scan".

When --upload is set, the generated SBOM is uploaded to the Kusari
platform after generation. Most "kusari platform upload" flags are
accepted; --file-path is replaced by --output, and the VEX-only
flags --openvex, --tag, and --sbom-subject are not applicable here.

Environment variables:
  KUSARI_MIKEBOM_BIN     Use this binary instead of downloading.
  KUSARI_NO_AUTO_INSTALL If "1", fail rather than download on first run.

Examples:
  kusari platform generate -- --path .
  kusari platform generate --upload --tenant demo -- --path .
  kusari platform generate --upload --forge github.com --org myorg \
    --repo myrepo -- --path .`,
		Args: cobra.ArbitraryArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			if !generateUpload {
				return
			}
			uploadPreRun(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			userFormat := flagValue(args, "--format")

			// --upload can only carry one file to the platform. Reject the
			// multi-format and multi-output cases loudly rather than silently
			// uploading only the first.
			if generateUpload && (countFlag(args, "--output") > 1 || strings.Contains(userFormat, ",")) {
				return fmt.Errorf("--upload does not support multi-format generation; pass at most one --format and at most one --output")
			}

			defaultOutput := defaultSbomFilename(userFormat)

			scanArgs := []string{"sbom", "scan"}
			if !hasFlag(args, "--warm-go-cache") {
				scanArgs = append(scanArgs, "--warm-go-cache=per-workspace")
			}
			if !hasFlag(args, "--no-deps-dev") {
				scanArgs = append(scanArgs, "--no-deps-dev")
			}
			if !hasFlag(args, "--no-clearly-defined") {
				scanArgs = append(scanArgs, "--no-clearly-defined")
			}
			if !hasFlag(args, "--output") {
				scanArgs = append(scanArgs, "--output", defaultOutput)
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
				sbomOutputPath(args, defaultOutput),
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
				uploadResultsFile,
				uploadMapComponents,
			)
		},
	}
	cmd.Flags().BoolVar(&generateUpload, "upload", false, "After generating, upload the SBOM to the Kusari platform")
	addUploadFlags(cmd, false)
	// These flags are VEX-document-wrapper metadata and don't apply to the
	// SBOM that "mikebom sbom scan" produces. Hide them from help to avoid
	// the confusing errors a user would hit downstream in repo.Upload's
	// OpenVEX validation.
	for _, flag := range []string{"openvex", "tag", "sbom-subject"} {
		if err := cmd.Flags().MarkHidden(flag); err != nil {
			panic(err)
		}
	}
	return cmd
}

func hasFlag(args []string, name string) bool {
	return countFlag(args, name) > 0
}

func countFlag(args []string, name string) int {
	n := 0
	for _, a := range args {
		if a == name || strings.HasPrefix(a, name+"=") {
			n++
		}
	}
	return n
}

// sbomOutputPath returns the path that mikebom will write the SBOM to,
// derived from any user-supplied --output flag (handles "--output PATH",
// "--output=PATH", and the per-format "--output FMT=PATH" form). When the
// user did not supply --output, defaultPath is returned — this must match
// the value injected as the --output default in scanArgs so the upload
// step finds the file mikebom actually wrote.
func sbomOutputPath(args []string, defaultPath string) string {
	raw := flagValue(args, "--output")
	if raw == "" {
		return defaultPath
	}
	if _, path, ok := strings.Cut(raw, "="); ok {
		return path
	}
	return raw
}

// defaultSbomFilename picks a sensible default output path for mikebom's
// "--format" value. Without this, "--format spdx-3-json" would write SPDX
// content to project.cdx.json (the previous hardcoded default), producing
// a misleading file name.
func defaultSbomFilename(format string) string {
	switch {
	case format == "" || format == "cyclonedx-json":
		// Empty matches mikebom's own default (cyclonedx-json).
		return "project.cdx.json"
	case strings.HasPrefix(format, "spdx-"):
		return "project.spdx.json"
	default:
		// Unknown / future format: avoid pretending it's CycloneDX.
		return "project.sbom.json"
	}
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
