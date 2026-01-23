# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build
go build -v ./...

# Run tests
go test -v ./...

# Run a single package's tests
go test -v ./pkg/repo/...

# Run linter (same as CI)
golangci-lint run --timeout 3m --verbose

# Format code
find -name \*.go | xargs gofmt -w

# Tidy dependencies
go mod tidy

# Vet code
go vet ./...

# Install locally
go install ./kusari
```

The PR workflow runs: lint → build → test → vet → format check. All must pass.

## Architecture Overview

Kusari CLI is a Cobra-based CLI for Kusari's SBOM scanning and security platform.

### Package Structure

- **`kusari/`** - Main application entry point
  - `main.go` - Entry point with version injection (`-X main.version=...`)
  - `cmd/` - Cobra command definitions

- **`pkg/auth/`** - OAuth2/OIDC authentication
  - Token storage: `~/.kusari/tokens.json`
  - Workspace selection: `~/.kusari/workspace.json`
  - Supports browser-based and headless (client secret) flows

- **`pkg/repo/`** - Core scanning functionality
  - `scanner.go` - Orchestrates scan workflow (init → upload → poll → results)
  - `uploader.go` - Presigned URL uploads with retry logic
  - `packager.go` - Creates `.tar.bz2` archives respecting `.gitignore`

- **`pkg/sarif/`** - Converts Kusari results to SARIF 2.1.0 format

- **`pkg/configuration/`** - Generates/updates `kusari.yaml` config files

- **`api/`** - API type definitions (separate from implementation)

### Command Hierarchy

```
kusari
├── auth
│   ├── login           # OAuth2 login (browser or --client-secret)
│   └── select-workspace
├── repo
│   ├── scan <dir> <rev>   # Diff-based scan against git revision
│   └── risk-check <dir>   # Full repository risk analysis
├── platform
└── configuration
    ├── generate-config
    └── update-config
```

### Configuration

- **Environment variables**: `KUSARI_` prefix (e.g., `KUSARI_PLATFORM_URL`)
- **Flags**: `--platform-url`, `--console-url`, `--verbose`
- **Config file**: `.env` in current directory auto-loaded via Viper

### Key Patterns

- All files use copyright header: `// Copyright (c) Kusari <https://www.kusari.dev/>`
- Error handling: `fmt.Errorf("%w", err)` for wrapping
- Tests use `testify/assert` and table-driven patterns
- Mock HTTP servers for API testing in `*_test.go` files
