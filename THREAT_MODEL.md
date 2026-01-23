# Threat Model - Kusari CLI

**Version:** 1.0
**Last Updated:** 2026-01-23
**Document Owner:** Kusari Security Team

## Executive Summary

This document describes the threat model for the Kusari CLI, a command-line tool for SBOM scanning and security analysis. The CLI handles sensitive operations including authentication, token storage, and repository scanning.

## System Overview

### Components

| Component | Description | Trust Level |
|-----------|-------------|-------------|
| CLI Application | Go-based command-line tool | Trusted |
| Local Token Storage | `~/.kusari/tokens.json` | Sensitive |
| Workspace Config | `~/.kusari/workspace.json` | Low sensitivity |
| Kusari Platform API | Backend scanning service | External trusted |
| Auth Service | OAuth2/OIDC authentication | External trusted |

### Data Flow

```
User -> CLI -> Auth Service (OAuth2/OIDC)
                    |
                    v
              Token Storage (~/.kusari/)
                    |
User -> CLI -> Platform API -> Scan Results
         |
         v
    Git Repository (read-only access)
```

## Asset Inventory

### Entry Points

| Entry Point | Description | Risk Level |
|-------------|-------------|------------|
| CLI Commands | User-initiated commands | Medium |
| Environment Variables | `KUSARI_*` configuration | Low |
| Configuration Files | `.env`, `kusari.yaml` | Low |
| Network Endpoints | Platform/Auth URLs | Medium |

### Sensitive Data

| Asset | Location | Classification |
|-------|----------|----------------|
| OAuth2 Access Token | `~/.kusari/tokens.json` | Confidential |
| OAuth2 Refresh Token | `~/.kusari/tokens.json` | Confidential |
| Client Secret | CLI argument/env var | Confidential |
| Repository Contents | User's filesystem | Varies |

## Threat Analysis (STRIDE)

### Spoofing

| Threat | Description | Mitigation | Severity |
|--------|-------------|------------|----------|
| S1 | Malicious server impersonation | TLS certificate validation for all API calls | High |
| S2 | Token theft via filesystem access | File permissions (0600) on token storage | Medium |
| S3 | Man-in-the-middle on auth flow | OAuth2 PKCE flow, HTTPS-only endpoints | High |

### Tampering

| Threat | Description | Mitigation | Severity |
|--------|-------------|------------|----------|
| T1 | Modified scan results | Server-side validation, signed responses | Medium |
| T2 | Tampered configuration files | Validate config before use | Low |
| T3 | Modified repository contents during upload | Archive integrity checks | Medium |

### Repudiation

| Threat | Description | Mitigation | Severity |
|--------|-------------|------------|----------|
| R1 | Scan actions not attributable | Workspace-based audit trail on server | Low |
| R2 | Unauthorized workspace access | Authentication required for all operations | Medium |

### Information Disclosure

| Threat | Description | Mitigation | Severity |
|--------|-------------|------------|----------|
| I1 | Token leakage in logs | Never log tokens or secrets | High |
| I2 | Repository contents exposed | Upload only to authenticated endpoints | Medium |
| I3 | Client secret in process list | Use environment variables over CLI args | Medium |
| I4 | Sensitive files in upload | Respect `.gitignore`, exclude sensitive patterns | Medium |

### Denial of Service

| Threat | Description | Mitigation | Severity |
|--------|-------------|------------|----------|
| D1 | Large repository upload exhausts resources | Client-side size limits, streaming upload | Low |
| D2 | Rate limiting by platform | Exponential backoff, retry logic | Low |

### Elevation of Privilege

| Threat | Description | Mitigation | Severity |
|--------|-------------|------------|----------|
| E1 | Cross-workspace access | Server-side workspace authorization | High |
| E2 | Token scope escalation | Request minimum required scopes | Medium |

## Security Controls

### Authentication

- OAuth2/OIDC with PKCE for browser flow
- Client credentials flow for CI/CD (headless)
- Token refresh handled automatically
- Tokens stored with restrictive file permissions

### Network Security

- All communications over HTTPS/TLS
- Certificate validation enabled
- Configurable endpoints for different environments

### Data Protection

- Sensitive data excluded from verbose output
- `.gitignore` patterns respected during packaging
- Temporary files cleaned up after operations

## Recommendations

### Immediate Actions

1. Ensure token files have `0600` permissions
2. Use environment variables for client secrets in CI/CD
3. Review `.gitignore` before scanning sensitive repositories

### Future Improvements

1. Consider encrypted token storage
2. Add token expiration warnings
3. Implement audit logging for local operations

## Review Schedule

This threat model should be reviewed:
- Annually
- When adding new authentication methods
- When adding new data flows
- After any security incident

## References

- [STRIDE Threat Modeling](https://docs.microsoft.com/en-us/azure/security/develop/threat-modeling-tool-threats)
- [OWASP Threat Modeling](https://owasp.org/www-community/Threat_Modeling)
- [OAuth 2.0 Security Best Practices](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics)
