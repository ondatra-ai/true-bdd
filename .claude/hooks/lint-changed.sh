#!/usr/bin/env bash
# Usage: lint-changed.sh   (PostToolUse hook; the tool payload arrives on stdin)
# Lint the file Claude just wrote and hand the verdict straight back to it.
#
# The verdict travels as JSON on stdout with exit 0 — the shape every
# published lint hook settled on, and the one that carries an instruction as
# well as a finding. `reason` is DISCARDED unless `decision` is "block", so a
# finding sent without one is a finding nobody reads.
#
# Nothing here announces what `--fix` rewrote: Claude Code posts its own
# "PostToolUse hook modified <file>" notice, measured on this hook.
#
# Everything unjudgeable — no file_path, a path outside the repository, an
# ignored file — exits 0 and says nothing.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

path=$(jq -r '.tool_input.file_path // empty')
[ -n "$path" ] || exit 0

case "$path" in
"$ROOT"/*) rel=${path#"$ROOT"/} ;;
*) exit 0 ;;
esac

if git -C "$ROOT" check-ignore -q "$rel"; then
	exit 0
fi

if output=$("$ROOT/scripts/lints.sh" "$rel" 2>&1); then
	exit 0
fi

reason="LINT FAILED on $rel. Fix it in that file now, before any other work:
these same gates run at commit time and reject the branch otherwise. What was
auto-fixable is already applied; what follows needs a real edit.

$output"

printf '{"decision":"block","reason":%s}\n' "$(printf '%s' "$reason" | jq -Rs .)"
