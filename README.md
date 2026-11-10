# Kusari CLI

Command line interface for Kusari.

## Installation

1. Have [Go installed](https://go.dev/doc/install).

1. `go install github.com/kusaridev/kusari-cli/v2/kusari@latest`

Alternatively, you can install pre-built binaries for supported platforms from
the [GitHub releases page](https://github.com/kusaridev/kusari-cli/releases).

## Usage

For detailed information, see the [Kusari Documentation](https://docs.kusari.cloud/reference/CLI/).

When enabled in a CI/CD environment, Kusari Inspector via the `repo scan` command will:
- Post a summary comment with security findings
- Post inline comments on specific lines of code where issues are detected
- Update existing comments instead of creating duplicates on subsequent runs

**CI/CD Setup Instructions:**

For complete setup instructions, templates, and reusable workflows for both GitLab and GitHub, see the [Kusari CI Templates repository](https://github.com/kusaridev/kusari-ci-templates).

## Tagging deployments from CI

After a deploy, record which image SBOM version is running in which environment:

```bash
kusari platform sboms tag-environment <sbom-id> <version-id> <environment> --commit-time <RFC3339>
```

It tags the image SBOM version `environment=<environment>` and ends that tag on the SBOM's other
versions. It then finds the source SBOM in the same component and does the same for the source SBOM
version deployed with it. The environment can be any value (`prod`, `staging`, ...); it is stored
lowercase. `--commit-time` is the deployed commit's committer date. It is used only when the source
SBOM has no version at the commit recorded on the image version, and it cannot be `now`.

Get the SBOM and version IDs of the deployed image from your upload, or with
`kusari platform sboms id-by-identifier --commit-sha <sha>`.

Progress goes to stderr. stdout is one JSON object:

```json
{
  "environment": "prod",
  "image": {
    "sbom_id": 42,
    "version_id": 99,
    "result": "created",
    "ended_version_ids": [98]
  },
  "source": {
    "sbom_id": 14,
    "name": "app-source",
    "version_id": 4790,
    "version_found_by": "commit",
    "result": "created",
    "ended_version_ids": [4788],
    "message": "version 4790 is at commit 1a2b3c4"
  }
}
```

- `result` is `created`, `already_tagged` (a rerun leaves the tag as is), or, for the source only,
  `not_tagged`.
- `source` is `null` when the image SBOM is in no component or its component has no source SBOM.
  The reason is printed on stderr.
- When the source SBOM has no version to tag, `source.result` is `not_tagged`, `version_id` is left
  out, and stderr has a line starting with `warning:`.

Exit codes:

| Code | Meaning |
| ---- | ------- |
| 0 | The image SBOM version is tagged. The source SBOM version is tagged, not tagged (with a `warning:`), or there is no source SBOM. |
| 1 | A bad argument, or an API error. On an API error the JSON for the steps that finished is still printed. If tagging the image fails, the source SBOM is not touched. |

The [API key](https://docs.kusari.cloud/administration/api-keys) used to log in must be scoped
and include:

| Permission | Used to |
| ---------- | ------- |
| `platform_data_read` | Look up SBOMs, versions, tags and components |
| `platform_data_write` | Create tags, and find versions by commit |
| `software_metadata_write` | End the tag on other versions |

GitLab CI job, with `KUSARI_CLIENT_ID`, `KUSARI_CLIENT_SECRET`, `SBOM_ID` and `VERSION_ID` set as
CI/CD variables:

```yaml
tag-environment:
  stage: deploy
  image: golang:1.26
  needs: [deploy-prod]
  script:
    - go install github.com/kusaridev/kusari-cli/v2/kusari@latest
    - kusari auth login --client-id "$KUSARI_CLIENT_ID" --client-secret "$KUSARI_CLIENT_SECRET"
    - kusari platform sboms tag-environment "$SBOM_ID" "$VERSION_ID" prod --commit-time "$CI_COMMIT_TIMESTAMP"
```

Jenkins `sh` step, after the deploy step, with the credentials bound to `KUSARI_CLIENT_ID` and
`KUSARI_CLIENT_SECRET`:

```groovy
sh '''
  go install github.com/kusaridev/kusari-cli/v2/kusari@latest
  kusari auth login --client-id "$KUSARI_CLIENT_ID" --client-secret "$KUSARI_CLIENT_SECRET"
  kusari platform sboms tag-environment "$SBOM_ID" "$VERSION_ID" prod --commit-time "$(git log -1 --format=%cI)"
'''
```

## Development

Build the binary locally

```bash
go build -o kusari/kusari ./kusari      # kusari/kusari is gitignored
```

Run the local binary as `./kusari/kusari` from this directory.

```bash
./kusari/kusari --help

# Log in with the local binary:
./kusari/kusari auth login
``