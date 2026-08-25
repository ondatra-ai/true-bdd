#!/usr/bin/env bash
# Invoked by /task-start (SKILL.md) through the `!` injection, before the model
# reads a word.
#
# Rolls the Task history state and nothing else: the state file goes, so the
# next prompt opens a fresh file under docs/history/. It does NOT touch the
# working tree — starting a Task must not be able to discard uncommitted work —
# and it does NOT touch ClickUp, which is the skill body's job.
set -euo pipefail

ROOT="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"

# Whatever is bound belongs to the Task that just ended. Leaving it would let a
# later /task-done close a Ticket this Task never touched — but say which one
# was dropped, because it is still PROCESSING and nothing else will mention it.
ORPHAN="$("$ROOT/.claude/hooks/history.sh" bound)"

"$ROOT/.claude/hooks/history.sh" new-task
"$ROOT/.claude/hooks/history.sh" unbind

echo "history rolled: the next prompt opens a fresh file in docs/history/"

if [ -n "$ORPHAN" ]; then
  echo "WARNING: ticket $ORPHAN was still bound and is now unbound."
  echo "It is still PROCESSING in ClickUp — nobody closed it. Tell the user."
fi
