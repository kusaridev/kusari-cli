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

**Output Formats:**

The scan command supports multiple output formats via the `--output-format` flag:

```bash
kusari repo scan --output-format sarif . origin/main
```

Available formats:
- `json` (default) - Detailed JSON output with full analysis results
- `sarif` - SARIF format compatible with GitLab SAST reports

The SARIF format integrates with GitLab's Security Dashboard when used as a CI artifact.

#### GitLab Integration

The `kusari repo scan` command supports automatic posting of scan results as comments on GitLab merge requests using the `--gitlab-comment` flag:

```bash
kusari repo scan --gitlab-comment . origin/main
```

When enabled in a GitLab CI environment, this will:
- Post a summary comment to the merge request with security findings
- Post inline comments on specific lines of code where issues are detected
- Update existing comments instead of creating duplicates on subsequent runs

**Required Environment Variables:**
- `CI_PROJECT_ID` - GitLab project ID (automatically set by GitLab CI)
- `CI_MERGE_REQUEST_IID` - Merge request IID (automatically set by GitLab CI)
- `GITLAB_TOKEN` or `CI_JOB_TOKEN` - Token with API access for posting comments

**GitLab CI/CD Template:**

For easy integration, use the provided GitLab CI/CD template:

```yaml
include:
  - remote: 'https://raw.githubusercontent.com/kusaridev/kusari-cli/main/ci-templates/gitlab/kusari-scan.yml'
```

**Setup Instructions:**

1. Add the CI/CD variables to your GitLab project (Settings > CI/CD > Variables):
   - `KUSARI_CLIENT_ID` - Your Kusari client ID
   - `KUSARI_CLIENT_SECRET` - Your Kusari client secret (mark as masked)

2. Set up GitLab token for posting comments (choose one option):

   **Option A - Use a GitLab Token (Recommended):**
   - Create a Project Access Token with `api` scope (Settings > Access Tokens)
   - Add it as `GITLAB_TOKEN` variable (mark as masked)

   **Option B - Enable CI_JOB_TOKEN API Access:**
   - Go to Settings > CI/CD > Token Access
   - Enable "Allow CI job tokens from this project to access this project's API"

The template will automatically run on merge requests and post security findings as comments.

For more details, see the [template documentation](https://github.com/kusaridev/kusari-cli/blob/main/ci-templates/gitlab/kusari-scan.yml).

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
