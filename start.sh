#!/usr/bin/env bash
# Start Claude Code with this repository's secrets loaded.
#
# Keys are read from the environment rather than from a config file, so
# .env has to be exported before the session starts — a key sourced inside a
# subshell during the session does not reach the scripts the agent runs.
set -euo pipefail
cd "$(dirname "$0")"

# Every commit runs the gates its staged change needs (.githooks/pre-commit).
(
  for name in $(git rev-parse --local-env-vars); do unset "$name"; done
  git config core.hooksPath .githooks
)

if [[ ! -f .env ]]; then
  echo "Missing .env — copy .env.example and fill in values." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

exec claude --dangerously-skip-permissions "$@"
