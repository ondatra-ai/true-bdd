#!/usr/bin/env bash
# Usage: commit.sh
# Stages all changes, asks Claude (via `claude -p`) to generate the commit
# message from the staged diff, commits, pushes, then chains to pr-update.sh.
set -euo pipefail

if ! command -v claude >/dev/null 2>&1; then
  echo "claude CLI not found in PATH." >&2
  exit 1
fi

# Bounds the staged diff piped to `claude -p` and names the failure when the
# call still fails — the same trap pr-update.sh fell into on PR #70.
# shellcheck source=../lib/diff-context.sh
source "$(dirname "${BASH_SOURCE[0]}")/../lib/diff-context.sh"

mkdir -p ./tmp
COMMIT_MSG_FILE=./tmp/commit-msg.txt
BRANCH_NAME_FILE=./tmp/branch-name.txt

# The context file is the staged diff, up to DIFF_BUDGET_BYTES of it. On a
# failure `set -e` aborts before any explicit cleanup, so the trap owns it.
# COMMIT_MSG_FILE is NOT in here: `git commit -F` still needs it.
CONTEXT_FILE=./tmp/staged-context.txt
trap 'rm -f "$CONTEXT_FILE" "$BRANCH_NAME_FILE"' EXIT

git add .

if git diff --cached --quiet; then
  echo "Nothing staged to commit." >&2
  exit 1
fi

# Refuse to commit directly to main/master — create a feature branch first.
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" = "main" ] || [ "$CURRENT_BRANCH" = "master" ]; then
  BRANCH_PROMPT='Generate a short git branch name for the staged changes shown below. Rules: lowercase kebab-case; start with a type prefix (feat/, fix/, chore/, docs/, refactor/); max 60 chars total; no trailing slash; no quotes; no explanation. Output the branch name only on a single line.'

  emit_diff_context --cached > "$CONTEXT_FILE"
  run_claude_or_explain branch-name "$BRANCH_PROMPT" "$BRANCH_NAME_FILE" < "$CONTEXT_FILE"

  # One awk over the file, no pipeline: `… | head -n 1` makes the upstream
  # stage die of SIGPIPE, and under `set -o pipefail` a command
  # substitution returning 141 aborts the script with nothing printed.
  NEW_BRANCH=$(awk '{gsub(/\r/, "")} !/^```[a-zA-Z]*$/ && NF {print; exit}' "$BRANCH_NAME_FILE")
  rm "$BRANCH_NAME_FILE"

  if [ -z "$NEW_BRANCH" ]; then
    echo "Claude returned an empty branch name." >&2
    exit 1
  fi
  # A generated name starting with '-' would be read by git as a flag
  # rather than as a ref, and quoting does not help with that.
  case "$NEW_BRANCH" in
    -*) echo "Refusing branch name '$NEW_BRANCH': git would read it as a flag." >&2; exit 1 ;;
  esac

  echo "On $CURRENT_BRANCH — creating branch '$NEW_BRANCH' for the commit." >&2
  git checkout -b "$NEW_BRANCH"
fi

PROMPT='Generate a git commit message for the staged changes shown below. Format: title on line 1 (max 120 chars; use a conventional prefix like feat:/fix:/chore:/docs:/refactor: when the recent commits use them, otherwise plain imperative); blank line on line 2; then a bullet list of the changes (lines starting with "- ", no blank lines between bullets). No markdown code fences, no Co-authored-by, no Generated-with trailers, no surrounding quotes, no explanation. Output the message only.'

{
  echo "=== Recent commits (style reference) ==="
  git log -5 --pretty=format:"%s%n%n%b%n---"
  echo ""
  emit_diff_context --cached
} > "$CONTEXT_FILE"

run_claude_or_explain commit-msg "$PROMPT" "$COMMIT_MSG_FILE" < "$CONTEXT_FILE"
rm -f "$CONTEXT_FILE"
sed -i.bak -e '/^```[a-zA-Z]*$/d' -e '/^```$/d' "$COMMIT_MSG_FILE" && rm -f "$COMMIT_MSG_FILE.bak"

git commit -F "$COMMIT_MSG_FILE"
git push origin HEAD
rm "$COMMIT_MSG_FILE"
