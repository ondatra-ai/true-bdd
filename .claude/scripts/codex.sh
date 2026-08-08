#!/usr/bin/env bash
# codex.sh — non-interactive codex invocation wrapper.
#
# Bakes in the flags that matter most — above all a sandbox policy: without one,
# `codex exec` blocks silently on approval prompts and hangs headlessly forever.
#
# Usage (run from the repo root):
#   ./.claude/scripts/codex.sh <mode> <prompt-file> [label]
#
#   mode        ro   = read-only audit/verify
#                auto = workspace-write (Codex may edit files / run side effects)
#   prompt-file path to the prompt, read via stdin
#   label       filename label (default: review) -> ./tmp/codex-<label>.md + .trace.log
#
# Launch this as a BACKGROUND task and arm a Monitor that fires on exit.
set -euo pipefail

mode="${1:?usage: codex.sh <ro|auto> <prompt-file> [label]}"
prompt_file="${2:?prompt file required}"
label="${3:-review}"

mkdir -p ./tmp
out="./tmp/codex-${label}.md"
trace="./tmp/codex-${label}.trace.log"

# Refuse the prompt==answer collision: -o "$out" overwrites the prompt file.
# The run still "works" (stdin is consumed before -o writes) but the prompt
# artifact is destroyed. Fail loudly before burning a Codex call so the caller
# uses distinct paths
# (e.g. <label>-rN-prompt.md in, <label>-rN.md out).
if [ "$(cd "$(dirname "$out")" && pwd -P)/$(basename "$out")" = \
      "$(cd "$(dirname "$prompt_file")" && pwd -P)/$(basename "$prompt_file")" ]; then
  printf 'codex.sh: -o answer path (%s) equals the prompt file — use distinct paths\n' "$out" >&2
  exit 3
fi

case "$mode" in
  ro)   sandbox=(-s read-only --ephemeral) ;;
  auto) sandbox=(-s workspace-write --ephemeral) ;;
  *)    printf 'unknown mode %q (use ro or auto)\n' "$mode" >&2; exit 2 ;;
esac

# Read prompt from stdin (-); -o captures the final answer so trace can't bury it.
codex exec "${sandbox[@]}" \
  -C "$PWD" --color never \
  -c model_reasoning_effort=low \
  -o "$out" \
  - < "$prompt_file" 2>&1 | tee "$trace"

echo "---- codex done: answer in $out, full trace in $trace"
