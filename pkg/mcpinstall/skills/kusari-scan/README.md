# Scan Local Changes

Scans your **uncommitted** work with Kusari Inspector, reports what it found, fixes it,
and re-scans to prove the fix.

## What Makes This Different

Kusari Inspector analyzes code that exists only in your working tree — never committed,
never pushed, never seen by CI. Code findings come back with a file path and line number,
so Claude can fix them in place in the same conversation.

**Key Features**:
- Scans uncommitted changes, including untracked files
- Secrets, SAST, and dependency findings in one pass
- Code findings carry `path:line`, so fixes are applied directly
- Re-scans after fixing to confirm the verdict flipped
- Runs against the Kusari platform, not a local heuristic review

## Usage

```
/kusari-scan [base-ref] [repo-path]
```

Examples:
```
/kusari-scan                    # uncommitted changes in the current repo
/kusari-scan main               # everything on this branch vs main
/kusari-scan HEAD ./services/api    # uncommitted changes in a subdirectory
```

Claude also reaches for this on its own when you ask things like *"scan my local changes"*
or *"check my changes for security issues before I commit."*

## Base ref

The default `base_ref` is `HEAD`, which scans **uncommitted changes only**. That is
usually what you want.

Pass `main` or `origin/main` instead to scan an entire branch, including commits you have
already made locally.

## Requirements

- The Kusari MCP server installed: `kusari ai install claude-code`
- An authenticated session: `kusari auth login`

If your session has expired, the skill calls the `authenticate` tool and retries once.
