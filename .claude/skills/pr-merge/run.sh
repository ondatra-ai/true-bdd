#!/usr/bin/env bash
# The merge loop — one command, start to finish.
#
#   ./.claude/skills/pr-merge/run.sh [pr]        run (or resume) the loop
#   ./.claude/skills/pr-merge/run.sh [pr] --restart   discard saved state
#
# Everything is orchestrated by orchestrate.py: three review rounds, the
# triage, the fixes, the ClickUp deferrals, the thread answers, pr-commit,
# the approval and the merge. It is resumable — a run takes the better part
# of an hour and has to survive a dropped connection or a rate limit.
#
# Exit codes. There is no code for "did not merge because something went
# wrong" — a failed round, a missing review or a red preflight is recorded
# as an anomaly and the loop continues to the merge regardless.
#   0   merged
#   1   merged, but merge.sh reported a problem
#   2   a required tool is missing from PATH; nothing ran
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# The scripts read CODERABBIT_API_KEY (and CLICKUP_API_TOKEN when set) from
# the environment. Sourcing here rather than relying on the caller's shell
# is what makes the command work the same from a session, a terminal or CI.
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

for tool in gh git python3 claude; do
  command -v "$tool" >/dev/null || { echo "$tool not found in PATH" >&2; exit 2; }
done

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARGS=()
for arg in "$@"; do
  if [[ "$arg" =~ ^[0-9]+$ ]]; then ARGS+=(--pr "$arg"); else ARGS+=("$arg"); fi
done

exec python3 "$HERE/orchestrate.py" ${ARGS[@]+"${ARGS[@]}"}
