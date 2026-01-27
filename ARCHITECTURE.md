# Kusari CLI Architecture

## Overview

Kusari CLI is a command-line interface tool written in Go for interacting with the Kusari platform. It provides authentication, repository scanning, and risk analysis capabilities.

## System Components

```
kusari-cli/
├── kusari/           # Main application entry point
│   ├── main.go       # CLI bootstrap
│   └── cmd/          # Command implementations
├── api/              # API client layer
│   └── configuration/# Configuration API types
├── pkg/              # Shared packages
│   ├── auth/         # Authentication subsystem
│   ├── config/       # Configuration management
│   ├── repo/         # Repository operations
│   ├── sarif/        # SARIF format handling
│   └── url/          # URL utilities
└── docs/             # Documentation
```

## Core Subsystems

### 1. Command Layer (`kusari/cmd/`)

The command layer uses [Cobra](https://github.com/spf13/cobra) for CLI structure:

- **root.go**: Base command and global flags
- **auth.go**: Authentication command group
  - `auth_login.go`: OAuth2/OIDC login flow
  - `auth_select_workspace.go`: Workspace selection
  - `auth_select_tenant.go`: Tenant selection
- **repo.go**: Repository command group
  - `repo_scan.go`: Diff-based repository scanning
  - `repo_risk_check.go`: Full repository risk analysis
- **configuration.go**: Configuration management commands

### 2. Authentication Subsystem (`pkg/auth/`)

Handles OAuth2/OIDC authentication with the Kusari platform:

| Component | Responsibility |
|-----------|----------------|
| `client.go` | OAuth2 client configuration |
| `oidc.go` | OpenID Connect token handling |
| `browser.go` | Browser-based auth flow |
| `httpservice.go` | Local callback server |
| `storage.go` | Token persistence |
| `workspace.go` | Workspace management |

**Authentication Flows:**
- **Browser Flow**: OAuth2 PKCE for interactive sessions
- **Client Credentials**: For CI/CD environments

### 3. Repository Operations (`pkg/repo/`)

Handles repository packaging, scanning, and uploads:

| Component | Responsibility |
|-----------|----------------|
| `packager.go` | Creates uploadable archives |
| `scanner.go` | Orchestrates scan workflow |
| `uploader.go` | Handles file uploads |
| `diff.go` | Git diff analysis |

### 4. API Layer (`api/`)

Provides typed interfaces to the Kusari platform API:

- `bundle.go`: Bundle/package operations
- `docstore.go`: Document storage operations
- `configuration/`: Configuration types

## Data Flow

### Authentication Flow

```
User → CLI → Browser → Auth Service
                           ↓
                    Callback Server
                           ↓
                    Token Storage (~/.kusari/tokens.json)
```

### Repository Scan Flow

```
User → CLI → Git Repository
              ↓
        Packager (archive)
              ↓
        Uploader → Platform API
                        ↓
                   Scan Results → Console
```

## Configuration

### Runtime Configuration

- **Viper**: Configuration management with env/flag/file support
- **Cobra**: Command-line parsing and help generation

### Storage Locations

| File | Purpose |
|------|---------|
| `~/.kusari/tokens.json` | OAuth2 tokens |
| `~/.kusari/workspace.json` | Active workspace |

## Security Considerations

- Tokens stored with restrictive permissions (0600)
- All API communication over HTTPS
- OAuth2 PKCE for browser authentication
- No secrets logged or exposed in verbose output

See [THREAT_MODEL.md](THREAT_MODEL.md) for detailed security analysis.

## Build and Release

- **Go Modules**: Dependency management via `go.mod`
- **GoReleaser**: Cross-platform binary builds
- **ko**: Container image builds
- **SLSA**: Provenance generation for releases

## Dependencies

See [DEPENDENCIES.md](DEPENDENCIES.md) for the complete dependency list and their purposes.
