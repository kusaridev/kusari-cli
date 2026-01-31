# Architecture

This document describes the architecture of the Kusari CLI.

## Overview

Kusari CLI is a command-line interface written in Go that provides tools for software supply chain security. It communicates with the Kusari platform to perform repository scans and security analysis.

## Components

```
kusari-cli/
├── kusari/           # Main application entrypoint
│   └── main.go       # CLI entrypoint
├── cmd/              # Command implementations
│   ├── root.go       # Root command and global flags
│   ├── auth/         # Authentication commands
│   │   └── login.go  # OAuth2/OIDC authentication
│   └── repo/         # Repository commands
│       └── scan.go   # Repository scanning
├── internal/         # Internal packages
│   ├── auth/         # Authentication logic
│   ├── api/          # API client for Kusari platform
│   └── config/       # Configuration management
└── pkg/              # Public packages (if any)
```

## Key Technologies

- **Language**: Go 1.24+
- **CLI Framework**: [Cobra](https://github.com/spf13/cobra) for command structure
- **Configuration**: [Viper](https://github.com/spf13/viper) for configuration management
- **Authentication**: OAuth2/OIDC via `coreos/go-oidc`
- **Output Formatting**: [Glamour](https://github.com/charmbracelet/glamour) for terminal rendering

## Authentication Flow

```
┌──────────┐     ┌────────────┐     ┌───────────────┐
│  User    │────>│ Kusari CLI │────>│ Auth Provider │
└──────────┘     └────────────┘     └───────────────┘
                       │                    │
                       │  1. Initiate OAuth │
                       │───────────────────>│
                       │                    │
                       │  2. Browser opens  │
                       │<───────────────────│
                       │                    │
                       │  3. User logs in   │
                       │                    │
                       │  4. Callback token │
                       │<───────────────────│
                       │                    │
                       ▼                    │
              ┌─────────────────┐           │
              │ Store token in  │           │
              │ ~/.kusari/      │           │
              └─────────────────┘           │
```

## Repository Scanning Flow

```
┌──────────┐     ┌────────────┐     ┌─────────────────┐
│  User    │────>│ Kusari CLI │────>│ Kusari Platform │
└──────────┘     └────────────┘     └─────────────────┘
                       │                     │
                       │  1. Package repo    │
                       │                     │
                       │  2. Upload package  │
                       │────────────────────>│
                       │                     │
                       │  3. Scan results    │
                       │<────────────────────│
                       │                     │
                       ▼                     │
              ┌─────────────────┐            │
              │ Display results │            │
              │ in terminal     │            │
              └─────────────────┘            │
```

## Configuration

Configuration is stored in `~/.kusari/`:

- `tokens.json` - OAuth2 access and refresh tokens
- `workspace.json` - Selected workspace configuration

## Security Considerations

- Tokens are stored locally with user-only permissions
- All API communication uses HTTPS
- OAuth2 PKCE flow for secure authentication
- No credentials are logged or displayed

## Build and Release

- Releases are built using [GoReleaser](https://goreleaser.com/)
- Container images are built with [ko](https://ko.build/)
- SLSA Level 3 provenance is generated for release artifacts
- SBOMs are generated in CycloneDX format
