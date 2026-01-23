# Threat Model

This document describes the threat model for the Kusari CLI, identifying potential security threats and the mitigations in place.

## Overview

The Kusari CLI is a command-line tool that authenticates users and uploads repository data to the Kusari platform for security analysis. This threat model uses the STRIDE methodology to identify and address potential security concerns.

## Assets

### High-Value Assets
1. **Authentication Tokens**: OAuth/OIDC tokens stored locally
2. **Repository Data**: Source code and metadata uploaded for scanning
3. **User Credentials**: Authentication flow credentials
4. **API Keys**: Client secrets for CI/CD authentication

### Medium-Value Assets
1. **Configuration Files**: User preferences and settings
2. **Workspace Information**: Tenant and workspace identifiers
3. **Scan Results**: Links to security analysis results

## Trust Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│                     User's Machine                          │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                    Kusari CLI                        │   │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────────┐   │   │
│  │  │   Auth    │  │   Repo    │  │  Config Store │   │   │
│  │  │  Module   │  │  Scanner  │  │   (~/.kusari) │   │   │
│  │  └─────┬─────┘  └─────┬─────┘  └───────────────┘   │   │
│  └────────┼──────────────┼────────────────────────────┘   │
│           │              │                                  │
└───────────┼──────────────┼──────────────────────────────────┘
            │              │
    ════════╪══════════════╪════════════════════════════════════
            │              │         Network Boundary
            ▼              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Kusari Platform                            │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐   │
│  │   Auth/OIDC   │  │  Scan API     │  │   Console     │   │
│  │   Provider    │  │               │  │               │   │
│  └───────────────┘  └───────────────┘  └───────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## STRIDE Analysis

### Spoofing

| Threat | Description | Mitigation |
|--------|-------------|------------|
| S1 | Attacker impersonates Kusari auth server | TLS certificate validation; hardcoded trusted endpoints |
| S2 | Attacker impersonates user via stolen tokens | Tokens stored with restricted file permissions (0600) |
| S3 | Malicious OAuth callback | State parameter validation; localhost-only callback server |

### Tampering

| Threat | Description | Mitigation |
|--------|-------------|------------|
| T1 | Man-in-the-middle modifies upload data | HTTPS-only communication; TLS 1.2+ required |
| T2 | Local token file modification | File permission enforcement; token validation on use |
| T3 | Configuration file tampering | Non-sensitive defaults; validation on load |

### Repudiation

| Threat | Description | Mitigation |
|--------|-------------|------------|
| R1 | User denies performing scan | Server-side audit logging on Kusari platform |
| R2 | Denial of upload actions | Platform maintains upload history per workspace |

### Information Disclosure

| Threat | Description | Mitigation |
|--------|-------------|------------|
| I1 | Token leakage via logs | No token logging; redacted output |
| I2 | Repository data exposure in transit | HTTPS-only; encrypted uploads |
| I3 | Local credential theft | File permissions; no plaintext secrets |
| I4 | Environment variable exposure | Sensitive values not logged |

### Denial of Service

| Threat | Description | Mitigation |
|--------|-------------|------------|
| D1 | Resource exhaustion during packaging | Size limits on uploads; timeout handling |
| D2 | Authentication endpoint flooding | Rate limiting on Kusari platform |

### Elevation of Privilege

| Threat | Description | Mitigation |
|--------|-------------|------------|
| E1 | CLI runs with elevated permissions | No setuid/elevated permissions required |
| E2 | Workspace access escalation | Server-side workspace authorization |
| E3 | Token scope escalation | Minimal scope tokens; server-enforced permissions |

## Attack Scenarios

### Scenario 1: Stolen Authentication Token
**Attack**: Attacker gains access to `~/.kusari/tokens.json`
**Impact**: Unauthorized API access until token expiry
**Mitigations**:
- File permissions set to 0600 (owner read/write only)
- Tokens have limited lifetime
- Refresh tokens can be revoked server-side

### Scenario 2: Malicious Dependency
**Attack**: Supply chain attack via compromised dependency
**Impact**: Arbitrary code execution
**Mitigations**:
- Dependabot automated security updates
- Dependency review in CI/CD
- SLSA provenance attestations for releases
- go.sum integrity verification

### Scenario 3: Network Interception
**Attack**: Attacker intercepts network traffic
**Impact**: Data exposure; credential theft
**Mitigations**:
- HTTPS required for all communications
- Certificate validation enforced
- No fallback to HTTP

## Security Controls

### Authentication
- [x] OAuth 2.0 with PKCE
- [x] OIDC token validation
- [x] Secure token storage
- [x] SSO/SAML support

### Transport Security
- [x] TLS 1.2+ required
- [x] Certificate validation
- [x] No HTTP fallback

### Data Protection
- [x] Minimal data collection
- [x] No sensitive data logging
- [x] Encrypted storage where applicable

### Build Security
- [x] Reproducible builds
- [x] SLSA provenance
- [x] Signed releases
- [x] SBOM generation

## Recommendations for Users

1. **Protect token files**: Ensure `~/.kusari/` has appropriate permissions
2. **Use CI/CD secrets**: Store client secrets in secure secret stores
3. **Review before scanning**: Understand what data will be uploaded
4. **Keep CLI updated**: Apply security updates promptly
5. **Use SSO when available**: Leverage enterprise authentication

## Incident Response

If you discover a security vulnerability:
1. **Do not** create a public issue
2. Report via GitHub Security Advisories (private)
3. See [SECURITY.md](../SECURITY.md) for detailed instructions

## Review Schedule

This threat model should be reviewed:
- Annually
- After significant architectural changes
- Following security incidents
- When adding new features that handle sensitive data
