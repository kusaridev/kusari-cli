---
name: "kusari-scan"
description: |
  Scan uncommitted local changes with Kusari Inspector, then fix what it finds. Use this whenever the user asks to scan, check, or review their local/uncommitted/working-tree changes for security issues, vulnerabilities, secrets, or SAST findings — and whenever they ask to check changes before committing or opening a PR. This runs the real Kusari Inspector scan via MCP; prefer it over reviewing the diff yourself, because Kusari sees dependency and secret findings that reading the diff cannot produce.
argument-hint: "[base-ref] [repo-path]"
license: apache-2.0
metadata:
  version: 1.0.0
---

# Kusari Inspector — Scan Local Changes

Scan the user's uncommitted work with Kusari Inspector, report what it found, fix it,
and prove the fix by re-scanning.

$ARGUMENTS

If a base ref is given above, use it. If a repo path is given, use it. Otherwise use
the defaults below.

## Defaults

- **repo_path**: the current working directory
- **base_ref**: `HEAD` — this scans **uncommitted changes only**, which is the point
  of this skill
- **output_format**: `sarif`

---

## Step 1: Confirm there is something to scan

```bash
git status --short && echo "---" && git diff --stat HEAD
```

The scan diffs the working tree against `base_ref`. With the default `HEAD` that means
uncommitted changes only, so:

- **Nothing uncommitted** → stop and tell the user there are no local changes to scan.
  Offer the alternative: scanning a whole branch instead, with `base_ref` set to `main`
  or `origin/main`. Do not scan with an empty diff; it just errors.
- **Changes present** → continue.

Untracked files *are* included in the scan, so do not stage or commit anything to make
them visible.

## Step 2: Run the scan

```
mcp__kusari-inspector__scan_local_changes with:
  repo_path: <repo_path>
  base_ref: <base_ref>
  output_format: sarif
```

This takes a minute or more — it uploads the change set and waits for analysis. Do not
poll, retry, or run a second scan while it is in flight.

**If it returns an authentication error**: call `mcp__kusari-inspector__authenticate`,
then retry the scan once.

**If it fails for any other reason**: report the failure and stop. Do **not** substitute
your own reading of the diff and present it as a Kusari result — the user asked for a
Kusari scan, and a manual review is a different thing with different coverage. Say
plainly that the scan failed and what the error was.

## Step 3: Report what it found

The SARIF result carries three kinds of entry:

| Rule ID | What it is | What you get |
|---|---|---|
| `security-analysis` | Overall verdict | `should_proceed`, `health_score`, `justification`, `recommendation` |
| `code-mitigation` | A code finding | File path, start line, code snippet |
| `dependency-mitigation` | A dependency finding | Text only — **no** file or line |

Lead with the overall verdict (`should_proceed` and `health_score`), then list the
findings grouped by kind, most serious first. Cite code findings as `path:line` so the
user can click through. Include the console URL from the result.

If there are no findings, say so and stop — there is nothing to fix.

## Step 4: Fix the findings

Ask before making edits unless the user already said to fix them.

**Code findings** — you have a path and a line number, so go straight there:

- Read the file at that location before editing; the snippet in the result is context,
  not the current file state.
- Make the **minimal** change that resolves the finding. Do not refactor surrounding
  code, rename things, or fix unrelated issues you notice along the way.
- Match the file's existing style and idiom.

**Dependency findings** — these have no location, so find the manifest yourself:

- Update the manifest **and the lockfile together** (`package.json` + `package-lock.json`,
  `go.mod` + `go.sum`, `pyproject.toml` + the lock, and so on). A manifest edit alone
  leaves the lockfile pinning the vulnerable version, and the next scan will still flag it.
- Prefer the smallest version bump that clears the finding.
- If a fix requires a major-version bump or an API change, do not do it silently — say
  what it would take and let the user decide.

**Do not commit anything.** The scan reads uncommitted changes; committing mid-flow
removes them from `git diff HEAD` and the verification scan in Step 5 will see nothing.
Commit only if the user asks.

Anything you could not fix — blocked, needs a decision, out of scope — say so explicitly
rather than leaving it silently unaddressed.

## Step 5: Prove the fix

Re-run the scan from Step 2 with the same arguments.

Editing files changed the diff, so this is a genuinely fresh scan rather than a cached
result. Report the new `should_proceed` and `health_score` against the old ones, and
name anything still outstanding.

---

## Error Handling

- Auth error → call `authenticate`, retry the scan **once**, then stop.
- Scan error → report it and stop. Never present a manual diff review as a Kusari result.
- Not a git repository → say so; this skill only works inside a git repo.
