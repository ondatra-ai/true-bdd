#!/usr/bin/env bash
# Usage: gates.sh
# Run the true-bdd quality pipeline before the commit step.
# Aborts on any failure. Mirrors the commands the README documents
# for gating production-ready code.
set -euo pipefail

./scripts/lint-layout.sh
./scripts/lint-comments.sh
./scripts/lint-schemas.sh
./scripts/lint-claude.md.sh
./scripts/lint-markdown.sh
golangci-lint run
mkdir -p ./bin
go build -o ./bin/true-bdd ./services/bdd-cli
go test ./...

# The bdd-tagged tree is invisible to `go vet ./...`/golangci-lint (no
# build tags set) — so shims, TestMain and generated tests go unchecked.
# Not theoretical: vet catches a generated `Testfoo` that compiles, never runs, and still passes coverage.
go vet -tags bdd ./tests/...

# BDD fixtures, replayed — cassette-served: no model, no cost, <1 min.
# Only check that runs the engine: caught a template silently rewriting
# data, and a fenced-YAML crash mid-fix — both missed by unit tests.
#	exit 86            stale cassette — re-record the fixtures
#	golden mismatch    regression signal — read the diff first
#	ordinary failure   exit code or stdout assertion
#	live suite (real models, ~30-90 min, $): go test -tags bdd -timeout=180m ./tests/bdd-cli/...
go test -tags bdd -timeout=20m ./tests/bdd-cli/ -mode=replay

# Web suite guards only (no node/browser/build; answers in ms) — the
# suite itself stays manual. TestStepCoverage stays OUT: only 4 of 244
# scenarios' steps bind — adding it would make this gate UNSATISFIABLE, not just fail it.
#	~1680 unbound steps across those 243 would block EVERY PR, not just
#	the port's. Land it with the web step definitions: ClickUp 86cb6fjwy.
#	Meanwhile: go test -tags bdd -count=1 -run '^TestStepCoverage$' ./tests/bdd-web/
go test -tags bdd -count=1 -run '^TestScenarioCoverage$' ./tests/bdd-web/
