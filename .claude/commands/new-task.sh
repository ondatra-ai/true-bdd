#!/usr/bin/env bash
# Invoked by /new-task (.claude/commands/new-task.md).
#
# Rolls the task history state, then resets the repository to a clean state:
# all local changes are discarded, untracked files removed (ignored files
# like tmp/ and docs/history/ are kept), and the current branch is
# fast-forwarded from origin. The branch is never switched. docs/context/ is
# exempt from the reset so any uncommitted edits to the requirements ledger
# survive a task reset.
set -euo pipefail

ROOT="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"

"$ROOT/.claude/hooks/history.py" new-task

# A new task means the previous implement-task phase state (gates, banners,
# commit block) must not leak forward. tmp/ is gitignored, so git clean below
# won't touch it — clear it explicitly. /new-task is user-invoked, so this is
# an inherently user-consented reset of any half-done task's enforcement state.
rm -rf "$ROOT/tmp/implement-task"

git -C "$ROOT" restore --staged --worktree -- . ':(exclude)docs/context'
git -C "$ROOT" clean -fd
BRANCH=$(git -C "$ROOT" branch --show-current)
git -C "$ROOT" pull --ff-only origin "$BRANCH" || echo "WARNING: pull failed; continuing on local $BRANCH"

echo "repo reset: $BRANCH @ $(git -C "$ROOT" rev-parse --short HEAD), working tree clean (docs/context preserved)"
