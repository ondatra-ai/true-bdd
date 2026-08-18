#!/usr/bin/env bash
# Review helper for the pr-merge skill.
#
#   threads.sh list [pr] [--all]     every finding on the PR, reconciled
#   threads.sh status [pr]           what is blocking, plus stale-verdict evidence
#   threads.sh await-review [s] [pr] wait, bounded, for a NEW review object
#   threads.sh preflight [pr]        can this merge? names the missing precondition
#   threads.sh reply <id> <text>     reply to one review comment
#   threads.sh resolve <thread>      resolve ONE thread by its node id
#
# `list` covers BOTH classes of finding, because CodeRabbit posts in two
# places and only one of them is a thread: the "Actionable comments" become
# review threads, while "Nitpick / Outside diff range / Duplicate comments"
# live only inside the review body and have no reply target at all. Querying
# reviewThreads alone showed 16 of the 28 findings on PR #70. The rendering
# and the reconciliation both live in render_review.py.
#
# There is deliberately no "resolve everything" verb. Resolving a thread is
# a claim that it was read and answered; a bulk resolve makes that claim
# about comments nobody looked at, and CodeRabbit posts a fresh review on
# every push, so the list grows while you work.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RENDER="$HERE/render_review.py"

REPO=$(gh repo view --json nameWithOwner --jq .nameWithOwner)

# Resolve the PR: an explicit numeric argument wins, otherwise the current
# branch's. Kept out of render_review.py's way so both agree.
resolve_pr() {
  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    echo "$1"
  else
    gh pr view --json number --jq .number
  fi
}

case "${1:-}" in
list)
  shift
  ARGS=()
  for arg in "$@"; do
    if [[ "$arg" =~ ^[0-9]+$ ]]; then
      ARGS+=(--pr "$arg")
    else
      ARGS+=("$arg")
    fi
  done
  exec python3 "$RENDER" list ${ARGS[@]+"${ARGS[@]}"}
  ;;

status)
  shift
  ARGS=()
  if [[ "${1:-}" =~ ^[0-9]+$ ]]; then ARGS+=(--pr "$1"); fi
  exec python3 "$RENDER" status ${ARGS[@]+"${ARGS[@]}"}
  ;;

await-review)
  # A bot acknowledgement is not a bot verdict. On PR #70 CodeRabbit
  # answered "I will perform a fresh review" and never posted a review
  # object; the session waited 4h37m because nothing bounded it. This
  # bounds it and reports the timeout as its own exit code, so the skill
  # can tell "a new verdict arrived" from "none ever will".
  #
  # Run this with run_in_background: true — the sleep is not something to
  # block a foreground turn on.
  shift
  BUDGET="${1:-900}"
  if ! [[ "$BUDGET" =~ ^[0-9]+$ ]]; then
    echo "await-review: first argument must be a number of seconds" >&2
    exit 2
  fi
  PR=$(resolve_pr "${2:-}")

  # `gh api --paginate` applies --jq to EACH page and concatenates the
  # results, so `[.[].id] | max` emits one number per page. Two pages
  # would make the numeric comparison below fail outright. Emit the ids
  # and take the maximum here instead.
  latest_review_id() {
    gh api --paginate "repos/$REPO/pulls/$PR/reviews?per_page=100" --jq '.[].id' \
      | sort -n | tail -1
  }

  BASELINE=$(latest_review_id); BASELINE=${BASELINE:-0}
  echo "await-review: PR #$PR, baseline review id $BASELINE, budget ${BUDGET}s" >&2

  ELAPSED=0
  while [ "$ELAPSED" -lt "$BUDGET" ]; do
    STEP=30
    if [ $((BUDGET - ELAPSED)) -lt "$STEP" ]; then STEP=$((BUDGET - ELAPSED)); fi
    sleep "$STEP"
    ELAPSED=$((ELAPSED + STEP))

    LATEST=$(latest_review_id); LATEST=${LATEST:-0}
    if [ "$LATEST" -gt "$BASELINE" ]; then
      echo "await-review: new review after ${ELAPSED}s" >&2
      gh api --paginate "repos/$REPO/pulls/$PR/reviews?per_page=100" \
        --jq "[.[] | select(.id > $BASELINE)] | .[] | \"review \(.id) \(.state) by \(.user.login) at \(.submitted_at)\""
      exit 0
    fi
  done

  echo "await-review: no review posted in ${BUDGET}s — the bot acknowledged but did not deliver." >&2
  echo "Do NOT dismiss on your own. Run 'threads.sh status $PR' and ask the user." >&2
  exit 3
  ;;

