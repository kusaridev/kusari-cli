# Dependencies

This document describes the direct dependencies used by the Kusari CLI and their purposes.

## Direct Dependencies

| Package | Version | Purpose | License |
|---------|---------|---------|---------|
| [github.com/briandowns/spinner](https://github.com/briandowns/spinner) | v1.23.2 | Terminal spinner for progress indication | Apache-2.0 |
| [github.com/charmbracelet/glamour](https://github.com/charmbracelet/glamour) | v0.10.0 | Markdown rendering in terminal | MIT |
| [github.com/coreos/go-oidc/v3](https://github.com/coreos/go-oidc) | v3.17.0 | OpenID Connect client for authentication | Apache-2.0 |
| [github.com/spf13/cobra](https://github.com/spf13/cobra) | v1.10.2 | CLI framework and command structure | Apache-2.0 |
| [github.com/spf13/pflag](https://github.com/spf13/pflag) | v1.0.10 | POSIX/GNU-style CLI flags | BSD-3-Clause |
| [github.com/spf13/viper](https://github.com/spf13/viper) | v1.21.0 | Configuration management | MIT |
| [github.com/stretchr/testify](https://github.com/stretchr/testify) | v1.11.1 | Testing assertions and mocks | MIT |
| [golang.org/x/oauth2](https://golang.org/x/oauth2) | v0.34.0 | OAuth 2.0 client implementation | BSD-3-Clause |
| [golang.org/x/sync](https://golang.org/x/sync) | v0.19.0 | Synchronization primitives | BSD-3-Clause |
| [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) | v3.0.1 | YAML parsing and serialization | MIT |

## Dependency Categories

### Authentication & Authorization
- **go-oidc**: Handles OpenID Connect authentication flows with Kusari's identity provider
- **oauth2**: Provides OAuth 2.0 token management and refresh capabilities

### CLI Framework
- **cobra**: Powers the command-line interface structure and subcommands
- **pflag**: Handles command-line flag parsing
- **viper**: Manages configuration files and environment variables

### User Interface
- **spinner**: Displays progress spinners during long-running operations
- **glamour**: Renders markdown output for rich terminal display

### Data Processing
- **yaml.v3**: Parses and generates YAML configuration files

### Testing
- **testify**: Provides testing utilities, assertions, and mocking capabilities

### Concurrency
- **sync**: Extended synchronization primitives for concurrent operations

## Dependency Management

Dependencies are managed using Go modules (`go.mod` and `go.sum`). To update dependencies:

```bash
# Update all dependencies
go get -u ./...

# Update a specific dependency
go get -u github.com/spf13/cobra@latest

# Tidy dependencies
go mod tidy
```

## Security Considerations

- All dependencies are pinned to specific versions in `go.sum`
- Dependabot is configured to automatically check for security updates
- Dependencies are scanned during CI/CD using GitHub's dependency review action

## License Compliance

All direct dependencies use OSI-approved open source licenses compatible with the MIT license used by this project.
