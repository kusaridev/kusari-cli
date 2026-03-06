# Implementation Plan: Kusari MCP Server

**Branch**: `001-mcp-server` | **Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-mcp-server/spec.md`

## Summary

Implement an MCP (Model Context Protocol) server as a subcommand of the existing Kusari CLI that enables AI coding assistants (Claude Code, Cursor, Windsurf, Cline, Continue) to perform security scans. The server uses the official go-sdk, communicates via stdio transport, and directly calls internal packages (`pkg/repo`, `pkg/auth`) for scan operations.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**:
- `github.com/modelcontextprotocol/go-sdk` v1.4.0 (MCP protocol)
- `github.com/spf13/cobra` v1.10.2 (CLI framework)
- `github.com/spf13/viper` v1.21.0 (configuration)
- `github.com/charmbracelet/huh` (interactive UI)
- `github.com/stretchr/testify` v1.11.1 (testing)

**Storage**: File-based JSON configs (`~/.kusari/tokens.json`, client MCP configs)
**Testing**: `go test` with testify assertions
**Target Platform**: macOS, Linux, Windows
**Project Type**: CLI tool (subcommand extension)
**Performance Goals**: Scan results within 60 seconds for typical repos
**Constraints**: Stdio transport, single binary, client-spawned subprocess
**Scale/Scope**: 5 MCP clients, 4 tools, 3 platforms

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Security-First | ✅ PASS | Tool is security scanner; tokens stored in `~/.kusari/tokens.json`; no credentials logged |
| II. Test-Driven Development | ✅ PASS | TDD enforced - tests written before implementation in all phases |
| III. Unit Test Coverage | ✅ PASS | All packages have `*_test.go` files; exported functions tested |
| IV. CLI Interface Standards | ✅ PASS | Using cobra; stdout/stderr separation; --verbose flag; exit codes |
| V. Code Quality | ✅ PASS | go fmt, go vet, golangci-lint in polish phase |
| VI. Simplicity (YAGNI) | ✅ PASS | Reusing existing packages; minimal new abstractions; no speculative features |

**Gate Status**: ✅ All principles pass - proceed to Phase 0

### Post-Design Re-Check (Phase 1 Complete)

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Security-First | ✅ PASS | Internal packages used directly; no subprocess invocation; tokens via existing pkg/auth |
| II. Test-Driven Development | ✅ PASS | All packages have tests written first (server_test.go, installer_test.go, etc.) |
| III. Unit Test Coverage | ✅ PASS | config_test.go, clients_test.go, platforms_test.go, server_test.go, installer_test.go |
| IV. CLI Interface Standards | ✅ PASS | cobra commands; stdout/stderr; --verbose flag; proper exit codes |
| V. Code Quality | ✅ PASS | go fmt/vet/lint in Phase 8 Polish |
| VI. Simplicity (YAGNI) | ✅ PASS | Direct pkg/repo calls instead of subprocess; reusing existing patterns |

**Post-Design Gate Status**: ✅ All principles pass - ready for Phase 2 task generation

## Project Structure

### Documentation (this feature)

```text
specs/001-mcp-server/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── cli-commands.md  # CLI command schemas
│   └── mcp-tools.md     # MCP tool definitions
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
kusari/
└── cmd/
    ├── mcp.go           # Parent command
    ├── mcp_serve.go     # kusari mcp serve
    ├── mcp_install.go   # kusari mcp install (with interactive UI)
    ├── mcp_uninstall.go # kusari mcp uninstall
    └── mcp_list.go      # kusari mcp list

pkg/
├── mcp/                 # MCP server package
│   ├── server.go        # MCP server implementation
│   ├── server_test.go   # Server tests
│   ├── config.go        # MCP config loading
│   ├── config_test.go   # Config tests
│   ├── types.go         # ScanRequest, ScanResult types
│   └── tools.go         # Tool handlers (call pkg/repo)
│
└── mcpinstall/          # Installation package
    ├── installer.go     # Install/Uninstall functions
    ├── installer_test.go
    ├── clients.go       # Client definitions
    ├── clients_test.go
    ├── platforms.go     # Platform detection
    └── platforms_test.go
```

**Structure Decision**: Extension of existing CLI structure. New packages (`pkg/mcp`, `pkg/mcpinstall`) follow existing patterns. Commands in `kusari/cmd/` follow cobra conventions.

## Complexity Tracking

> No violations - structure follows existing patterns and constitution principles.
