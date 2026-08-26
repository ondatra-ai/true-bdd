#!/usr/bin/env bash
# Usage: timings.sh start | add <step> <seconds> | render
#
# The commit run's stopwatch. An append-only TSV under ./tmp that every step
# writes one `step<TAB>seconds` row to, and `render` prints as a table. A file
# rather than a variable because the steps are separate processes — three of
# them are skill invocations the agent makes, not commands a shell wraps.
#
# `start` truncates, so it belongs in the skill once per invocation and never
# inside a step: gates.sh alone runs again for every fix round of a merge, and
# a reset there would erase the rows written before it.
#
# scripts/merge reads the same file to size the whole task-handling process,
# so the format is a contract: two tab-separated fields, seconds as an integer.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
LEDGER="$ROOT/tmp/timings.tsv"

mkdir -p "$ROOT/tmp"

case "${1:-}" in
  start)
    : > "$LEDGER"
    ;;
  add)
    printf '%s\t%s\n' "${2:?step name}" "${3:?elapsed seconds}" >> "$LEDGER"
    ;;
  render)
    if [ ! -s "$LEDGER" ]; then
      echo "no step timings were recorded."
      exit 0
    fi
    awk -F'\t' '
      BEGIN { print ""; print "| Step | Seconds |"; print "| --- | --- |" }
      { printf "| %s | %s |\n", $1, $2; total += $2 }
      END { printf "| **total** | **%d** |\n\n", total }
    ' "$LEDGER"
    ;;
  *)
    echo "usage: timings.sh start | add <step> <seconds> | render" >&2
    exit 1
    ;;
esac
