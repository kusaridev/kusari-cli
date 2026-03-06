# Quickstart: Kusari MCP Server

**Date**: 2026-03-03
**Feature**: 001-mcp-server

This guide walks through using the Kusari MCP Server with Claude Code.

## Prerequisites

- Kusari CLI installed (`go install github.com/kusaridev/kusari-cli/kusari@latest`)
- Claude Code (VS Code extension) installed
- Kusari account ([sign up at console.kusari.cloud](https://console.kusari.cloud))

## Installation

### Step 1: Install the MCP Server for Claude Code

```bash
kusari mcp install claude
```

Expected output:
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
```

### Step 2: Reload VS Code

1. Open VS Code Command Palette: `Cmd+Shift+P` (macOS) or `Ctrl+Shift+P` (Windows/Linux)
2. Type and select: `Developer: Reload Window`

### Step 3: Verify Installation

Ask Claude: "What MCP tools do you have available?"

Claude should list `kusari-inspector` with tools like `scan_local_changes` and `scan_full_repo`.

## Usage

### Scan Local Changes

Make some code changes in your repository, then ask Claude:

```
Claude, scan my local changes for security issues
```

On first use:
1. Your browser will open to authenticate with Kusari
2. Log in with your Kusari account
3. The scan will proceed automatically after authentication

### Full Repository Audit

For a comprehensive security audit:

```
Claude, run a full security audit on this repository
```

This takes longer but includes:
- OpenSSF Scorecard analysis
- Complete dependency scanning
- Full SAST analysis

## Viewing Results

Each scan returns:
- A summary of findings directly in the chat
- A clickable link to the Kusari Console for detailed results

Example output:
```
================================================================================
📊 View Detailed Results in Kusari Console:
   https://console.us.kusari.cloud/workspaces/.../analysis/.../result
================================================================================

## Security Analysis Results

### Vulnerabilities Found: 2
- CVE-2024-1234: High severity in dependency X
- CVE-2024-5678: Medium severity in dependency Y

### Secrets Detected: 0
No exposed secrets found.

### Code Issues: 3
- SQL injection risk in file.go:42
...
```

## Other MCP Clients

### Install for Cursor

```bash
kusari mcp install cursor
```

Then restart Cursor.

### Install for Windsurf

```bash
kusari mcp install windsurf
```

Then restart Windsurf.

### List All Clients

```bash
kusari mcp list
```

## Troubleshooting

### MCP Server Not Showing Up

1. Verify installation status:
   ```bash
   kusari mcp list
   ```

2. Check the config file was created:
   - macOS: `cat ~/Library/Application\ Support/Claude/claude_desktop_config.json`
   - Linux: `cat ~/.config/claude/claude_desktop_config.json`

3. Reload VS Code window

### Authentication Issues

1. Check if tokens exist:
   ```bash
   ls ~/.kusari/tokens.json
   ```

2. Re-authenticate:
   ```bash
   rm ~/.kusari/tokens.json
   # Then run a scan - it will prompt for auth
   ```

### Scan Errors

If scans fail, check:

1. You're in a git repository root:
   ```bash
   ls -la .git
   ```

2. You have network connectivity to Kusari:
   ```bash
   curl -I https://platform.api.us.kusari.cloud/
   ```

3. Run with verbose mode for details:
   ```bash
   KUSARI_VERBOSE=true kusari mcp serve
   ```

## Uninstalling

To remove the MCP server from a client:

```bash
kusari mcp uninstall claude
```

Then reload VS Code.

## What's Next?

- Set up CI/CD integration: See [Kusari CI Templates](https://github.com/kusaridev/kusari-ci-templates)
- Configure workspace settings: Run `kusari auth login` to select a workspace
- Explore the console: Visit [console.kusari.cloud](https://console.kusari.cloud) for detailed analysis
