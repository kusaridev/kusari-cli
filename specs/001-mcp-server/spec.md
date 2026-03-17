# Feature Specification: Kusari MCP Server

**Feature Branch**: `001-mcp-server`
**Created**: 2026-03-03
**Status**: Draft
**Input**: User description: "Create MCP server using go-sdk with auto-installation for coding agents, providing inspector capabilities via Kusari CLI"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install MCP Server for Claude Code (Priority: P1)

A developer wants to use Kusari Inspector directly from Claude Code without manual configuration. They run a simple install command that configures Claude Code to automatically spawn the Kusari MCP server when needed. After installation, the server starts automatically when Claude Code needs it - the user never runs the server manually.

**Why this priority**: Installation is the gateway to all other functionality. Without a simple installation experience, users cannot access any security scanning features.

**Independent Test**: Can be fully tested by running the install command and verifying Claude Code recognizes the MCP server, then asking Claude to list available tools.

**Acceptance Scenarios**:

1. **Given** the user has the Kusari CLI installed and Claude Code running, **When** they run `kusari mcp install claude` (or `kusari mcp install` and select "Claude Code" from the interactive menu), **Then** the MCP server configuration is added to Claude Code and the server appears in Claude's MCP server list.

2. **Given** the MCP server is already installed for Claude Code, **When** the user runs the install command again, **Then** the existing configuration is updated (not duplicated) and the user sees a success message.

3. **Given** the user has not authenticated with Kusari, **When** they run the install command, **Then** installation completes and authentication is deferred to first scan.

4. **Given** the MCP server is installed for Claude Code, **When** Claude Code starts or needs to use Kusari tools, **Then** Claude Code automatically spawns the MCP server process without any user action.

---

### User Story 2 - Scan Local Changes for Security Issues (Priority: P1)

A developer working on code changes wants to quickly check their uncommitted changes for security vulnerabilities, exposed secrets, and code issues before committing. They ask Claude to scan their local changes and receive a summary of findings.

**Why this priority**: This is the core value proposition - enabling AI assistants to perform security scans. Tied for P1 with installation since both are required for MVP.

**Independent Test**: Can be tested by making code changes in a repository, asking Claude to scan local changes, and verifying scan results are returned.

**Acceptance Scenarios**:

1. **Given** the user has uncommitted changes in a git repository, **When** they ask Claude "scan my local changes for security issues", **Then** the MCP server triggers a diff-based scan and returns findings including vulnerabilities, secrets, and SAST issues.

2. **Given** the user has not authenticated with Kusari, **When** a scan is requested, **Then** the user's browser opens to the Kusari authentication page, and after successful login, the scan proceeds automatically.

3. **Given** the user has no uncommitted changes, **When** they request a local changes scan, **Then** they receive a clear message indicating there are no changes to scan.

---

### User Story 3 - Perform Full Repository Security Audit (Priority: P2)

A developer wants to perform a comprehensive security audit of their entire repository, including OpenSSF Scorecard analysis, complete dependency scanning, and full SAST analysis across all files.

**Why this priority**: Full scans provide comprehensive security assessment but take longer. Most users will use quick diff scans for daily work, making this secondary.

**Independent Test**: Can be tested by running a full scan on a repository and verifying comprehensive results including scorecard, dependencies, and SAST findings are returned.

**Acceptance Scenarios**:

1. **Given** the user is in a git repository, **When** they ask Claude "run a full security audit on this repository", **Then** the MCP server triggers a comprehensive scan and returns detailed findings.

2. **Given** a full scan is in progress, **When** the user asks for status, **Then** they receive progress information about the ongoing scan.

---

### User Story 4 - Install for Multiple Coding Agents (Priority: P2)

A developer uses multiple AI coding assistants (Cursor, Windsurf, Cline) and wants to install the Kusari MCP server for each of them using a consistent interface.

**Why this priority**: Supporting multiple clients expands the user base significantly, but Claude Code is the primary target for MVP.

**Independent Test**: Can be tested by running the install command for different clients and verifying each client's configuration is updated correctly.

**Acceptance Scenarios**:

1. **Given** the user has Cursor installed, **When** they run `kusari mcp install cursor`, **Then** the MCP server is configured for Cursor's MCP configuration location.

2. **Given** the user runs `kusari mcp list`, **Then** they see all supported MCP clients with their installation status.

3. **Given** the user wants to remove the server from a client, **When** they run `kusari mcp uninstall <client>`, **Then** the configuration is removed from that client.

---

### User Story 5 - View Detailed Results in Web Console (Priority: P3)

After a scan completes, the developer wants to view detailed results with code context, remediation guidance, and historical tracking in the Kusari web console.

**Why this priority**: The core scan functionality returns results inline. Web console access is an enhancement for deeper analysis.

**Independent Test**: Can be tested by completing a scan and verifying the returned results include a clickable link to the web console with the scan details.

**Acceptance Scenarios**:

1. **Given** a scan has completed, **When** results are returned, **Then** the output includes a direct URL to view detailed results in the Kusari Cloud console.

---

### Edge Cases

- What happens when the user runs `kusari mcp serve` directly (for testing/debugging)? The server should start and respond to MCP protocol requests via stdio, allowing developers to test the server or configure clients manually. This is not the normal usage path.
- What happens when authentication tokens expire? The server should detect expired tokens and initiate re-authentication automatically.
- What happens when the user is offline? The server should return a clear error message indicating network connectivity is required.
- What happens when the repository is very large? The server should handle timeouts gracefully and provide progress feedback for long-running scans.
- What happens when the client configuration file doesn't exist? The server should create the necessary directory structure and configuration file.
- What happens when the user doesn't have write permissions to the config location? The server should display a clear error with instructions for manual configuration.
- What happens when a user requests a scan while another is in progress? The server should queue the request and notify the user of their position in the queue.

