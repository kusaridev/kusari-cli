# Architecture

This document describes the high-level architecture of the Kusari CLI.

## Overview

The Kusari CLI is a command-line tool that interfaces with the Kusari platform to provide security scanning and analysis capabilities for software repositories. It is written in Go and follows a modular architecture.

## Directory Structure

```
kusari-cli/
├── api/                    # API client and data structures
│   └── configuration/      # Configuration API types
├── kusari/                 # Main application
│   └── cmd/                # CLI commands (Cobra)
├── pkg/                    # Reusable packages
│   ├── auth/               # Authentication handling
│   ├── config/             # Configuration management
│   ├── configuration/      # Configuration utilities
│   ├── constants/          # Application constants
│   ├── login/              # Login flow implementation
│   ├── port/               # Port generation utilities
│   ├── repo/               # Repository operations
│   ├── sarif/              # SARIF report handling
│   └── url/                # URL building utilities
└── docs/                   # Documentation
```

## Core Components

### Command Layer (`kusari/cmd/`)

The CLI uses [Cobra](https://github.com/spf13/cobra) for command management:

- **root.go**: Root command and global flags
- **auth.go**: Authentication command group
  - **auth_login.go**: Login flow implementation
  - **auth_select_tenant.go**: Tenant selection
  - **auth_select_workspace.go**: Workspace selection
- **repo.go**: Repository command group
  - **repo_scan.go**: Repository scanning
  - **repo_risk_check.go**: Risk assessment
- **configuration.go**: Configuration management commands
- **platform.go**: Platform interaction commands
- **upload.go**: Upload functionality

### Authentication Package (`pkg/auth/`)

Handles all authentication concerns:

- **client.go**: HTTP client with authentication
- **oidc.go**: OpenID Connect authentication flow
- **httpservice.go**: Local HTTP server for OAuth callback
- **storage.go**: Token persistence
- **workspace.go**: Workspace management
- **browser.go**: Browser launch for OAuth
- **state.go**: OAuth state management
- **types.go**: Authentication data types

### Repository Package (`pkg/repo/`)

Manages repository operations:

- **scanner.go**: Git repository scanning
- **packager.go**: Repository packaging for upload
- **uploader.go**: Package upload to Kusari platform
- **diff.go**: Git diff generation

### API Package (`api/`)

Client for Kusari platform API:

- **bundle.go**: Bundle/package management
- **docstore.go**: Document storage operations
- **configuration/config.go**: Configuration types

## Data Flow

### Authentication Flow

```
┌─────────┐    ┌──────────┐    ┌─────────────┐    ┌────────────┐
│  User   │───>│  CLI     │───>│ OIDC Server │───>│  Browser   │
└─────────┘    └──────────┘    └─────────────┘    └────────────┘
                    │                                    │
                    │         OAuth Callback             │
                    │<───────────────────────────────────┘
                    │
              ┌─────▼─────┐
              │  Token    │
              │  Storage  │
              └───────────┘
```

### Repository Scan Flow

```
┌─────────┐    ┌──────────┐    ┌───────────┐    ┌─────────────┐
│  User   │───>│  Scan    │───>│  Packager │───>│  Uploader   │
└─────────┘    │  Command │    └───────────┘    └─────────────┘
               └──────────┘                            │
                    │                                  │
              ┌─────▼─────┐                     ┌──────▼──────┐
              │  Git Diff │                     │   Kusari    │
              │  Analysis │                     │   Platform  │
              └───────────┘                     └─────────────┘
```

## Configuration

Configuration is managed through [Viper](https://github.com/spf13/viper):

- Configuration file: `$HOME/.kusari/config.yaml`
- Tokens: `$HOME/.kusari/tokens.json`
- Workspace: `$HOME/.kusari/workspace.json`

Environment variables can override configuration values with the `KUSARI_` prefix.

## Security Architecture

### Authentication
- OAuth 2.0 with PKCE for secure authentication
- OpenID Connect for identity verification
- Secure token storage with file permissions
- SSO/SAML support for enterprise authentication

### Data Protection
- HTTPS-only communication with Kusari platform
- No sensitive data logging
- Secure credential storage

### Build Security
- Reproducible builds with GoReleaser
- SLSA Level 3 provenance attestations
- Signed release artifacts with Sigstore/Cosign
- SBOM generation for transparency

## Extension Points

### Adding New Commands

1. Create a new file in `kusari/cmd/`
2. Define command using Cobra
3. Register with parent command in `init()`

### Adding New Packages

1. Create directory under `pkg/`
2. Follow Go package conventions
3. Keep dependencies minimal

## Testing

- Unit tests alongside source files (`*_test.go`)
- Integration tests in CI/CD pipeline
- Test coverage enforced via CI

## Build and Release

- Built with Go toolchain
- Released via GoReleaser
- Container images published to GHCR
- Multi-platform support (Linux, macOS, Windows)
