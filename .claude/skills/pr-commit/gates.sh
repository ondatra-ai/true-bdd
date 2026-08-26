#!/usr/bin/env bash
# Usage: gates.sh [--changed <base-ref>]
# Run the true-bdd quality pipeline before the commit step. Aborts on the
# first failure.
#
# The pipeline itself is data in scripts/gates — which gates exist, what each
# costs, and which changed paths make one necessary. Read that package's doc
# comment before adding, removing or reordering anything here.
#
# Bare, this runs every gate; that is what CI and every human commit do.
# `--changed main` narrows it to the gates the diff needs, which is how
# task-handle spends ~2s on a documentation ticket instead of ~140s. Selection
# is LOCAL ONLY: CI stays exhaustive, so whatever the selector skips is still
# caught before the merge.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
HERE="$(dirname "${BASH_SOURCE[0]}")"

mkdir -p "$ROOT/bin"

# No `exec`: the row has to be written whichever way the pipeline exits, and a
# red run is the one worth timing. `start` is the skill's, never this script's
# — a merge round runs the gates again and would erase the rows before it.
START=$(date +%s)
set +e
go -C "$ROOT" run ./scripts/cmd/gates run "$@"
CODE=$?
set -e

# `|| true`: an unwritable ./tmp must leave the pipeline silent, not red.
"$HERE/timings.sh" add gates "$(($(date +%s) - START))" || true
exit "$CODE"
