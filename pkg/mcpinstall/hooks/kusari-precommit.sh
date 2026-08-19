#!/usr/bin/env bash
#
# Kusari Inspector pre-commit scan for Claude Code.
#
# Installed by `kusari ai install --with-commit-hook` as a PreToolUse hook on
# Bash. It reads the hook payload on stdin and, when the command is a git
# commit, scans the working tree before the commit is allowed to run.
#
# POLICY: this hook blocks only on a definite "there are findings" signal.
# Everything else allows the commit through -- not a commit, not a git repo,
# expired credentials, a scan failure, output it could not parse. A developer
# cannot log in from inside a hook, so blocking on an expired token would
# strand them with no way forward.
#
# Blocking uses exit code 2 with the reason on stderr, which Claude Code feeds
# back to the model. That avoids having to JSON-escape scan output from shell.

set -u

# Absolute path, substituted at install time: a hook does not necessarily
# inherit the interactive shell's PATH.
KUSARI_BIN="__KUSARI_BIN__"

# Allow silently.
allow() { exit 0; }

# Allow, but surface a one-line reason. The JSON here is always a literal
# authored in this script, never interpolated scan output, so it cannot be
# malformed by a finding that contains quotes or newlines.
skip() {
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"},"systemMessage":"Kusari: %s"}\n' "$1"
  exit 0
}

payload=$(cat)

# Only interested in git commits. Matching the raw payload avoids a jq
# dependency; the pattern also catches "git -C <path> commit" and a commit
# chained after another command ("git add -A && git commit ..."), which a
# settings-level `if` filter on a command prefix would miss.
if ! printf '%s' "$payload" \
  | grep -Eq 'git([[:space:]]+-[^[:space:]]+)*([[:space:]]+[^[:space:]]+)?[[:space:]]+commit([[:space:]]|\\|"|$)'; then
  allow
fi

command -v "$KUSARI_BIN" >/dev/null 2>&1 || [ -x "$KUSARI_BIN" ] || skip "scan skipped (kusari binary not found at $KUSARI_BIN)"

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || skip "scan skipped (not a git repository)"
[ -n "$repo_root" ] || skip "scan skipped (not a git repository)"

# Nothing uncommitted means nothing for a diff scan to look at. This happens
# for an empty or amend-only commit.
if git diff --quiet HEAD 2>/dev/null && [ -z "$(git ls-files --others --exclude-standard)" ]; then
  allow
fi

# --full-output because the CLI truncates by default, and a truncated finding
#   list would let the model believe it had addressed everything.
# --fail-on-findings so the verdict arrives as an exit code. Without it this
#   script had to dig should_proceed out of the SARIF document, which meant a jq
#   dependency, a grep fallback coupled to the JSON emitter's exact indentation,
#   and one genuinely dangerous failure mode: jq's `//` operator treats false the
#   same as null, so the one value meaning "there are findings" silently became
#   "no verdict" and the commit sailed through.
# markdown rather than sarif because the verdict no longer has to be machine
#   readable, and markdown is what the model reads best.
output=$("$KUSARI_BIN" repo scan "$repo_root" HEAD \
  --output-format markdown \
  --full-output \
  --fail-on-findings \
  --wait 2>&1)

case $? in
  0)
    allow
    ;;
  3)
    # Exit 3 is the scan's dedicated "analysis says do not proceed" status. It is
    # deliberately distinct from 1, so an outage cannot be mistaken for a finding.
    {
      echo "Kusari Inspector found issues in the changes about to be committed."
      echo
      echo "$output"
      echo
      echo "Fix these before committing. Do not bypass this by amending or"
      echo "re-running the commit unchanged -- the findings are in the working"
      echo "tree, so they will still be there."
    } >&2
    exit 2
    ;;
  *)
    # Any other status is the scan itself failing, including expired
    # credentials. Allow the commit: a developer cannot log in from inside a
    # hook, so blocking here would strand them with no way forward.
    if printf '%s' "$output" | grep -qi 'auth token\|token is expired\|authentication required'; then
      # No quotes around the command: the message is interpolated into a JSON
      # string, and a stray quote there produces invalid JSON rather than a
      # readable message.
      skip "scan skipped -- run: kusari auth login"
    fi
    skip "scan skipped (scan failed; commit allowed through)"
    ;;
esac
