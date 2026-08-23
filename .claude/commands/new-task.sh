#!/usr/bin/env bash
# Invoked by /new-task (.claude/commands/new-task.md).
#
# Rolls the task history state, and nothing else: the state file goes, so the
# next prompt opens a fresh file under docs/history/. It does NOT touch the
# working tree — starting a task must not be able to discard uncommitted work.
set -euo pipefail

ROOT="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"

"$ROOT/.claude/hooks/history.py" new-task

# history.py is silent by design, so say what happened — otherwise /new-task
# prints nothing and reads as a no-op.
echo "history rolled: the next prompt opens a fresh file in docs/history/"
