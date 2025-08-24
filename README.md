# Kusari CLI

Command line interface for Kusari.

## Installation

1. Have [Go installed](https://go.dev/doc/install).

1. `go install github.com/kusaridev/kusari-cli/kusari@latest`

## Commands

### `kusari auth login`

Logs into Kusari. Default parameters are good for most use cases. Access token
is stored in `$HOME/.kusari/tokens.json`.

### `kusari repo scan`

Scans a diff on a git repository using [Kusari
Inspector](https://www.kusari.dev/inspector). This will scan a set of changes,
so a "git diff" path is needed. Usage:

```
kusari repo scan <directory> <git-diff path>
```

Where `<directory>` is the directory of the git repository you wish to scan,
and `<git-diff path>` is the path to pass to `git diff` to generate the
set of changes. See [Git
documentation](https://git-scm.com/docs/git-diff#_examples), for examples of
commands.

Example:

```
kusari repo scan ~/git/guac HEAD^
```

Will scan my `~/git/guac` repository with the git-diff command `HEAD^` which
compares my working tree with the commit before the most recent commit.

Kusari Inspector results will be stored and displayed in the [Kusari
Console](https://console.us.kusari.cloud/analysis/cli).
