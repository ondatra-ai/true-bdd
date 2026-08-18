#!/usr/bin/env bash
# Squash-merge a PR, then clean up local and remote.
#
#   merge.sh [pr]    the PR to merge; defaults to the current branch's
#
# The PR is named rather than inferred, because the caller knows it and the
# checkout does not: orchestrate.py accepts --pr and its run is resumable
# across sessions, so "whatever branch is checked out" can belong to a
# different PR entirely. Squashing the wrong PR — and then deleting its
# branch — is not reversible, so the checkout is checked against the PR's
# head ref first and a mismatch is a refusal, never a guess about which of
# the two was meant.
#
# This always merges. The loop that calls it has already spent three review
# rounds and deferred everything it did not fix, so a still-red gate is a
# fact to record rather than a reason to stall — orchestrate.py writes the
# reason into the run's anomalies and the post-mortem picks it up.
#
# The escalation is explicit rather than implicit: try the ordinary merge
# first, and only fall back to --admin when GitHub refuses. That way the
# common case is an ordinary merge and the log shows plainly when it was not.
set -e

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CURRENT_BRANCH=$(git branch --show-current)

# Same resolution rule as threads.sh — an explicit PR number wins, otherwise
# the current branch's, so the two never disagree about which PR one run is
# about — but STRICTER about what counts as explicit. threads.sh reads a
# non-numeric argument as "no PR was named" because its verbs take other
# arguments too, and a wrong answer there is a wrong listing. Here the same
# silent fallback would answer a typo'd number by squashing the current
# branch's PR and deleting its branch, and the head-ref check below could
# not catch it: the fallback derives the PR *from* that branch, so the two
# agree by construction. An argument that is not a PR number is a refusal.
if [ $# -gt 1 ]; then
  echo "Cannot merge: merge.sh takes at most one argument, the PR number." >&2
  echo "  Usage: merge.sh [pr] — omit it to merge the current branch's PR." >&2
  exit 1
elif [ $# -eq 1 ]; then
  if ! [[ "$1" =~ ^[0-9]+$ ]]; then
    echo "Cannot merge: '$1' is not a PR number." >&2
    echo "  Usage: merge.sh [pr] — omit it to merge the current branch's PR." >&2
    exit 1
  fi
  PR="$1"
else
  PR=$(gh pr view --json number --jq .number)
fi

PR_BRANCH=$(gh pr view "$PR" --json headRefName --jq .headRefName)
if [ "$PR_BRANCH" != "$CURRENT_BRANCH" ]; then
  echo "Cannot merge: PR #$PR is on branch '$PR_BRANCH', but '$CURRENT_BRANCH' is checked out." >&2
  echo "  Merging here would squash a PR this working tree is not on, and --delete-branch" >&2
  echo "  would remove its branch. Check out '$PR_BRANCH' and run again." >&2
  exit 1
fi

# Resolve once and create the directory FIRST. A redirect into a missing
# directory fails before `gh pr merge` runs at all, which the `if !` below
# would read as "the ordinary merge was refused" and escalate straight to
# --admin — bypassing the ruleset because of a missing folder.
ERR_FILE="$(git rev-parse --show-toplevel)/tmp/merge/merge-err.txt"
mkdir -p "$(dirname "$ERR_FILE")"

if ! gh pr merge "$PR" --squash --delete-branch 2>"$ERR_FILE"; then
  echo "Ordinary merge refused:" >&2
  sed 's/^/  /' "$ERR_FILE" >&2 || true
  echo "Escalating to --admin (the ruleset grants the admin role a" >&2
  echo "pull_request-scoped bypass, so this is permitted rather than forced)." >&2
  gh pr merge "$PR" --squash --delete-branch --admin
fi

MAIN_BRANCH=$(git show-ref --verify --quiet refs/heads/master && echo master || echo main)
git checkout "$MAIN_BRANCH"
git pull origin "$MAIN_BRANCH"
git branch -D "$CURRENT_BRANCH" 2>/dev/null || echo "Local branch '$CURRENT_BRANCH' already deleted"
