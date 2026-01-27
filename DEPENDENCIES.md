# Dependencies

This document lists the dependencies used by Kusari CLI and their purposes.

## Direct Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| [github.com/briandowns/spinner](https://github.com/briandowns/spinner) | v1.23.2 | Terminal progress indicators |
| [github.com/charmbracelet/glamour](https://github.com/charmbracelet/glamour) | v0.10.0 | Markdown rendering in terminal |
| [github.com/coreos/go-oidc/v3](https://github.com/coreos/go-oidc) | v3.17.0 | OpenID Connect client |
| [github.com/spf13/cobra](https://github.com/spf13/cobra) | v1.10.2 | CLI framework |
| [github.com/spf13/pflag](https://github.com/spf13/pflag) | v1.0.10 | POSIX-style flags |
| [github.com/spf13/viper](https://github.com/spf13/viper) | v1.21.0 | Configuration management |
| [github.com/stretchr/testify](https://github.com/stretchr/testify) | v1.11.1 | Testing utilities |
| [golang.org/x/oauth2](https://golang.org/x/oauth2) | v0.34.0 | OAuth2 client |
| [golang.org/x/sync](https://golang.org/x/sync) | v0.19.0 | Synchronization primitives |
| [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3) | v3.0.1 | YAML parsing |

## Indirect Dependencies

These are transitive dependencies required by the direct dependencies above:

| Package | Purpose |
|---------|---------|
| github.com/alecthomas/chroma/v2 | Syntax highlighting (glamour) |
| github.com/aymanbagabas/go-osc52/v2 | Terminal clipboard (glamour) |
| github.com/aymerick/douceur | CSS parsing (glamour) |
| github.com/charmbracelet/lipgloss | Terminal styling (glamour) |
| github.com/davecgh/go-spew | Debug printing (testify) |
| github.com/dlclark/regexp2 | Regular expressions (chroma) |
| github.com/fatih/color | Terminal colors (spinner) |
| github.com/fsnotify/fsnotify | File watching (viper) |
| github.com/go-jose/go-jose/v4 | JOSE/JWT (go-oidc) |
| github.com/go-viper/mapstructure/v2 | Struct mapping (viper) |
| github.com/gorilla/css | CSS parsing (bluemonday) |
| github.com/inconshreveable/mousetrap | Windows support (cobra) |
| github.com/lucasb-eyer/go-colorful | Color manipulation (lipgloss) |
| github.com/mattn/go-colorable | Windows colors (spinner) |
| github.com/mattn/go-isatty | TTY detection |
| github.com/mattn/go-runewidth | Unicode width (glamour) |
| github.com/microcosm-cc/bluemonday | HTML sanitization (glamour) |
| github.com/muesli/reflow | Text reflow (glamour) |
| github.com/muesli/termenv | Terminal detection (glamour) |
| github.com/pelletier/go-toml/v2 | TOML parsing (viper) |
| github.com/pmezard/go-difflib | Diff generation (testify) |
| github.com/rivo/uniseg | Unicode segmentation |
| github.com/sagikazarmark/locafero | File locator (viper) |
| github.com/sourcegraph/conc | Concurrency utilities (viper) |
| github.com/spf13/afero | Filesystem abstraction (viper) |
| github.com/spf13/cast | Type casting (viper) |
| github.com/subosito/gotenv | Environment files (viper) |
| github.com/yuin/goldmark | Markdown parsing (glamour) |
| github.com/yuin/goldmark-emoji | Emoji support (glamour) |
| golang.org/x/net | Network utilities |
| golang.org/x/sys | System calls |
| golang.org/x/term | Terminal utilities |
| golang.org/x/text | Text processing |

## Dependency Management

Dependencies are managed using Go modules (`go.mod` and `go.sum`).

### Updating Dependencies

```bash
# Update all dependencies
go get -u ./...

# Update a specific dependency
go get -u github.com/spf13/cobra@latest

# Tidy up go.mod and go.sum
go mod tidy
```

### Security Scanning

Dependencies are automatically scanned for vulnerabilities using:
- GitHub Dependabot (configured in `.github/dependabot.yml`)
- Dependency review on pull requests

## License Compliance

All dependencies are MIT, BSD, or Apache 2.0 licensed, compatible with the MIT license of this project.
