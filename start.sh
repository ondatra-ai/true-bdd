#!/usr/bin/env bash
# Start Claude Code with this repository's secrets loaded.
#
# The merge tooling reads CODERABBIT_API_KEY (and CLICKUP_API_TOKEN, when
# set) from the environment rather than from a config file, so they have to
# be exported before the session starts — a key sourced inside a subshell
# during the session does not reach the scripts the agent runs.
set -euo pipefail
cd "$(dirname "$0")"

if [[ ! -f .env ]]; then
  echo "Missing .env — copy .env.example and fill in values." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

exec claude --dangerously-skip-permissions "$@"