## Requirements *(mandatory)*

### Functional Requirements

**MCP Server Core**
- **FR-001**: System MUST implement an MCP server using the go-sdk that communicates via stdio transport.
- **FR-002**: System MUST expose tools for: scanning local changes, scanning full repository, checking scan status, and retrieving scan results.
- **FR-003**: System MUST call internal Go packages directly (e.g., `pkg/repo.Scan()`, `pkg/auth`) to perform scan operations, not invoke the kusari binary as a subprocess.
- **FR-004**: System MUST support configuration via environment variables (CONSOLE_URL, API_ENDPOINT, AUTH_ENDPOINT).
- **FR-005**: System MUST support configuration via a YAML config file at `~/.kusari/mcp-config.yaml`.

**Auto-Installation**
- **FR-006**: System MUST provide an `install` subcommand that configures the MCP server for a specified client.
- **FR-006a**: When `kusari mcp install` is run without a client argument, system MUST present an interactive terminal UI with arrow-key navigation allowing the user to select from supported coding agents.
- **FR-007**: System MUST support installation for Claude Code, Cursor, Windsurf, Cline, and Continue clients.
- **FR-008**: System MUST detect the user's operating system and use the appropriate configuration file paths for each platform (macOS, Linux, Windows).
- **FR-009**: System MUST provide an `uninstall` subcommand to remove configuration from a client.
- **FR-010**: System MUST provide a `list` subcommand showing all supported clients and their installation status.

**Authentication**
- **FR-011**: System MUST trigger browser-based OAuth authentication when credentials are not available.
- **FR-012**: System MUST store authentication tokens securely at `~/.kusari/tokens.json`.
- **FR-013**: System MUST automatically refresh expired tokens when possible.

**Architecture**
- **FR-014**: The MCP server MUST be implemented as a subcommand (`kusari mcp`) of the existing Kusari CLI, sharing authentication and configuration infrastructure.
- **FR-015**: The `kusari mcp serve` subcommand MUST be designed to run as a subprocess spawned by the MCP client (coding agent), not as a user-initiated foreground process.
- **FR-016**: The MCP server subcommand MUST directly import and call existing CLI packages (`pkg/repo`, `pkg/auth`, `pkg/configuration`) for authentication, configuration, and scan operations - no subprocess invocation.
- **FR-022**: The `kusari mcp serve` subcommand MUST support a `--verbose` flag to enable detailed logging to stderr for troubleshooting.
- **FR-023**: The `kusari mcp install` command MUST configure the client's MCP settings with the full command path to spawn the server (e.g., `kusari mcp serve`), enabling the client to start the server automatically when needed.

**Scan Tools**
- **FR-017**: The `scan_local_changes` tool MUST accept optional parameters for repository path, base git reference, and output format (markdown or SARIF).
- **FR-018**: The `scan_full_repo` tool MUST accept an optional repository path parameter.
- **FR-019**: Scan results MUST include a URL to view detailed findings in the Kusari Cloud console.
- **FR-020**: System MUST return results in both human-readable and structured formats.
- **FR-021**: System MUST queue scan requests when a scan is already in progress and notify the user of their queue position.

### Key Entities

- **MCP Server**: The main server process that handles MCP protocol communication and tool invocations.
- **Tool**: An MCP tool definition with name, description, and input schema that maps to internal package function calls (e.g., `pkg/repo.Scan()`).
- **Client Configuration**: Platform-specific configuration for each supported AI coding assistant (config paths, CLI commands, server keys).
- **Scan Result**: The output of a security scan including findings, console URL, and status information.
- **Authentication Token**: OAuth credentials stored locally for authenticating with Kusari services.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can install the MCP server and complete their first security scan within 5 minutes of starting.
- **SC-002**: Installation succeeds on first attempt for 95% of users with standard development environments.
- **SC-003**: Local change scans return results within 60 seconds for typical repositories (under 10,000 files).
- **SC-004**: The MCP server successfully installs for all supported clients (Claude, Cursor, Windsurf, Cline, Continue) on all supported platforms (macOS, Linux, Windows).
- **SC-005**: Users can perform scans without needing to leave their IDE or coding assistant interface.
- **SC-006**: 90% of users successfully complete authentication on first attempt when prompted.

## Clarifications

### Session 2026-03-03

- Q: What is the binary distribution strategy for the MCP server? → A: Subcommand of existing Kusari CLI (single binary)
- Q: How should concurrent scan requests be handled? → A: Queue requests and notify user of position
- Q: What logging/diagnostic capabilities should the MCP server have? → A: Verbose flag (`--verbose`) to enable detailed logging when needed
- Q: How is the MCP server process lifecycle managed? → A: Client-spawned subprocess; `kusari mcp install` configures the client to start the server automatically when needed; users never run `kusari mcp serve` directly
- Q: How should users select the coding agent during install? → A: Interactive terminal UI with arrow-key selection when no client argument provided
- Q: How should MCP tool handlers invoke scan operations? → A: Call internal Go packages directly (e.g., `pkg/repo.Scan()`), not invoke the kusari binary as a subprocess

## Assumptions

- Users have the Kusari CLI installed (via `go install` or pre-built binary from releases).
- Users have internet connectivity for OAuth authentication and scan execution.
- The internal packages (`pkg/repo`, `pkg/auth`) are the authoritative implementation for scan operations, shared by both CLI commands and MCP tools.
- Supported MCP clients follow their documented configuration file locations and formats.
- Users have necessary permissions to write to their home directory configuration locations.
