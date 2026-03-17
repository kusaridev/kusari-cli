# Data Model: Kusari MCP Server

**Date**: 2026-03-03
**Feature**: 001-mcp-server

## Entities

### 1. MCPServer

The main server instance managing MCP protocol communication.

| Field | Type | Description |
|-------|------|-------------|
| server | `*mcp.Server` | go-sdk server instance |
| config | `*Config` | Server configuration |
| scanQueue | `chan ScanRequest` | Queue for serializing scans |
| verbose | `bool` | Enable verbose logging |

**Lifecycle**:
- Created when `kusari mcp serve` starts
- Runs until client disconnects or process terminated
- Spawned by MCP client (not user-initiated)

### 2. Config

Server configuration loaded from environment and config file.

| Field | Type | Description |
|-------|------|-------------|
| ConsoleURL | `string` | Kusari console URL (default: `https://console.us.kusari.cloud/`) |
| PlatformURL | `string` | Kusari platform API URL (default: `https://platform.api.us.kusari.cloud/`) |
| AuthEndpoint | `string` | Kusari auth endpoint (default: `https://auth.us.kusari.cloud/`) |
| Verbose | `bool` | Enable verbose logging |

**Sources** (priority order):
1. Environment variables (`KUSARI_CONSOLE_URL`, etc.)
2. Config file (`~/.kusari/mcp-config.yaml`)
3. Default values

### 3. ClientConfig

Configuration for a supported MCP client.

| Field | Type | Description |
|-------|------|-------------|
| Name | `string` | Human-readable name (e.g., "Claude Code") |
| ID | `string` | CLI identifier (e.g., "claude") |
| ConfigPaths | `map[string]string` | Platform → config file path mapping |
| SupportsCliInstall | `bool` | Whether client has CLI for installation |
| CliCommand | `string` | CLI command name (e.g., "claude") |
| ServerKey | `string` | Key in mcpServers object (e.g., "kusari-inspector") |
| ConfigFormat | `ConfigFormat` | Standard or Continue format |

**Supported Clients**:
- `claude` - Claude Code
- `cursor` - Cursor IDE
- `windsurf` - Windsurf IDE
- `cline` - Cline VS Code extension
- `continue` - Continue VS Code extension

### 4. ScanRequest

Request queued for scan execution.

| Field | Type | Description |
|-------|------|-------------|
| ID | `string` | Unique request identifier |
| Type | `ScanType` | `diff` or `full` |
| RepoPath | `string` | Repository path to scan |
| BaseRef | `string` | Base git reference (for diff scans) |
| OutputFormat | `string` | `markdown` or `sarif` |
| ResultChan | `chan ScanResult` | Channel for returning results |

### 5. ScanResult

Result returned from a scan operation.

| Field | Type | Description |
|-------|------|-------------|
| Success | `bool` | Whether scan completed successfully |
| ConsoleURL | `string` | URL to view results in Kusari console |
| Results | `string` | Formatted scan results (markdown or SARIF) |
| Error | `string` | Error message if scan failed |
| QueuePosition | `int` | Position in queue (0 if processing) |

### 6. InstallationResult

Result of client installation/uninstallation.

| Field | Type | Description |
|-------|------|-------------|
| Success | `bool` | Whether operation succeeded |
| ClientName | `string` | Client that was configured |
| ConfigPath | `string` | Path to config file modified |
| Message | `string` | Success or error message |
| NeedsRestart | `bool` | Whether client needs restart |

## Enumerations

### ScanType

```go
type ScanType string

const (
    ScanTypeDiff ScanType = "diff"
    ScanTypeFull ScanType = "full"
)
```

### ConfigFormat

```go
type ConfigFormat int

const (
    ConfigFormatStandard ConfigFormat = iota  // mcpServers object
    ConfigFormatContinue                       // experimental.modelContextProtocolServers array
)
```

### Platform

```go
type Platform string

const (
    PlatformDarwin  Platform = "darwin"
    PlatformLinux   Platform = "linux"
    PlatformWindows Platform = "windows"
)
```

## Relationships

```
MCPServer 1 ──── 1 Config
    │
    └── processes ── * ScanRequest ──── 1 ScanResult

ClientConfig * ──── defines ── InstallationResult
```

## State Transitions

### ScanRequest Lifecycle

```
Created → Queued → Processing → Completed/Failed
   │         │
   │         └── Returns QueuePosition to caller
   └── Immediate if queue empty
```

### Server Lifecycle

```
Started → Running → Shutdown
   │         │
   │         └── Processing tool calls
   └── Loads config, initializes queue
```

## Validation Rules

1. **RepoPath**: Must be a valid directory containing `.git`
2. **BaseRef**: Must be a valid git reference (for diff scans)
3. **OutputFormat**: Must be `markdown` or `sarif`
4. **ClientID**: Must be one of supported client identifiers
5. **Config URLs**: Must be valid HTTPS URLs
