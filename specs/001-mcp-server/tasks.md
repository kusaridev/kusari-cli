# Tasks: Kusari MCP Server

**Input**: Design documents from `/specs/001-mcp-server/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Included (TDD is REQUIRED per project constitution)

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, etc.)
- Include exact file paths in descriptions

## Path Conventions

- **Commands**: `kusari/cmd/`
- **Packages**: `pkg/`
- **Tests**: Co-located with source (e.g., `pkg/mcp/server_test.go`)

---

## Phase 1: Setup (Shared Infrastructure) ✅

**Purpose**: Project initialization and go-sdk dependency

- [X] T001 Add go-sdk dependency: `go get github.com/modelcontextprotocol/go-sdk@latest`
- [X] T002 [P] Create pkg/mcp/ directory structure
- [X] T003 [P] Create pkg/mcpinstall/ directory structure
- [X] T004 Add `kusari mcp` parent command to kusari/cmd/root.go (register MCP() in Execute())

---

## Phase 2: Foundational (Blocking Prerequisites) ✅

**Purpose**: Core types and configuration that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational

- [X] T005 [P] Write config loading tests in pkg/mcp/config_test.go (test env vars, defaults, config file)
- [X] T006 [P] Write ClientConfig tests in pkg/mcpinstall/clients_test.go (test all 5 clients defined)
- [X] T007 [P] Write platform detection tests in pkg/mcpinstall/platforms_test.go (test darwin, linux, windows paths)

### Implementation for Foundational

- [X] T008 Implement Config struct and loading in pkg/mcp/config.go (ConsoleURL, PlatformURL, Verbose)
- [X] T009 [P] Implement ClientConfig struct in pkg/mcpinstall/clients.go (Name, ID, ConfigPaths, ConfigFormat)
- [X] T010 [P] Implement platform detection in pkg/mcpinstall/platforms.go (getPlatform(), getConfigPath())
- [X] T011 Implement ScanRequest and ScanResult types in pkg/mcp/types.go
- [X] T012 Create `kusari mcp` parent command in kusari/cmd/mcp.go (cobra command with help text)

**Checkpoint**: Foundation ready - core types defined, user story implementation can begin

---

## Phase 3: User Story 1 - Install MCP Server for Claude Code (Priority: P1) 🎯 MVP ✅

**Goal**: Enable `kusari mcp install claude` (or interactive selection) to configure Claude Code's MCP settings

**Independent Test**: Run `kusari mcp install claude`, verify config file created, reload VS Code, confirm kusari-inspector appears in MCP list

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T013 [P] [US1] Write installer tests in pkg/mcpinstall/installer_test.go (test Install(), config file creation, update existing)
- [X] T014 [P] [US1] Write MCP server initialization tests in pkg/mcp/server_test.go (test NewServer(), tool registration)

### Implementation for User Story 1

- [X] T015 [US1] Implement Install() function in pkg/mcpinstall/installer.go (create/update config for single client)
- [X] T016 [US1] Implement InstallationResult struct in pkg/mcpinstall/installer.go
- [X] T017 [US1] Implement MCP server skeleton in pkg/mcp/server.go (NewServer(), Run() with stdio transport)
- [X] T018 [US1] Create `kusari mcp serve` command in kusari/cmd/mcp_serve.go (calls server.Run())
- [X] T019 [US1] Create `kusari mcp install` command in kusari/cmd/mcp_install.go (calls installer.Install())
- [X] T019a [US1] Implement interactive client selection UI in kusari/cmd/mcp_install.go (arrow-key navigation when no client arg provided)
- [X] T020 [US1] Add Claude Code client definition in pkg/mcpinstall/clients.go (claude config paths for all platforms)
- [X] T021 [US1] Implement config file JSON read/write in pkg/mcpinstall/installer.go (handle mcpServers format)
- [X] T022 [US1] Add installation success message and next steps output in kusari/cmd/mcp_install.go

**Checkpoint**: User Story 1 complete - `kusari mcp install claude` works, server spawns when Claude needs it

---

## Phase 4: User Story 2 - Scan Local Changes for Security Issues (Priority: P1) 🎯 MVP ✅

**Goal**: Enable AI assistants to scan uncommitted changes via MCP tool

**Independent Test**: Make changes in a git repo, ask Claude "scan my local changes", verify scan results returned

### Tests for User Story 2

- [X] T023 [P] [US2] Write scan_local_changes tool tests in pkg/mcp/tools_test.go (test input validation, repo.Scan integration)
- [~] T024 [P] [US2] Write scan queue tests in pkg/mcp/server_test.go (SKIPPED - using synchronous execution via repo.Scan)

### Implementation for User Story 2

- [X] T025 [US2] Implement scan_local_changes tool handler in pkg/mcp/tools.go (calls pkg/repo.Scan() directly)
- [X] T026 [US2] Register scan_local_changes tool with MCP server in pkg/mcp/server.go
- [~] T027 [US2] Implement scan queue with channel in pkg/mcp/server.go (SKIPPED - synchronous execution sufficient for MVP)
- [~] T028 [US2] Add authentication check in tool handler in pkg/mcp/tools.go (SKIPPED - auth handled internally by pkg/repo.Scan)
- [X] T029 [US2] Implement tool input validation in pkg/mcp/tools.go (validate repo_path, base_ref, output_format)
- [~] T030 [US2] Add queue position notification in pkg/mcp/tools.go (SKIPPED - not needed for synchronous model)

**Checkpoint**: User Story 2 complete - scan_local_changes tool works end-to-end

---

## Phase 5: User Story 3 - Perform Full Repository Security Audit (Priority: P2) ⏸️ DEFERRED

**Goal**: Enable comprehensive repository security audits via MCP tool

**Status**: DEFERRED - RiskCheck functionality is not currently available. scan_full_repo registered as stub.

### Tests for User Story 3

- [~] T031 [P] [US3] Write scan_full_repo tool tests (SKIPPED - RiskCheck not available)

### Implementation for User Story 3

- [~] T032 [US3] scan_full_repo handler - STUB implemented (returns "not currently available")
- [X] T033 [US3] Register scan_full_repo tool with MCP server in pkg/mcp/server.go (registered as stub)
- [X] T034 [US3] Implement check_scan_status stub tool in pkg/mcp/server.go (returns "not available in CLI mode")
- [X] T035 [US3] Implement get_scan_results stub tool in pkg/mcp/server.go (returns "not available in CLI mode")

**Checkpoint**: User Story 3 deferred - stub tools registered, will be implemented when RiskCheck is available

---

## Phase 6: User Story 4 - Install for Multiple Coding Agents (Priority: P2) ✅

**Goal**: Support installation for Cursor, Windsurf, Cline, Continue; add list and uninstall commands

**Independent Test**: Run `kusari mcp install cursor`, `kusari mcp list`, `kusari mcp uninstall cursor`

### Tests for User Story 4

- [X] T036 [P] [US4] Write Uninstall() tests in pkg/mcpinstall/installer_test.go
- [X] T037 [P] [US4] Write ListClients() tests in pkg/mcpinstall/installer_test.go
- [X] T038 [P] [US4] Write Continue config format tests in pkg/mcpinstall/installer_test.go (test experimental.modelContextProtocolServers)

### Implementation for User Story 4

- [X] T039 [P] [US4] Add Cursor client definition in pkg/mcpinstall/clients.go
- [X] T040 [P] [US4] Add Windsurf client definition in pkg/mcpinstall/clients.go
- [X] T041 [P] [US4] Add Cline client definition in pkg/mcpinstall/clients.go
- [X] T042 [P] [US4] Add Continue client definition in pkg/mcpinstall/clients.go (ConfigFormatContinue)
- [X] T043 [US4] Implement Uninstall() function in pkg/mcpinstall/installer.go
- [X] T044 [US4] Implement ListClients() function in pkg/mcpinstall/installer.go (return installation status for all clients)
- [X] T045 [US4] Create `kusari mcp uninstall` command in kusari/cmd/mcp_uninstall.go
- [X] T046 [US4] Create `kusari mcp list` command in kusari/cmd/mcp_list.go
- [X] T047 [US4] Implement Continue config format handling in pkg/mcpinstall/installer.go (different JSON structure)

**Checkpoint**: User Story 4 complete - all 5 clients supported, list/uninstall work

---

## Phase 7: User Story 5 - View Detailed Results in Web Console (Priority: P3) ✅

**Goal**: Include console URL in all scan results for deeper analysis

**Independent Test**: Run a scan, verify results include clickable URL to Kusari Console

### Tests for User Story 5

- [X] T048 [P] [US5] Write console URL extraction tests in pkg/mcp/tools_test.go (TestExtractConsoleURL_FromStderr, TestFormatResultWithConsoleURL)

### Implementation for User Story 5

- [X] T049 [US5] Enhance scan result formatting in pkg/mcp/tools.go (formatResultWithConsoleURL adds console URL banner at top of results)
- [X] T050 [US5] Extract URL from pkg/repo.Scan stderr output in pkg/mcp/tools.go (extractConsoleURL, captureOutput)

**Checkpoint**: User Story 5 complete - all scan results include console URL

---

## Phase 8: Polish & Cross-Cutting Concerns ✅

**Purpose**: Code quality, edge cases, and documentation

- [~] T051 [P] Add verbose logging throughout pkg/mcp/server.go (DEFERRED - basic verbose logging exists)
- [~] T052 [P] Add error handling for offline/network failures in pkg/mcp/tools.go (DEFERRED - handled by pkg/repo)
- [X] T053 [P] Add error handling for expired tokens in pkg/mcp/tools.go (auto-auth with browser login + retry in auth.go)
- [~] T054 [P] Add error handling for permission denied in pkg/mcpinstall/installer.go (DEFERRED - basic errors returned)
- [X] T055 Run go fmt on all new files
- [X] T056 Run go vet on all new files
- [X] T057 Run golangci-lint on all new files (0 issues)
- [~] T058 Validate quickstart.md instructions work end-to-end (DEFERRED - manual testing)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - start immediately
- **Foundational (Phase 2)**: Depends on Setup - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational - MVP target
- **User Story 2 (Phase 4)**: Depends on Foundational + US1 (server must exist)
- **User Story 3 (Phase 5)**: Depends on US2 (extends tools)
- **User Story 4 (Phase 6)**: Depends on US1 (extends installer)
- **User Story 5 (Phase 7)**: Depends on US2 (enhances results)
- **Polish (Phase 8)**: Depends on all user stories

### User Story Dependencies

```
        ┌─────────────────────────────────────┐
        │         Setup (Phase 1)             │
        └─────────────────────────────────────┘
                        │
                        ▼
        ┌─────────────────────────────────────┐
        │      Foundational (Phase 2)         │
        │    ⚠️ BLOCKS ALL USER STORIES       │
        └─────────────────────────────────────┘
                        │
            ┌───────────┴───────────┐
            ▼                       ▼
    ┌───────────────┐       ┌───────────────┐
    │  US1 (P1)     │       │  US4 (P2)     │
    │  Install      │◄──────│  Multi-client │
    │  Claude Code  │       │  (extends)    │
    └───────────────┘       └───────────────┘
            │
            ▼
    ┌───────────────┐       ┌───────────────┐
    │  US2 (P1)     │       │  US5 (P3)     │
    │  Scan Local   │◄──────│  Console URL  │
    │  Changes      │       │  (enhances)   │
    └───────────────┘       └───────────────┘
            │
            ▼
    ┌───────────────┐
    │  US3 (P2)     │
    │  Full Audit   │
    │  (extends)    │
    └───────────────┘
