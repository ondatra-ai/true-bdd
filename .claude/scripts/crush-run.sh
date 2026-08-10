#!/usr/bin/env bash
# Single crush-invocation wrapper (author/fixer sandbox roles).
#
# Usage: crush-run.sh <author|fixer> <prompt-file|-> [label] [--continue]
#
#   role         author | fixer — exported as CRUSH_GUARD_ROLE, selecting the
#                write sandbox enforced by .crush/hooks/guard.py.
#   prompt-file  Path to the crush prompt, or '-' to read it from stdin.
#   label        Artifact basename (default: the role). Prompt is preserved at
#                tmp/crush/<label>.prompt.md, transcript at tmp/crush/<label>.out.
#   --continue   Resume crush's most recent session (follow-up turns). Bound
#                positionally as the 4th arg, so pass an explicit label with it.
#
# Config: crush's project config is generated here per invocation (model pin,
# read-only allowed_tools, the write-guard hook, MCP servers) and handed over
# through CRUSH_GLOBAL_CONFIG, then deleted on exit. It must NOT be a tracked
# file at the repo root — crush merges every .crush.json it finds walking up
# from its cwd, so a root config leaks into every nested crush process,
# including the ones true-bdd spawns against fixture workspaces under tmp/.
#
# Stall handling: crush's embedded shell pipe-deadlocks on chatty child output,
# so a hung run must be killed, not waited out. After CRUSH_TIMEOUT seconds
# (default 1800 — a long author/fixer round with one self-correction takes
# ~20-25 min) the whole crush process tree is killed and we exit 124 so the
# driver can tell a stall from a model failure.
set -u

# Repo root, location-independent: prefer git's toplevel, fall back to two
# levels up (.claude/scripts -> repo root).
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

# Crush config, generated per invocation and discarded on exit.
#
# This deliberately does NOT live at the repo root. crush resolves its
# project config by walking UP from its cwd and MERGES every .crush.json
# it finds, with PreToolUse hooks additive and un-overridable from a
# nested config. A tracked root .crush.json therefore hijacks every crush
# process started anywhere inside the repo — including the ones the
# true-bdd engine spawns for its own apply turns against fixture
# workspaces under tmp/, where the relative guard.py path does not
# resolve and every tool call is blocked. Generating the config here and
# passing it through CRUSH_GLOBAL_CONFIG keeps it scoped to this wrapper.
CONFIG_DIR="$OUT_DIR/config-$$"
mkdir -p "$CONFIG_DIR"
cleanup_config() { rm -rf "$CONFIG_DIR"; }
trap cleanup_config EXIT

# The guard path is absolute so the hook works regardless of cwd, and
# data_directory pins crush's SQLite state to its historical location so
# `--continue` keeps resuming the same sessions.
cat > "$CONFIG_DIR/crush.json" <<EOF
{
  "\$schema": "https://charm.land/crush.json",
  "options": {
    "data_directory": "$REPO/.crush"
  },
  "models": {
    "large": {
      "model": "glm-5.2",
      "provider": "zhipu-coding"
    }
  },
  "permissions": {
    "allowed_tools": ["view", "ls", "grep", "glob"]
  },
  "hooks": {
    "PreToolUse": [
      {
        "name": "harness-write-guard",
        "command": "python3 '$REPO/.crush/hooks/guard.py'",
        "timeout": 15
      }
    ]
  },
  "mcp": {
    "context7": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp@3.2.5"]
    },
    "terminal": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "mcp-interactive-terminal@1.0.9"]
    },
    "playwright": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@playwright/mcp@0.0.78"]
    }
  }
}
EOF

export CRUSH_GLOBAL_CONFIG="$CONFIG_DIR"
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
