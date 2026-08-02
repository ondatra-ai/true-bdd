#!/usr/bin/env bash
# codex.sh — proven non-interactive codex invocation, shared by the task-workflow skills.
#
# Bakes in the flags that matter most: a sandbox policy (without one, `codex exec`
# blocks silently on approval prompts and hangs headlessly forever), ephemeral
# session, no color, repo root, low reasoning effort, and the final answer written
# to a file. Full trace is teed so a timeout still leaves a partial report.
#
# Usage (run from the repo root):
#   ./.claude/skills/implement-task/scripts/codex.sh <mode> <prompt-file> [label]
#
#   mode        ro   = read-only audit/verify (default)
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
