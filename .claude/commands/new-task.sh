#!/usr/bin/env bash
# Invoked by /new-task (.claude/commands/new-task.md).
#
# Rolls the task history state, then resets the repository to a clean state:
# all local changes are discarded, untracked files removed (ignored files
# like tmp/ and docs/history/ are kept), and the current branch is
# fast-forwarded from origin. The branch is never switched.
set -euo pipefail

ROOT="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"

"$ROOT/.claude/hooks/history.py" new-task

git -C "$ROOT" restore --staged --worktree -- .
git -C "$ROOT" clean -fd
BRANCH=$(git -C "$ROOT" branch --show-current)
git -C "$ROOT" pull --ff-only origin "$BRANCH" || echo "WARNING: pull failed; continuing on local $BRANCH"

echo "repo reset: $BRANCH @ $(git -C "$ROOT" rev-parse --short HEAD), working tree clean"
