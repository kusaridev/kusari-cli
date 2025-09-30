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

### `kusari repo full-scan`

Scans a repository using [Kusari
Inspector](https://www.kusari.dev/inspector). This does a light weight scan on
the full repository.  Usage:

```
kusari repo full-scan <directory>
```

Where `<directory>` is the directory of the git repository you wish to scan.


Examples:

```
kusari repo full-scan ~/git/guac
```

Will perform a light scan on my full `~/git/guac` repository.

Kusari Inspector results will be stored and displayed in the [Kusari
Console](https://console.us.kusari.cloud/analysis/cli).
