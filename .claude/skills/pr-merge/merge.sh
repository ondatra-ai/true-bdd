#!/usr/bin/env bash
# Squash-merge the current branch's PR, then clean up local and remote.
set -e

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# `main` requires an approving review, the `gates` check, and every review
# thread resolved. Ask first and name which one is missing — left to
# `gh pr merge`, all three surface as one generic refusal, and `set -e`
# then kills the script before it says anything useful.
if ! "$HERE/threads.sh" preflight; then
  echo "" >&2
  echo "Not merging. Run './.claude/skills/pr-merge/threads.sh status' for the detail." >&2
  exit 1
fi

CURRENT_BRANCH=$(git branch --show-current)
gh pr merge --squash --delete-branch
MAIN_BRANCH=$(git show-ref --verify --quiet refs/heads/master && echo master || echo main)
git checkout "$MAIN_BRANCH"
git pull origin "$MAIN_BRANCH"
git branch -D "$CURRENT_BRANCH" 2>/dev/null || echo "Local branch '$CURRENT_BRANCH' already deleted"
