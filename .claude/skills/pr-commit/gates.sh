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

# The bdd-tagged tree is invisible to `go vet ./...` and to
# golangci-lint, which sets no build tags — so the suite shims, the
# TestMain bootstraps and every generated test file would go unchecked.
# That gap is not theoretical: vet is what catches a generated function
# named `Testfoo`, which compiles, is never run by `go test`, and still
# satisfies every coverage check.
go vet -tags bdd ./tests/...

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

# The web suite's guards, and only its guards. The suite itself needs
# node, a Next.js build and a browser and stays manual — but "does every
# bdd-web scenario have a generated test?" needs none of those and
# answers in milliseconds. Without this the web half of the registry
# could go stale for weeks and the only thing that would notice is a
# deliberate run nobody makes.
#
# TestStepCoverage is deliberately NOT here yet: tests/bdd-web/steps
# registers four definitions against 244 scenarios, so it reports ~1680
# unbound steps and would make this gate unsatisfiable rather than
# failing — blocking every PR in the repo, not just the port's own. Add it
# back to this line, and to the CI step that mirrors it, in the change
# that lands the web step definitions:
#
#   https://app.clickup.com/t/86cb6fjwy
#
# The question is not lost meanwhile: `build tests` asks it through the
# suite's `commands.coverage`, and one command still answers it —
#   go test -tags bdd -count=1 -run '^TestStepCoverage$' ./tests/bdd-web/
go test -tags bdd -count=1 -run '^TestScenarioCoverage$' ./tests/bdd-web/
