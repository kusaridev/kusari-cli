# Threat Model

This document describes the threat model for Kusari CLI, identifying potential security threats and the mitigations in place.

## Overview

Kusari CLI is a command-line tool that authenticates users and uploads repository data to the Kusari platform for security analysis. Understanding the threat landscape helps ensure appropriate security controls.

## Assets

### Critical Assets
- **OAuth2 Tokens**: Stored in `~/.kusari/tokens.json`
- **User Credentials**: Handled during OAuth flow (never stored directly)
- **Repository Data**: Temporarily packaged for upload during scans

### Important Assets
- **Workspace Configuration**: Stored in `~/.kusari/workspace.json`
- **CLI Binary**: Distributed via GitHub releases

## Trust Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│                     User's Machine                          │
│  ┌─────────────┐    ┌──────────────────────────────────┐   │
│  │   User      │    │         Kusari CLI               │   │
│  │             │───>│  ┌────────────┐ ┌─────────────┐  │   │
│  └─────────────┘    │  │ Token Store│ │ Config Store│  │   │
│                     │  └────────────┘ └─────────────┘  │   │
│                     └──────────────────────────────────┘   │
└───────────────────────────────┬─────────────────────────────┘
                                │ HTTPS (Trust Boundary)
                                ▼
┌───────────────────────────────────────────────────────────────┐
│                    External Services                          │
│  ┌─────────────────┐         ┌─────────────────────────────┐ │
│  │ Auth Provider   │         │    Kusari Platform API      │ │
│  │ (auth.kusari)   │         │    (kusari.api.us.kusari)   │ │
│  └─────────────────┘         └─────────────────────────────┘ │
└───────────────────────────────────────────────────────────────┘
```

## Threats and Mitigations

### STRIDE Analysis

#### Spoofing

| Threat | Risk | Mitigation |
|--------|------|------------|
| Attacker impersonates auth server | High | TLS certificate validation, pinned endpoints |
| Malicious CLI binary | High | Signed releases, SLSA provenance, checksums |
| Token theft via phishing | Medium | User education, short-lived tokens |

#### Tampering

| Threat | Risk | Mitigation |
|--------|------|------------|
| Token file modification | Medium | File permissions (0600), integrity checks |
| Man-in-the-middle attacks | High | TLS for all communications |
| Binary tampering | High | Code signing, SLSA provenance verification |

#### Repudiation

| Threat | Risk | Mitigation |
|--------|------|------------|
| Unauthorized scans | Low | Workspace-scoped access, audit logs on platform |
| Denial of actions | Low | Platform-side audit logging |

#### Information Disclosure

| Threat | Risk | Mitigation |
|--------|------|------------|
| Token leakage via logs | Medium | Never log sensitive data, redaction |
| Repository data exposure | Medium | TLS in transit, platform access controls |
| Credentials in error messages | Medium | Sanitized error messages |

#### Denial of Service

| Threat | Risk | Mitigation |
|--------|------|------------|
| API rate limiting attacks | Low | Server-side rate limiting |
| Large repository uploads | Low | Size limits, timeout handling |

#### Elevation of Privilege

| Threat | Risk | Mitigation |
|--------|------|------------|
| Token scope escalation | Medium | Minimal OAuth scopes requested |
| Workspace access escalation | Medium | Workspace-scoped tokens, platform RBAC |

## Security Controls

### Authentication
- OAuth2/OIDC with PKCE flow
- Short-lived access tokens with refresh capability
- SSO support for enterprise authentication

### Token Storage
- Tokens stored with restrictive file permissions (0600)
- Stored in user's home directory only
- No tokens in environment variables or command history

### Communication Security
- All API calls over HTTPS
- TLS 1.2+ required
- Certificate validation enabled

### Supply Chain Security
- SLSA Level 3 provenance for releases
- SBOMs published with releases
- Signed container images (cosign)
- Dependency scanning via Dependabot

### Build Security
- Reproducible builds via GoReleaser
- Minimal build dependencies
- CI/CD with pinned action versions

## Incident Response

Security issues should be reported per [SECURITY.md](../SECURITY.md).

## Review Schedule

This threat model should be reviewed:
- Annually at minimum
- When significant features are added
- After security incidents
- When the threat landscape changes
