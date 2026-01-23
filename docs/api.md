# Kusari CLI API Reference

This document describes the command-line interface for the Kusari CLI.

## Global Options

These options are available for all commands:

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--console-url` | `KUSARI_CONSOLE_URL` | `https://console.us.kusari.cloud/` | Kusari Console URL |
| `--platform-url` | `KUSARI_PLATFORM_URL` | `https://platform.api.us.kusari.cloud/` | Kusari Platform API URL |
| `-v, --verbose` | `KUSARI_VERBOSE` | `false` | Enable verbose output |
| `--version` | - | - | Display version information |
| `-h, --help` | - | - | Display help for command |

## Configuration

The CLI supports configuration via:

1. **Command-line flags** (highest priority)
2. **Environment variables** (prefix: `KUSARI_`)
3. **Configuration file** (`.env` in current directory)

Environment variable names use underscores instead of hyphens (e.g., `KUSARI_PLATFORM_URL`).

---

## Commands

### kusari auth

Authentication commands for the Kusari platform.

#### kusari auth login

Authenticate with the Kusari platform.

```bash
kusari auth login [flags]
```

**Flags:**

| Flag | Short | Environment Variable | Default | Description |
|------|-------|---------------------|---------|-------------|
| `--auth-endpoint` | `-p` | `KUSARI_AUTH_ENDPOINT` | `https://auth.us.kusari.cloud/` | Authentication endpoint URL |
| `--client-id` | `-c` | `KUSARI_CLIENT_ID` | `4lnk6jccl3hc4lkcudai5lt36u` | OAuth2 client ID |
| `--client-secret` | `-s` | `KUSARI_CLIENT_SECRET` | - | OAuth2 client secret (for CI/CD) |

**Examples:**

```bash
# Interactive login (opens browser)
kusari auth login

# CI/CD login with client credentials
kusari auth login --client-secret <secret> --client-id <id>

# Login to development environment
kusari auth login --platform-url https://platform.api.dev.kusari.cloud/ \
  --auth-endpoint https://auth.dev.kusari.cloud/
```

**Token Storage:**

Tokens are stored in `~/.kusari/tokens.json` with restricted permissions.

---

#### kusari auth select-workspace

Change the active workspace without re-authenticating.

```bash
kusari auth select-workspace
```

**Behavior:**

- Displays current workspace
- Lists available workspaces
- Prompts for selection
- Stores selection in `~/.kusari/workspace.json`

---

### kusari repo

Repository scanning commands.

#### kusari repo scan

Scan a diff on a git repository using Kusari Inspector.

```bash
kusari repo scan <directory> <git-rev> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `<directory>` | Yes | Path to git repository |
| `<git-rev>` | Yes | Git revision to compare with working tree |

**Flags:**

| Flag | Short | Environment Variable | Default | Description |
|------|-------|---------------------|---------|-------------|
| `--wait` | `-w` | `KUSARI_WAIT` | `true` | Wait for scan results |
| `--output-format` | - | `KUSARI_OUTPUT_FORMAT` | `markdown` | Output format (`markdown` or `sarif`) |

**Examples:**

```bash
# Scan changes since last commit
kusari repo scan ~/git/myproject HEAD^

# Scan changes since main branch
kusari repo scan ~/git/myproject origin/main

# Output results in SARIF format
kusari repo scan ~/git/myproject HEAD^ --output-format sarif

# Don't wait for results (async)
kusari repo scan ~/git/myproject HEAD^ --wait=false
```

**Exit Codes:**

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | Error (authentication, network, invalid arguments) |

---

#### kusari repo risk-check

Perform a full repository risk analysis using Kusari Inspector.

```bash
kusari repo risk-check <directory> [flags]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `<directory>` | Yes | Path to git repository |

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--wait` | `-w` | `true` | Wait for analysis results |
| `--scan-subprojects` | - | `false` | Automatically scan detected subprojects in monorepos |

**Examples:**

```bash
# Full risk check on repository
kusari repo risk-check ~/git/myproject

# Scan a monorepo with subprojects
kusari repo risk-check ~/git/monorepo --scan-subprojects

# Scan specific subproject
kusari repo risk-check ~/git/monorepo/packages/api
```

---

### kusari configuration

Configuration file management commands.

#### kusari configuration generate-config

Generate a new `kusari.yaml` configuration file.

```bash
kusari configuration generate-config [flags]
```

#### kusari configuration update-config

Update an existing `kusari.yaml` configuration file.

```bash
kusari configuration update-config [flags]
```

---

## Authentication Flow

### Browser Flow (Interactive)

1. CLI opens browser to authentication endpoint
2. User authenticates via OAuth2/OIDC
3. Browser redirects to localhost callback
4. CLI receives tokens and stores them

### Client Credentials Flow (CI/CD)

1. CLI sends client ID and secret directly
2. Receives tokens without browser interaction
3. First available workspace is auto-selected

---

## Output Formats

### Markdown (default)

Human-readable format with tables and formatting for terminal display.

### SARIF

[Static Analysis Results Interchange Format](https://sarifweb.azurewebsites.net/) for integration with:
- GitHub Code Scanning
- Azure DevOps
- Other SARIF-compatible tools

---

## Error Handling

Common error scenarios:

| Error | Cause | Resolution |
|-------|-------|------------|
| `authentication required` | No valid tokens | Run `kusari auth login` |
| `workspace not selected` | No workspace configured | Run `kusari auth select-workspace` |
| `invalid git revision` | Revision not found | Verify revision exists in repository |
| `network error` | Platform unreachable | Check network and platform URL |

---

## See Also

- [Kusari Documentation](https://docs.kusari.cloud/docs/CLI/)
- [Kusari Console](https://console.us.kusari.cloud/)
- [GitHub Releases](https://github.com/kusaridev/kusari-cli/releases)