preflight)
  # Named refusals, so merge.sh never surfaces GitHub's generic one.
  shift
  PR=$(resolve_pr "${1:-}")
  VIEW=$(gh pr view "$PR" --json reviewDecision,mergeStateStatus,statusCheckRollup)
  THREADS_JSON=$(gh api graphql -f query='
    query($owner:String!,$repo:String!,$pr:Int!){
      repository(owner:$owner,name:$repo){ pullRequest(number:$pr){
        reviewThreads(first:100){ nodes{ isResolved } } } } }' \
    -F owner="${REPO%%/*}" -F repo="${REPO##*/}" -F pr="$PR" \
    --jq '.data.repository.pullRequest.reviewThreads.nodes')
  UNRESOLVED=$(echo "$THREADS_JSON" | jq '[.[] | select(.isResolved | not)] | length')
  TOTAL_THREADS=$(echo "$THREADS_JSON" | jq 'length')

  BLOCKED=0
  if [ "${UNRESOLVED:-0}" -gt 0 ]; then
    echo "Cannot merge: $UNRESOLVED review thread(s) still unresolved (main requires conversation resolution)." >&2
    BLOCKED=1
  elif [ "${TOTAL_THREADS:-0}" -ge 100 ]; then
    # The query asks for first:100. A full page means there may be more,
    # and reporting "0 unresolved" from a truncated page is precisely the
    # silent under-report this whole helper exists to prevent.
    echo "Cannot merge: the thread page is full ($TOTAL_THREADS) — there may be more beyond it." >&2
    echo "  Resolve them on GitHub, or paginate this query, before trusting a zero here." >&2
    BLOCKED=1
  fi
  if [ "$(echo "$VIEW" | jq -r '.reviewDecision // ""')" != "APPROVED" ]; then
    echo "Cannot merge: no approving review (main requires 1). reviewDecision=$(echo "$VIEW" | jq -r '.reviewDecision // "none"')" >&2
    BLOCKED=1
  fi
  # Which checks are required is asked of the branch, not hardcoded — the
  # ruleset is free to change and a stale list here would report "ok" for
  # a check nobody ran. `gates` is a check run (carries `conclusion`);
  # `CodeRabbit` is a commit status (carries `state`, `conclusion` null),
  # so both fields are read.
  REQUIRED=$(gh api "repos/$REPO/rules/branches/$(gh repo view --json defaultBranchRef --jq .defaultBranchRef.name)" \
    --jq '[.[] | select(.type=="required_status_checks") | .parameters.required_status_checks[].context] | .[]' 2>/dev/null || true)
  if [ -z "$REQUIRED" ]; then
    echo "Warning: could not read the branch's required checks — falling back to 'gates'." >&2
    REQUIRED=gates
  fi
  while IFS= read -r context; do
    [ -n "$context" ] || continue
    STATE=$(echo "$VIEW" | jq -r --arg c "$context" \
      '[.statusCheckRollup[]? | select((.name // .context) == $c)] | .[0] | (.conclusion // .state // "MISSING") | ascii_upcase')
    if [ "$STATE" != "SUCCESS" ]; then
      echo "Cannot merge: required check '$context' is $STATE." >&2
      BLOCKED=1
    fi
  done <<< "$REQUIRED"
  [ "$BLOCKED" -eq 0 ] && echo "preflight: ok — approved, threads resolved, gates green."
  exit "$BLOCKED"
  ;;

reply)
  gh api -X POST "repos/$REPO/pulls/$(gh pr view --json number --jq .number)/comments/$2/replies" \
    -f body="$3" --jq '.id' >/dev/null
  echo "replied to comment $2"
  ;;

resolve)
  gh api graphql -f query='
    mutation($id:ID!){ resolveReviewThread(input:{threadId:$id}){ thread{ isResolved } } }' \
    -F id="$2" --jq '.data.resolveReviewThread.thread.isResolved'
  ;;

*)
  sed -n '2,16p' "${BASH_SOURCE[0]}" >&2
  exit 2
  ;;
esac
