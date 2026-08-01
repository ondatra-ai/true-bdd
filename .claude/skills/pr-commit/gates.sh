#!/usr/bin/env bash
# Usage: gates.sh
# Run the true-bdd quality pipeline before the commit step.
# Aborts on any failure. Mirrors the commands the README documents
# for gating production-ready code.
set -euo pipefail

golangci-lint run
mkdir -p ./bin
go build -o ./bin/true-bdd ./src
go test ./...
# The end-to-end BDD fixture suite (real Claude calls, ~30-90 min) is
# deliberately NOT part of the commit gate. Run it manually with:
#   go test -tags bdd -timeout=180m ./tests/bdd-cli/...
