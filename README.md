# Kusari CLI

Command line interface for Kusari.

## Installation

1. Have [Go installed](https://go.dev/doc/install).

1. `go install github.com/kusaridev/kusari-cli/kusari@latest`

Alternatively, you can install pre-built binaries for supported platforms from
the [GitHub releases page](https://github.com/kusaridev/kusari-cli/releases).

## Usage

For detailed information, see the [Kusari Documentation](https://docs.kusari.cloud/docs/CLI/).

### `kusari auth login`

Logs into Kusari. Default parameters are good for most use cases. Access token
is stored in `$HOME/.kusari/tokens.json`.

#### Workspace Selection

During your first login, you'll be prompted to select a workspace. Your selected
workspace determines where your scan results will be stored.

**Interactive Login (default):**
```bash
kusari auth login
```

If you have multiple workspaces, you'll see a prompt:
```
Available workspaces:
  [1] Development Workspace
  [2] Production Workspace

Select a workspace (1-2):
```

If you only have one workspace, it will be auto-selected.

**CI/CD Login (with client secret):**
```bash
kusari auth login --client-secret <secret> --client-id <id>
```

In CI/CD mode, the first available workspace is automatically selected without
prompting, ensuring non-interactive execution.

**SSO Login (SAML authentication):**
```bash
kusari auth login --use-sso
```

Use the `--use-sso` flag to authenticate with your organization's SSO (SAML)
identity provider. This redirects you to your corporate login page and opens
the Kusari console with SSO parameters after successful authentication.

**Workspace Storage:**
Your selected workspace is stored in `$HOME/.kusari/workspace.json` and is tied
to your current platform and authentication endpoint. If you switch environments
(e.g., from production to development), you'll be prompted to select a workspace
for the new environment.

**Switching Environments:**
If you need to work with different Kusari environments (development, production, etc.),
use the `--platform-url` and `--auth-endpoint` flags:

```bash
# Development environment
kusari auth login --platform-url https://platform.api.dev.kusari.cloud/ \
  --auth-endpoint https://auth.dev.kusari.cloud/

# Production environment (default)
kusari auth login
```

When you change environments, the CLI will detect the mismatch and prompt you to
select a workspace for the new environment.

### `kusari auth select-workspace`

Change your active workspace without re-authenticating. This is useful when you
need to switch between workspaces in the same environment.

```bash
kusari auth select-workspace
```

You'll be shown your current workspace and prompted to select a new one:
```
Current workspace: Development Workspace

Available workspaces:
  [1] Development Workspace
  [2] Production Workspace

Select a workspace (1-2):
```

### `kusari repo scan`

Scans a diff on a git repository using [Kusari
Inspector](https://www.kusari.dev/inspector). This will scan a set of changes,
so a git revision is needed to compare to. Usage:

```
kusari repo scan <directory> <git-rev>
```

Where `<directory>` is the directory of the git repository you wish to scan,
and `<git-rev>` is the git revision to compare with the working tree to
generate the set of changes. See [Git
documentation](https://git-scm.com/docs/gitrevisions), for examples of git
revisions. The revision must be a single revision to compare the working tree
against, not a range.

The scan will use your currently selected workspace, which will be displayed at
the start of the scan:
```
Packaging directory...
Using workspace: Development Workspace
Uploading package repo...
```

Examples:

```
kusari repo scan ~/git/guac HEAD^
```

Will scan my `~/git/guac` repository and compare the working tree with the
commit before the most recent commit.

```
kusari repo scan ~/git/guac origin/main
```

Will scan my `~/git/guac` repository and compare the working tree with the
`main` branch from the remote `origin`.

Kusari Inspector results will be stored and displayed in the [Kusari
Console](https://console.us.kusari.cloud/analysis/cli).

### `kusari repo risk-check`

Analyze a repository's overall security posture (Coming soon!)

<!-- 
Scans a repository for risks using [Kusari
Inspector](https://www.kusari.dev/inspector). This does a risk check on
the full repository.  Usage:

```
kusari repo risk-check <directory>
```

Where `<directory>` is the directory of the git repository you wish to scan.


Examples:

```
kusari repo risk-check ~/git/guac
```

Will perform a risk check on my full `~/git/guac` repository.

Kusari Inspector results will be stored and displayed in the [Kusari
Console](https://console.us.kusari.cloud/analysis/cli). -->
