# MCP Tool Contracts

**Date**: 2026-03-03
**Feature**: 001-mcp-server

This document defines the MCP tool schemas exposed by the Kusari Inspector MCP server.

## Tool: scan_local_changes

Scan uncommitted changes in a git repository for security vulnerabilities, secrets, and SAST issues.

### Input Schema

```json
{
  "type": "object",
  "properties": {
    "repo_path": {
      "type": "string",
      "description": "Path to the git repository to scan. Defaults to current directory."
    },
    "base_ref": {
      "type": "string",
      "description": "Base git reference for diff (e.g., 'HEAD', 'main', 'origin/main'). Defaults to 'HEAD'.",
      "default": "HEAD"
    },
    "output_format": {
      "type": "string",
      "enum": ["markdown", "sarif"],
      "description": "Output format - 'markdown' for human-readable text or 'sarif' for JSON format. Defaults to 'sarif'.",
      "default": "sarif"
    }
  },
  "required": []
}
```

### Output

Text content containing:
- Console URL for detailed results
- Scan findings (vulnerabilities, secrets, SAST issues)
- Error message if scan failed

### Example Request

```json
{
  "repo_path": "/path/to/repo",
  "base_ref": "HEAD",
  "output_format": "markdown"
}
```

### Example Response

```
================================================================================
📊 View Detailed Results in Kusari Console:
   https://console.us.kusari.cloud/workspaces/.../analysis/.../result
================================================================================

## Security Analysis Results

### Vulnerabilities Found: 2
...
```

---

## Tool: scan_full_repo

Perform a comprehensive security audit of the entire repository.

### Input Schema

```json
{
  "type": "object",
  "properties": {
    "repo_path": {
      "type": "string",
      "description": "Path to the git repository to scan. Defaults to current directory."
    }
  },
  "required": []
}
```

### Output

Text content containing:
- Console URL for detailed results
- OpenSSF Scorecard analysis
- Full dependency scan results
- Complete SAST findings
- Error message if scan failed

### Example Request

```json
{
  "repo_path": "/path/to/repo"
}
```

### Example Response

```
================================================================================
📊 View Detailed Results in Kusari Console:
   https://console.us.kusari.cloud/workspaces/.../risk-check/.../result
================================================================================

## Overall Score: 4/5

### Security Score: 4/5
...
```

---

## Tool: check_scan_status

Check the status of a previously submitted scan.

### Input Schema

```json
{
  "type": "object",
  "properties": {
    "scan_id": {
      "type": "string",
      "description": "The scan ID returned from a previous scan operation"
    }
  },
  "required": ["scan_id"]
}
```

### Output

Text content with status information or "not available in CLI mode" message.

### Note

The CLI-based approach waits for scan completion, so async status checking is limited. This tool is maintained for API compatibility.

---

## Tool: get_scan_results

Retrieve detailed results from a completed scan.

### Input Schema

```json
{
  "type": "object",
  "properties": {
    "scan_id": {
      "type": "string",
      "description": "The scan ID to retrieve results for"
    }
  },
  "required": ["scan_id"]
}
```

### Output

Text content with scan results or "not available in CLI mode" message.

### Note

Scan results are returned immediately when scans complete. This tool is maintained for API compatibility.

---

## Error Handling

All tools return errors as text content with descriptive messages:

| Error Type | Example Message |
|------------|-----------------|
| Authentication | "Not authenticated. Please run 'kusari auth login' first." |
| Repository | "No .git directory found. Directory must be root of repo." |
| Network | "Failed to connect to Kusari API. Check your internet connection." |
| Validation | "Invalid output format: xyz (must be 'markdown' or 'sarif')" |
| Queue | "Scan queued. Position: 2. Please wait..." |

---

## Server Metadata

When clients list available tools, the server provides:

```json
{
  "name": "kusari-inspector",
  "version": "<kusari-cli-version>",
  "tools": [
    {"name": "scan_local_changes", "description": "..."},
    {"name": "scan_full_repo", "description": "..."},
    {"name": "check_scan_status", "description": "..."},
    {"name": "get_scan_results", "description": "..."}
  ]
}
```
