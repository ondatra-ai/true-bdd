#!/usr/bin/env bash
# Usage: commit.sh
# Stages all changes, asks Claude (via `claude -p`) to generate the commit
# message from the staged diff, commits, pushes, then chains to pr-update.sh.
set -euo pipefail

if ! command -v claude >/dev/null 2>&1; then
  echo "claude CLI not found in PATH." >&2
  exit 1
fi

mkdir -p ./tmp
COMMIT_MSG_FILE=./tmp/commit-msg.txt
BRANCH_NAME_FILE=./tmp/branch-name.txt

git add .

if git diff --cached --quiet; then
  echo "Nothing staged to commit." >&2
  exit 1
fi

# Refuse to commit directly to main/master — create a feature branch first.
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" = "main" ] || [ "$CURRENT_BRANCH" = "master" ]; then
  BRANCH_PROMPT='Generate a short git branch name for the staged changes shown below. Rules: lowercase kebab-case; start with a type prefix (feat/, fix/, chore/, docs/, refactor/); max 60 chars total; no trailing slash; no quotes; no explanation. Output the branch name only on a single line.'

  {
    echo "=== Staged changes summary ==="
    git diff --cached --stat
    echo ""
    echo "=== Staged diff ==="
    git diff --cached
  } | CLAUDE_HISTORY_ROLE=branch-name claude -p "$BRANCH_PROMPT" | tr -d '\r' | sed -e '/^```[a-zA-Z]*$/d' -e '/^```$/d' -e '/^$/d' | head -n 1 > "$BRANCH_NAME_FILE"

  NEW_BRANCH=$(cat "$BRANCH_NAME_FILE")
  rm "$BRANCH_NAME_FILE"

  if [ -z "$NEW_BRANCH" ]; then
    echo "Claude returned an empty branch name." >&2
    exit 1
  fi

  echo "On $CURRENT_BRANCH — creating branch '$NEW_BRANCH' for the commit." >&2
  git checkout -b "$NEW_BRANCH"
fi

PROMPT='Generate a git commit message for the staged changes shown below. Format: title on line 1 (max 120 chars; use a conventional prefix like feat:/fix:/chore:/docs:/refactor: when the recent commits use them, otherwise plain imperative); blank line on line 2; then a bullet list of the changes (lines starting with "- ", no blank lines between bullets). No markdown code fences, no Co-authored-by, no Generated-with trailers, no surrounding quotes, no explanation. Output the message only.'

{
  echo "=== Recent commits (style reference) ==="
  git log -5 --pretty=format:"%s%n%n%b%n---"
  echo ""
  echo "=== Staged changes summary ==="
  git diff --cached --stat
  echo ""
  echo "=== Staged diff ==="
  git diff --cached
} | CLAUDE_HISTORY_ROLE=commit-msg claude -p "$PROMPT" | sed -e '/^```[a-zA-Z]*$/d' -e '/^```$/d' > "$COMMIT_MSG_FILE"

if [ ! -s "$COMMIT_MSG_FILE" ]; then
  echo "Claude returned an empty commit message." >&2
  exit 1
fi

git commit -F "$COMMIT_MSG_FILE"
git push origin HEAD
rm "$COMMIT_MSG_FILE"