```

### Parallel Opportunities

**Within Foundational (Phase 2)**:
```
Parallel: T005, T006, T007 (all tests)
Parallel: T009, T010 (different packages)
```

**Within User Story 1**:
```
Parallel: T013, T014 (tests in different files)
```

**Within User Story 4**:
```
Parallel: T036, T037, T038 (all tests)
Parallel: T039, T040, T041, T042 (client definitions in same file but independent)
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1 (Install for Claude)
4. Complete Phase 4: User Story 2 (Scan Local Changes)
5. **STOP and VALIDATE**: Test with Claude Code - install + scan should work
6. Deploy/demo if ready

### Incremental Delivery

1. **Setup + Foundational** → Core infrastructure ready
2. **Add US1** → Can install for Claude Code (partial MVP)
3. **Add US2** → Can scan local changes (full MVP!)
4. **Add US3** → Can run full audits
5. **Add US4** → Supports all 5 clients
6. **Add US5** → Enhanced results with console links
7. **Polish** → Production ready

---

## Task Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| Setup | T001-T004 | Project structure, dependencies |
| Foundational | T005-T012 | Core types, config, tests |
| US1 (P1) | T013-T022 + T019a | Install for Claude Code (with interactive UI) |
| US2 (P1) | T023-T030 | Scan local changes tool |
| US3 (P2) | T031-T035 | Full repo audit tool |
| US4 (P2) | T036-T047 | Multi-client support |
| US5 (P3) | T048-T050 | Console URL in results |
| Polish | T051-T058 | Quality, edge cases |

**Total Tasks**: 59
**MVP Tasks (through US2)**: 31

---

## Notes

- All tests must be written and FAIL before implementation (TDD required per constitution)
- Commit after each task or logical group
- Tests co-located with source files (`foo_test.go` alongside `foo.go`)
- MCP server reuses existing `pkg/repo.Scan()` and `pkg/auth` - no duplication
- Stop at any user story checkpoint to validate independently
