#!/usr/bin/env bash
set -e

CURRENT_BRANCH=$(git branch --show-current)
gh pr merge --squash --delete-branch
MAIN_BRANCH=$(git show-ref --verify --quiet refs/heads/master && echo master || echo main)
git checkout "$MAIN_BRANCH"
git pull origin "$MAIN_BRANCH"
git branch -D "$CURRENT_BRANCH" 2>/dev/null || echo "Local branch '$CURRENT_BRANCH' already deleted"
