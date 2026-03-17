# CLI Command Contracts

**Date**: 2026-03-03
**Feature**: 001-mcp-server

This document defines the CLI commands for the MCP server subcommand.

## Command: kusari mcp

Parent command for MCP server operations.

```
kusari mcp [command]

Available Commands:
  serve       Start the MCP server (for client spawning)
  install     Install the MCP server for a client
  uninstall   Remove the MCP server from a client
  list        List supported MCP clients and status

Flags:
  -h, --help   help for mcp

Global Flags:
      --console-url string    console url (default "https://console.us.kusari.cloud/")
      --platform-url string   platform url (default "https://platform.api.us.kusari.cloud/")
  -v, --verbose               Verbose output
```

---

## Command: kusari mcp serve

Start the MCP server process. Designed to be spawned by MCP clients, not run directly by users.

### Usage

```
kusari mcp serve [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--verbose` | bool | false | Enable verbose logging to stderr |

### Behavior

1. Loads configuration from environment and config file
2. Initializes MCP server with tool definitions
3. Runs server over stdio transport
4. Processes tool calls until client disconnects

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Normal shutdown (client disconnected) |
| 1 | Error during startup or execution |

### Example

```bash
# Typically spawned by MCP client, but can be run for testing:
kusari mcp serve --verbose
```

---

## Command: kusari mcp install

Install and configure the MCP server for a specific client.

### Usage

```
kusari mcp install <client> [flags]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `client` | Yes | Client identifier: `claude`, `cursor`, `windsurf`, `cline`, `continue` |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--verbose` | bool | false | Show detailed installation output |

### Behavior

1. Validates client identifier
2. Detects current platform (macOS/Linux/Windows)
3. Locates client configuration file
4. Creates config directory if needed
5. Adds or updates MCP server configuration
6. Displays post-install instructions

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Installation successful |
| 1 | Error (invalid client, permission denied, etc.) |

### Output

```
======================================================================
Kusari Inspector MCP Server - Installation
======================================================================

Configuring for: Claude Code
Platform: darwin

✓ Successfully configured Claude Code

======================================================================
Installation Complete!
======================================================================

Kusari Inspector has been configured for Claude Code.

Next steps:
1. Reload VS Code: Cmd+Shift+P → 'Developer: Reload Window'
2. Check MCP status - you should see 'kusari-inspector' running
3. Ask Claude to scan: 'Claude, scan my local changes for security issues'

For authentication:
- On first use, your browser will open to authenticate with Kusari
- Credentials are saved to ~/.kusari/tokens.json

======================================================================
```

### Examples

```bash
# Install for Claude Code
kusari mcp install claude

# Install for Cursor with verbose output
kusari mcp install cursor --verbose

# Install for Continue
kusari mcp install continue
```

---

## Command: kusari mcp uninstall

Remove the MCP server configuration from a client.

### Usage

```
kusari mcp uninstall <client> [flags]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `client` | Yes | Client identifier: `claude`, `cursor`, `windsurf`, `cline`, `continue` |

### Behavior

1. Validates client identifier
2. Locates client configuration file
3. Removes kusari-inspector from configuration
4. Preserves other MCP server configurations

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Uninstallation successful (or already not installed) |
| 1 | Error during uninstallation |

### Output

```
======================================================================
Kusari Inspector MCP Server - Uninstallation
======================================================================

✓ Removed Kusari Inspector from Claude Code

Restart Claude Code for changes to take effect.
```

### Example

```bash
kusari mcp uninstall claude
```

---

## Command: kusari mcp list

List all supported MCP clients and their installation status.

### Usage

```
kusari mcp list [flags]
```

### Behavior

1. Iterates through supported clients
2. Checks if each client's config file exists
3. Checks if kusari-inspector is configured
4. Displays status table

### Output Format

```
Supported MCP Clients:

  Client        Status      Config Path
  ──────        ──────      ───────────
  claude        installed   ~/Library/Application Support/Claude/claude_desktop_config.json
  cursor        not found   ~/.cursor/mcp.json
  windsurf      not found   ~/.codeium/windsurf/mcp_config.json
  cline         installed   ~/Library/Application Support/Code/User/.../cline_mcp_settings.json
  continue      not found   ~/.continue/config.json

To install: kusari mcp install <client>
```

### Status Values

| Status | Meaning |
|--------|---------|
| `installed` | kusari-inspector configured in client |
| `not configured` | Config file exists but kusari-inspector not present |
| `not found` | Client config file doesn't exist |

### Example

```bash
kusari mcp list
```

---

## Environment Variables

All commands respect these environment variables (via viper):

| Variable | Description |
|----------|-------------|
| `KUSARI_CONSOLE_URL` | Override console URL |
| `KUSARI_PLATFORM_URL` | Override platform API URL |
| `KUSARI_VERBOSE` | Enable verbose output (`true`/`false`) |
