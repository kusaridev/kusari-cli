# Dependencies

This document describes the dependencies used by Kusari CLI and the rationale for their inclusion.

## Direct Dependencies

| Package | Version | Purpose | License |
|---------|---------|---------|---------|
| `github.com/spf13/cobra` | v1.10.2 | CLI framework and command structure | Apache-2.0 |
| `github.com/spf13/viper` | v1.21.0 | Configuration management | MIT |
| `github.com/spf13/pflag` | v1.0.10 | POSIX/GNU-style flags | BSD-3-Clause |
| `github.com/coreos/go-oidc/v3` | v3.17.0 | OpenID Connect authentication | Apache-2.0 |
| `golang.org/x/oauth2` | v0.34.0 | OAuth2 client implementation | BSD-3-Clause |
| `github.com/charmbracelet/glamour` | v0.10.0 | Markdown rendering in terminal | MIT |
| `github.com/briandowns/spinner` | v1.23.2 | Terminal spinner/progress indicator | Apache-2.0 |
| `github.com/stretchr/testify` | v1.11.1 | Testing assertions and mocks | MIT |
| `golang.org/x/sync` | v0.19.0 | Synchronization primitives | BSD-3-Clause |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML parsing | Apache-2.0 |

## Dependency Selection Criteria

Dependencies are chosen based on:

1. **Security**: Well-maintained with responsive security handling
2. **Licensing**: Compatible with MIT license
3. **Stability**: Mature, stable APIs
4. **Community**: Active maintenance and community support
5. **Minimalism**: Prefer standard library when sufficient

## Dependency Management

- Dependencies are managed via Go modules (`go.mod`, `go.sum`)
- `go.sum` provides cryptographic verification of dependency integrity
- Dependabot is configured for automated security updates
- Dependencies are reviewed during pull request process

## Security Scanning

- Dependencies are scanned by GitHub Dependency Review
- Dependabot alerts for known vulnerabilities
- SBOM generated during releases for supply chain transparency

## Updating Dependencies

To update dependencies:

```bash
# Update all dependencies
go get -u ./...

# Update specific dependency
go get -u github.com/spf13/cobra

# Tidy module files
go mod tidy

# Verify checksums
go mod verify
```

## Indirect Dependencies

Indirect dependencies are managed automatically by Go modules. Key indirect dependencies include:

- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/alecthomas/chroma/v2` - Syntax highlighting
- `github.com/go-jose/go-jose/v4` - JSON Web Token handling

For a complete list, see `go.sum`.
