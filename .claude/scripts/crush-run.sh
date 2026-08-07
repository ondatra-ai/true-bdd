#!/usr/bin/env bash
# Crush invocation wrapper for the visual-sweep driver agents.
#
# Usage: crush-run.sh <author|fixer> <prompt-file|-> [label] [--continue]
#
#   role         author | fixer — exported as CRUSH_GUARD_ROLE, selecting the
#                write sandbox enforced by .crush/hooks/guard.py.
#   prompt-file  Path to the crush prompt, or '-' to read it from stdin
#                (drivers have no Write tool; they pipe a quoted heredoc).
#   label        Artifact basename (default: the role). Prompt is preserved at
#                tmp/crush/<label>.prompt.md, transcript at tmp/crush/<label>.out.
#   --continue   Resume crush's most recent session (follow-up turns). Drivers
#                run strictly sequentially, so "most recent" is safe inside a
#                driver's window.
#
# Stall handling: crush's embedded shell pipe-deadlocks on chatty child output,
# so a hung run must be killed, not waited out. After CRUSH_TIMEOUT seconds
# (default 1800 — a measured authoring round with one self-correction takes
# ~20-25 min at GLM-5.2 xhigh) the whole crush process tree is killed and we
# exit 124 so the driver can tell a stall from a model failure.
set -u

# Repo root, location-independent (this script moved out of a skill dir): prefer
# git's toplevel, fall back to two levels up (.claude/scripts -> repo root).
REPO="$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel 2>/dev/null || (cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd))"
USAGE="usage: crush-run.sh <author|fixer> <prompt-file|-> [label] [--continue]"

ROLE="${1:?$USAGE}"
PROMPT_SRC="${2:?$USAGE}"
LABEL="${3:-$ROLE}"
CONT="${4:-}"

case "$ROLE" in
  author|fixer) ;;
  *) echo "crush-run: unknown role '$ROLE'; $USAGE" >&2; exit 64 ;;
esac

OUT_DIR="$REPO/tmp/crush"
mkdir -p "$OUT_DIR"
PROMPT_FILE="$OUT_DIR/$LABEL.prompt.md"
OUT_FILE="$OUT_DIR/$LABEL.out"

if [ "$PROMPT_SRC" = "-" ]; then
  cat > "$PROMPT_FILE"
elif [ -f "$PROMPT_SRC" ]; then
  cp "$PROMPT_SRC" "$PROMPT_FILE"
else
  echo "crush-run: prompt file not found: $PROMPT_SRC" >&2
  exit 66
fi
if [ ! -s "$PROMPT_FILE" ]; then
  echo "crush-run: empty prompt; refusing" >&2
  exit 65
fi

ARGS=(run --quiet)
[ "$CONT" = "--continue" ] && ARGS=(run --quiet -C)

TIMEOUT="${CRUSH_TIMEOUT:-1800}"

export CRUSH_GUARD_ROLE="$ROLE"
( cd "$REPO" && exec crush "${ARGS[@]}" < "$PROMPT_FILE" > "$OUT_FILE" 2>&1 ) &
PID=$!

kill_tree() { # depth-first kill of a process and its descendants
  local sig="$1" pid="$2" child
  for child in $(pgrep -P "$pid" 2>/dev/null); do kill_tree "$sig" "$child"; done
  kill "-$sig" "$pid" 2>/dev/null
}

SECS=0
while kill -0 "$PID" 2>/dev/null; do
  if [ "$SECS" -ge "$TIMEOUT" ]; then
    kill_tree TERM "$PID"
    sleep 3
    kill_tree KILL "$PID"
    echo "crush-run: STALLED — killed the crush tree after ${TIMEOUT}s (role=$ROLE label=$LABEL)." >&2
    echo "crush-run: transcript tail ($OUT_FILE):" >&2
    tail -n 20 "$OUT_FILE" >&2 2>/dev/null
    exit 124
  fi
  sleep 5
  SECS=$((SECS + 5))
done

wait "$PID"
RC=$?
echo "crush-run: exit=$RC role=$ROLE label=$LABEL transcript=$OUT_FILE"
tail -n 40 "$OUT_FILE"
exit "$RC"
