#!/usr/bin/env bash
# Usage: pr-update.sh
# Asks Claude (via `claude -p`) to generate a PR title and body from the
# branch's commits + diff vs base, then creates a new PR or edits the existing one.
set -euo pipefail

if ! command -v claude >/dev/null 2>&1; then
  echo "claude CLI not found in PATH." >&2
  exit 1
fi

mkdir -p ./tmp
PR_OUTPUT_FILE=./tmp/pr-output.txt
PR_BODY_FILE=./tmp/pr-body.md

BASE_BRANCH="main"
if ! git show-ref --verify --quiet refs/remotes/origin/main; then
  if git show-ref --verify --quiet refs/remotes/origin/master; then
    BASE_BRANCH="master"
  fi
fi

PROMPT='Read the branch context below and output a pull request TITLE and BODY in this exact format:
LINE 1: the title (max 120 chars, conventional commit prefix like feat:/fix:/chore:/docs:/refactor:)
LINE 2: blank
LINE 3 onward: the body — bullet list of changes (lines starting with "- ", no blank lines between bullets), then a blank line, then a brief why-paragraph.

No markdown code fences, no Co-authored-by, no Generated-with trailers, no surrounding quotes, no introduction. Output only the title and body.'

{
  echo "=== Commits on this branch (vs origin/'"$BASE_BRANCH"') ==="
  git log "origin/$BASE_BRANCH"..HEAD --pretty=format:"%s%n%n%b%n---"
  echo ""
  echo "=== Diff stat vs origin/$BASE_BRANCH ==="
  git diff "origin/$BASE_BRANCH"...HEAD --stat
  echo ""
  echo "=== Diff vs origin/$BASE_BRANCH ==="
  git diff "origin/$BASE_BRANCH"...HEAD
} | CLAUDE_HISTORY_ROLE=pr-content claude -p "$PROMPT" | sed -e '/^```[a-zA-Z]*$/d' -e '/^```$/d' > "$PR_OUTPUT_FILE"

if [ ! -s "$PR_OUTPUT_FILE" ]; then
  echo "Claude returned an empty PR title/body." >&2
  exit 1
fi

PR_TITLE=$(head -n 1 "$PR_OUTPUT_FILE")
tail -n +3 "$PR_OUTPUT_FILE" > "$PR_BODY_FILE"
rm "$PR_OUTPUT_FILE"

if [ -z "$PR_TITLE" ] || [ ! -s "$PR_BODY_FILE" ]; then
  echo "Parsed empty PR title or body from Claude output." >&2
  exit 1
fi

if gh pr view --json number >/dev/null 2>&1; then
  gh pr edit --title "$PR_TITLE" --body-file "$PR_BODY_FILE" >/dev/null
else
  gh pr create --title "$PR_TITLE" --body-file "$PR_BODY_FILE" >/dev/null
fi

rm "$PR_BODY_FILE"
gh pr view --json url -q .url
