#!/usr/bin/env bash
# Usage: gates.sh
# Run the true-bdd quality pipeline before the commit step.
# Aborts on any failure. Mirrors the commands the README documents
# for gating production-ready code.
set -euo pipefail

./scripts/validate-schemas.sh
golangci-lint run
mkdir -p ./bin
go build -o ./bin/true-bdd ./services/bdd-cli
go test ./...

# The BDD fixtures, replayed. Every AI turn is served from a committed
# cassette, so this spawns no model, costs nothing, needs no agent CLI
# installed, and finishes in under a minute — the three reasons the
# suite used to be excluded from this gate no longer hold.
#
# It is here because it is the only check that sees the engine actually
# run. Unit tests pass while a prompt template silently rewrites data it
# was asked to preserve, and while an applier's fenced YAML crashes a
# command mid-fix; replay caught both, one as a golden mismatch and one
# as a non-zero exit.
#
# A failure means one of three things, and the output says which:
#   stale cassette (exit 86) — a prompt/template/engine change altered a
#     request. Re-record the affected fixtures.
#   golden mismatch — the run produced different files than recorded.
#     Read the diff before re-recording; this is the regression signal.
#   ordinary failure — exit code or stdout assertion.
#
# The live suite (real models, ~30-90 min, real money) stays manual:
#   go test -tags bdd -timeout=180m ./tests/bdd-cli/...
go test -tags bdd -timeout=20m ./tests/bdd-cli/ -mode=replay
