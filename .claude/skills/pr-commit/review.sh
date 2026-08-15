#!/usr/bin/env bash
# Usage: review.sh [--base <branch>]
#
# Runs CodeRabbit over the pending change BEFORE it is committed, and
# writes its findings to ./tmp/review-findings.jsonl.
#
# Before, not after, for a reason this repository has already paid for:
# the first reviewer to see a change used to be the GitHub bot, which
# reviews what has already been pushed. A recording carrying a developer's
# home directory and their connected integrations reached a public repo
# that way. The same reviewer, run here, reports it while it is still a
# working-tree edit.
#
# It finds; it does not decide. Triage stays with the agent, which reads
# the file the finding points at and rules Fix now / Defer / Reject — a
# finder that judged its own findings would have nobody checking it.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel) || {
  echo "not inside a git repository — nothing to review." >&2
  exit 1
}
cd "$ROOT"

OUT=./tmp/review-findings.jsonl
mkdir -p ./tmp

if ! command -v coderabbit >/dev/null 2>&1; then
  echo "coderabbit CLI not found — skipping the pre-commit review round." >&2
  echo "  install: curl -fsSL https://cli.coderabbit.ai/install.sh | sh && coderabbit auth login" >&2
  : > "$OUT"
  exit 0
fi

# --include-untracked matters more here than anywhere else: a brand-new
# fixture recording is exactly the file most likely to carry something
# that should not be published, and it is untracked until the moment it
# is staged.
TARGET=(--uncommitted --include-untracked)
if [ "${1:-}" = "--base" ]; then
  TARGET=(--base "${2:?--base needs a branch}")
fi

# CodeRabbit refuses more than 150 files in one review, and a single
# re-record of the fixtures changes ~475 — recordings the reviewer has
# nothing useful to say about anyway. So the review is scoped to the
# AUTHORED directories: the ones holding files a person wrote. This is
# the same generated-vs-authored split the commit message needs, applied
# to the same problem from the other side.
# Recordings are excluded from the LLM review and checked by
# scan-recordings.sh instead — deterministically, exhaustively, and
# without a per-review file limit. Excluding them here is only safe
# BECAUSE that scan exists; drop one and the other stops being enough.
GENERATED='/cassettes/|^services/bdd-web/src/|package-lock\.json$'

# Bash 3 on macOS has no mapfile, and this script has to run on the
# machine it is committed from.
# Discovery follows the SAME change set the review targets: comparing
# against HEAD while reviewing against a base branch would scope the
# review to the wrong directories.
DIFF_BASE=HEAD
if [ "${1:-}" = "--base" ]; then
  DIFF_BASE="origin/${2}...HEAD"
fi

DIRS=()
while IFS= read -r dir; do
  [ -n "$dir" ] && DIRS+=("$dir")
done < <(
  { git diff --name-only "$DIFF_BASE"; git ls-files --others --exclude-standard; } \
    | grep -vE "$GENERATED" \
    | sed 's|/[^/]*$||' \
    | sed 's|^\(\([^/]*/\)\{0,2\}[^/]*\).*|\1|' \
    | sort -u
)

echo "coderabbit: reviewing ${TARGET[*]} across ${#DIRS[@]} authored dir(s) — a few minutes …" >&2
START=$(date +%s)

: > "$OUT"
: > ./tmp/review-findings.err
STATUS=0

for dir in "${DIRS[@]}"; do
  [ -d "$dir" ] || continue

  coderabbit review "${TARGET[@]}" --dir "$dir" --agent >> "$OUT" 2>>./tmp/review-findings.err || STATUS=$?
done

ELAPSED=$(( $(date +%s) - START ))

# Reported even when other directories succeeded: a partial review that
# looks complete is how a finding goes missing.
if [ $STATUS -ne 0 ]; then
  echo "coderabbit: at least one directory failed (exit $STATUS after ${ELAPSED}s)" >&2
  echo "  the findings below cover only the directories that succeeded — see tmp/review-findings.err" >&2
fi

python3 - "$OUT" "$ELAPSED" <<'PY'
import json, sys
from collections import Counter

path, elapsed = sys.argv[1], int(sys.argv[2])
findings, severities = [], Counter()

for line in open(path, errors="replace"):
    line = line.strip()
    if not line:
        continue
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    if event.get("type") == "finding":
        findings.append(event)
        severities[event.get("severity", "?")] += 1

summary = ", ".join(f"{n} {sev}" for sev, n in severities.most_common()) or "none"
print(f"coderabbit: {len(findings)} finding(s) in {elapsed}s — {summary}", file=sys.stderr)

for finding in findings:
    # The first sentence of the instruction is the claim; the rest is
    # the bot's boilerplate about not trusting review data.
    claim = finding.get("codegenInstructions", "").split("\n\n")[-1].strip()
    print(f"  [{finding.get('severity','?'):8s}] {finding.get('fileName','?')}\n      {claim[:180]}",
          file=sys.stderr)
PY
