#!/usr/bin/env bash
# Usage: close-task.sh <STATUS> <comment...>
# Invoked by /task-done and /task-fail. The whole terminal transition, so that
# neither skill has to get the order right in prose.
#
# Order is the point: unbind runs LAST. A status write that fails with the
# binding already gone would leave a Ticket stuck in PROCESSING that nothing
# can close, so `set -e` aborts before the unbind rather than after it.
#
# The Ticket's body is never touched — `clickup status` says so in its prompt.
set -euo pipefail

ROOT="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"
HOOK="$ROOT/.claude/hooks/history.sh"

if [ $# -lt 2 ]; then
  echo "usage: close-task.sh <STATUS> <comment...>" >&2
  exit 1
fi

STATUS="$1"
shift

TICKET="$("$HOOK" bound)"

if [ -z "$TICKET" ]; then
  echo "No Ticket is bound to this Task — nothing to close." >&2
  exit 1
fi

go -C "$ROOT" run ./scripts/cmd/clickup status "$TICKET" "$STATUS" "$*"

"$HOOK" unbind

echo "closed $TICKET as $STATUS; binding cleared"
