# Kusari CLI

Command line interface for Kusari.

## Installation

1. Have [Go installed](https://go.dev/doc/install).

1. `go install github.com/kusaridev/kusari-cli/kusari@latest`

Alternatively, you can install pre-built binaries for supported platforms from
the [GitHub releases page](https://github.com/kusaridev/kusari-cli/releases).

## Usage

For detailed information, see the [Kusari Documentation](https://docs.kusari.cloud/reference/CLI/).

When enabled in a CI/CD environment, Kusari Inspector via the `repo scan` command will:
- Post a summary comment with security findings
- Post inline comments on specific lines of code where issues are detected
- Update existing comments instead of creating duplicates on subsequent runs

**CI/CD Setup Instructions:**

For complete setup instructions, templates, and reusable workflows for both GitLab and GitHub, see the [Kusari CI Templates repository](https://github.com/kusaridev/kusari-ci-templates).
