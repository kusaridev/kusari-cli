<!--
================================================================================
SYNC IMPACT REPORT
================================================================================
Version change: 0.0.0 → 1.0.0 (initial ratification)
Modified principles: N/A (initial creation)
Added sections:
  - Core Principles (6 principles)
  - Security Requirements
  - Development Workflow
  - Governance
Removed sections: N/A
Templates requiring updates:
  - .specify/templates/plan-template.md: ✅ compatible (Constitution Check section exists)
  - .specify/templates/spec-template.md: ✅ compatible (test-first language aligned)
  - .specify/templates/tasks-template.md: ✅ compatible (TDD workflow aligned)
Follow-up TODOs: None
================================================================================
-->

# Kusari CLI Constitution

## Core Principles

### I. Security-First

All design decisions MUST prioritize security considerations. As a security scanning CLI tool, Kusari CLI MUST embody the security standards it enforces on other projects.

- Security vulnerabilities in CLI code are unacceptable and MUST be addressed before any feature work
- User credentials, tokens, and sensitive data MUST never be logged or exposed in output
- All external inputs (command-line arguments, file contents, API responses) MUST be validated
- Dependencies MUST be regularly audited for known vulnerabilities

**Rationale**: A security tool with security flaws undermines user trust and defeats its purpose.

### II. Test-Driven Development (NON-NEGOTIABLE)

TDD is mandatory for all code changes. The Red-Green-Refactor cycle MUST be strictly enforced.

- Tests MUST be written before implementation code
- Tests MUST fail before implementation begins (Red phase)
- Implementation MUST only satisfy the failing tests (Green phase)
- Refactoring MUST not change behavior (Refactor phase)
- No production code may be written without a corresponding failing test

**Rationale**: TDD ensures code correctness, prevents regression, and produces maintainable code by design.

### III. Unit Test Coverage

All packages MUST have unit test coverage. Code without tests is considered incomplete.

- Every exported function MUST have at least one unit test
- Edge cases and error conditions MUST be tested
- Test files MUST be co-located with their source files (e.g., `foo_test.go` alongside `foo.go`)
- Mock external dependencies to isolate unit under test

**Rationale**: Unit tests provide fast feedback, document expected behavior, and enable safe refactoring.

### IV. CLI Interface Standards

The CLI MUST follow text I/O protocol conventions for composability and debuggability.

- Input via stdin and command-line arguments
- Normal output to stdout, errors to stderr
- Support both JSON and human-readable output formats via `--format` flag
- Exit codes MUST follow conventions: 0 for success, non-zero for failures
- Error messages MUST be actionable and include remediation guidance

**Rationale**: Consistent CLI behavior enables scripting, CI/CD integration, and user trust.

### V. Code Quality

All code MUST pass quality checks before merge. No exceptions.

- Code MUST pass `go fmt` formatting
- Code MUST pass `go vet` static analysis
- Code MUST pass configured linters (golangci-lint)
- Functions SHOULD be kept small and focused (single responsibility)
- Package names MUST be descriptive and follow Go conventions

**Rationale**: Consistent code quality reduces cognitive load and maintenance burden.

### VI. Simplicity (YAGNI)

Start simple and avoid over-engineering. Only build what is needed now.

- Do not add features, abstractions, or configurability for hypothetical future needs
- Prefer straightforward solutions over clever ones
- Avoid premature optimization
- Three similar lines of code is better than a premature abstraction
- Delete unused code rather than commenting it out

**Rationale**: Simplicity reduces bugs, improves maintainability, and speeds development.

## Security Requirements

As a security-focused tool, Kusari CLI MUST meet elevated security standards:

- **Authentication**: OIDC tokens MUST be stored securely using OS keychain where available
- **Data handling**: SBOM and scan data MUST not be cached in plaintext on disk
- **Network**: All API communications MUST use HTTPS; certificate validation MUST NOT be disabled
- **Secrets scanning**: The CLI MUST NOT output detected secrets in plaintext; redaction is required
- **Supply chain**: Dependencies MUST be pinned to specific versions; Dependabot or equivalent MUST be enabled

## Development Workflow

### Pull Request Requirements

- All tests MUST pass in CI before merge
- Code review by at least one maintainer is required
- PR description MUST explain the change and link to relevant issues
- Breaking changes MUST be documented in PR description

### Commit Standards

- Commits SHOULD be atomic and focused on a single change
- Commit messages SHOULD follow conventional commits format
- Force-pushing to shared branches is prohibited

### Release Process

- Releases follow semantic versioning (MAJOR.MINOR.PATCH)
- CHANGELOG MUST be updated for each release
- Release binaries MUST be built via CI (not locally)

## Governance

This Constitution supersedes all other development practices for Kusari CLI.

### Amendment Process

1. Propose amendment via pull request to `.specify/memory/constitution.md`
2. Document rationale for the change
3. Obtain approval from at least two maintainers
4. Update version according to semantic versioning:
   - MAJOR: Principle removal or fundamental redefinition
   - MINOR: New principle added or material expansion
   - PATCH: Clarifications and wording improvements
5. Propagate changes to dependent templates if affected

### Compliance

- All PRs MUST verify compliance with Constitution principles
- Reviewers MUST check for Constitution violations
- Complexity beyond these principles MUST be explicitly justified

**Version**: 1.0.0 | **Ratified**: 2026-03-03 | **Last Amended**: 2026-03-03
