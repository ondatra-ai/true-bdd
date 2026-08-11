#!/usr/bin/env bash
# Review-thread helper for the pr-merge skill.
#
#   threads.sh list              unresolved threads, full bodies, with ids
#   threads.sh reply <id> <text> reply to one review comment
#   threads.sh resolve <thread>  resolve ONE thread by its node id
#   threads.sh status            what is currently blocking the merge
#
# There is deliberately no "resolve everything" verb. Resolving a thread
# is a claim that it was read and answered; a bulk resolve makes that
# claim about comments nobody looked at, and CodeRabbit posts a fresh
# review on every push, so the list grows while you work.
set -euo pipefail

REPO=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
PR=$(gh pr view --json number --jq .number)

case "${1:-}" in
list)
  # Thread node ids (GraphQL) joined to comment ids (REST): replying
  # needs the comment id, resolving needs the thread id.
  gh api graphql -f query='
    query($owner:String!,$repo:String!,$pr:Int!){
      repository(owner:$owner,name:$repo){
        pullRequest(number:$pr){
          reviewThreads(first:100){
            nodes{
              id isResolved isOutdated path line
              comments(first:20){ nodes{ databaseId author{login} body } }
            }
          }
        }
      }
    }' \
    -F owner="${REPO%%/*}" -F repo="${REPO##*/}" -F pr="$PR" \
    --jq '.data.repository.pullRequest.reviewThreads.nodes[]
          | select(.isResolved | not)
          | "════════════════════════════════════════════════
THREAD : \(.id)
FILE   : \(.path):\(.line // "?")\(if .isOutdated then "   [OUTDATED — line has since moved]" else "" end)
\(.comments.nodes[0].author.login) (comment \(.comments.nodes[0].databaseId)):
\(.comments.nodes[0].body | split("<details>")[0])
\(if (.comments.nodes | length) > 1 then "--- \((.comments.nodes | length) - 1) reply(ies) already on this thread ---" else "" end)"'
  ;;

reply)
  gh api -X POST "repos/$REPO/pulls/$PR/comments/$2/replies" -f body="$3" --jq '.id' >/dev/null
  echo "replied to comment $2"
  ;;

resolve)
  gh api graphql -f query='
    mutation($id:ID!){ resolveReviewThread(input:{threadId:$id}){ thread{ isResolved } } }' \
    -F id="$2" --jq '"thread \($2) resolved=\(.data.resolveReviewThread.thread.isResolved)"' 2>/dev/null \
    || gh api graphql -f query='
      mutation($id:ID!){ resolveReviewThread(input:{threadId:$id}){ thread{ isResolved } } }' \
      -F id="$2" --jq '.data.resolveReviewThread.thread.isResolved'
  ;;

status)
  gh pr view --json mergeable,mergeStateStatus,reviewDecision,statusCheckRollup \
    --jq '{mergeable, state: .mergeStateStatus, review: .reviewDecision,
           checks: [.statusCheckRollup[] | {name: (.name // .context),
                                            status: (.status // .state),
                                            conclusion}]}'
  ;;

*)
  echo "usage: threads.sh list|reply <comment-id> <text>|resolve <thread-id>|status" >&2
  exit 2
  ;;
esac
