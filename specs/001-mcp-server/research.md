# Research: Kusari MCP Server

**Date**: 2026-03-03
**Feature**: 001-mcp-server

## 1. MCP Go SDK Usage

### Decision
Use `github.com/modelcontextprotocol/go-sdk` for MCP server implementation.

### Rationale
- Official Go SDK maintained by the Model Context Protocol team in collaboration with Google
- Provides clean abstractions for server creation, tool registration, and transport handling
- Supports stdio transport required for AI coding assistant integration

### Key Patterns

**Server Creation**:
```go
server := mcp.NewServer(
    &mcp.Implementation{Name: "kusari-inspector", Version: version},
    nil,
)
```

**Tool Registration**:
```go
mcp.AddTool(server,
    &mcp.Tool{
        Name: "scan_local_changes",
        Description: "Scan uncommitted changes for security issues",
    },
    handleScanLocalChanges,
)
```

**Server Execution**:
```go
if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
    return err
}
```

### Alternatives Considered
- Custom JSON-RPC implementation: Rejected - unnecessary complexity
- Python MCP SDK (like reference): Rejected - Go aligns with existing CLI

## 2. Client Configuration Paths

### Decision
Use platform-specific paths for each supported MCP client, matching the reference implementation.

### Client Configuration Map

| Client | macOS | Linux | Windows |
|--------|-------|-------|---------|
| Claude Code | `~/Library/Application Support/Claude/claude_desktop_config.json` | `~/.config/claude/claude_desktop_config.json` | `%APPDATA%/Claude/claude_desktop_config.json` |
| Cursor | `~/.cursor/mcp.json` | `~/.cursor/mcp.json` | `~/.cursor/mcp.json` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | `~/.codeium/windsurf/mcp_config.json` | `~/.codeium/windsurf/mcp_config.json` |
| Cline | `~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` | `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` | `%APPDATA%/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` |
| Continue | `~/.continue/config.json` | `~/.continue/config.json` | `~/.continue/config.json` |

### Configuration Format

**Standard MCP format** (Claude, Cursor, Windsurf, Cline):
```json
{
  "mcpServers": {
    "kusari-inspector": {
      "command": "/path/to/kusari",
      "args": ["mcp", "serve"],
      "env": {
        "KUSARI_CONSOLE_URL": "https://console.us.kusari.cloud/",
        "KUSARI_PLATFORM_URL": "https://platform.api.us.kusari.cloud/"
      }
    }
  }
}
```

**Continue format** (different structure):
```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "name": "kusari-inspector",
        "transport": {
          "type": "stdio",
          "command": "/path/to/kusari",
          "args": ["mcp", "serve"]
        }
      }
    ]
  }
}
```

### Rationale
- Paths match documented locations for each client
- Configuration formats verified against reference Python implementation
- Continue uses different schema requiring special handling

## 3. Integration with Existing CLI

### Decision
Reuse existing `pkg/repo.Scan()` and `pkg/auth` packages for scan operations and authentication.

### Rationale
- Single source of truth for scan logic
- Consistent authentication behavior across CLI and MCP
- Reduces maintenance burden and potential for divergence

### Integration Points

| MCP Tool | Existing Function | Package |
|----------|-------------------|---------|
| `scan_local_changes` | `repo.Scan(dir, rev, ...)` | `pkg/repo` |
| `scan_full_repo` | `repo.RiskCheck(dir, ...)` | `pkg/repo` |
| Authentication | `auth.LoadToken()`, `auth.CheckTokenExpiry()` | `pkg/auth` |
| Workspace | `login.FetchWorkspaces()` | `pkg/login` |

### Modifications Required
- `pkg/repo.Scan()`: May need to return structured results instead of printing
- Consider adding a `ScanResult` struct for MCP tool responses

## 4. MCP Tool Schema Design

### Decision
Expose 4 tools matching the reference implementation capabilities.

### Tools

1. **scan_local_changes**
   - Input: `repo_path` (optional), `base_ref` (default: HEAD), `output_format` (markdown/sarif)
   - Output: Scan results with console URL

2. **scan_full_repo**
   - Input: `repo_path` (optional)
   - Output: Full audit results with console URL

3. **check_scan_status**
   - Input: `scan_id`
   - Output: Status information (note: may return "not available in CLI mode" like reference)

4. **get_scan_results**
   - Input: `scan_id`
   - Output: Results or "not available" message

### Rationale
- Matches reference implementation API surface
- `check_scan_status` and `get_scan_results` kept for API compatibility even if limited

## 5. Verbose Logging

### Decision
Use `--verbose` flag to enable detailed logging to stderr.

### Rationale
- Matches existing CLI pattern (`-v` / `--verbose` flag)
- Stderr used for logs to avoid polluting MCP protocol on stdout
- Environment variable `KUSARI_VERBOSE=true` also supported via viper

### Implementation
```go
if verbose {
    fmt.Fprintf(os.Stderr, "[kusari-mcp] %s\n", message)
}
```

## 6. Scan Queue Behavior

### Decision
Queue concurrent scan requests and notify user of position.

### Implementation Approach
- Use Go channel or mutex to serialize scan operations
- Return immediate acknowledgment with queue position
- Process scans sequentially

### Rationale
- Prevents resource contention on large scans
- Provides clear user feedback
- Simpler than parallel execution with result multiplexing

## Summary

All technical unknowns resolved. Ready to proceed to Phase 1 design.

| Area | Decision |
|------|----------|
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` |
| Client configs | Platform-specific JSON paths per client |
| Scan integration | Reuse `pkg/repo.Scan()` and `pkg/auth` |
| Tools | 4 tools matching reference implementation |
| Logging | `--verbose` flag, stderr output |
| Concurrency | Queue with position notification |
