#!/usr/bin/env bash
# Invoked by /new-task (.claude/commands/new-task.md).
#
# Rolls the task history state, then resets the repository to a clean state:
# all local changes are discarded, untracked files removed (ignored files
# like tmp/ and docs/history/ are kept), and the main branch checked out and
# fast-forwarded to origin. docs/context/ is exempt from the reset so any
# uncommitted edits to the requirements ledger survive a task reset.
set -euo pipefail

ROOT="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"

"$ROOT/.claude/hooks/history.py" new-task

git -C "$ROOT" restore --staged --worktree -- . ':(exclude)docs/context'
git -C "$ROOT" clean -fd
MAIN_BRANCH=$(git -C "$ROOT" show-ref --verify --quiet refs/heads/master && echo master || echo main)
[ "$(git -C "$ROOT" branch --show-current)" = "$MAIN_BRANCH" ] || git -C "$ROOT" checkout -q "$MAIN_BRANCH"
git -C "$ROOT" pull --ff-only origin "$MAIN_BRANCH" || echo "WARNING: pull failed; continuing on local $MAIN_BRANCH"

echo "repo reset: $MAIN_BRANCH @ $(git -C "$ROOT" rev-parse --short HEAD), working tree clean (docs/context preserved)"
