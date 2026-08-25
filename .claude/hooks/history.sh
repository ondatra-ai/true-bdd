#!/usr/bin/env bash
# Usage: history.sh prompt-submit | new-task | bind <id> | bound | unbind
# The entry point .claude/settings.json wires to UserPromptSubmit and Stop.
#
# A shim, not the tool: the Go source lives under scripts/, because the Go tool
# skips any directory whose name begins with a dot and a package under
# .claude/ would be invisible to `go build ./...`, `go test ./...` and
# golangci-lint.
#
# Two separate questions, deliberately not the same variable:
#
#   HERE  — where the Go module is, resolved from this script's own path,
#           because that is the only thing that is always true of it.
#   CLAUDE_PROJECT_DIR — which repository to log. Claude Code sets it when it
#           invokes a hook; the `!`-invoked /task-start skill gets no hook
#           environment, so it falls back to HERE. Exported either way: a
#           `go run` binary lives in a temporary directory and cannot find the
#           repository from its own path the way the Python found it from
#           __file__.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export CLAUDE_PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$HERE}"

exec go -C "$HERE" run ./scripts/cmd/history "$@"
