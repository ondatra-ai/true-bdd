#!/usr/bin/env bash
# Usage: lint-schemas.sh
# Validates every document that has a schema against it, with yamale.
#
# The pairing is by convention and driven by config, so a new schema needs
# no edit here: `true-bdd/<key>-schema.yaml` validates the document that
# `documents.<key>` in true-bdd/true-bdd.yaml points at. A schema whose
# key names no document is an error — it would otherwise sit unenforced.
set -euo pipefail

CONFIG="true-bdd/true-bdd.yaml"

if ! command -v yamale >/dev/null 2>&1; then
  echo "yamale not found in PATH. Install it with: pip install yamale" >&2
  exit 1
fi

shopt -s nullglob
schemas=(true-bdd/*-schema.yaml)
shopt -u nullglob

if [ ${#schemas[@]} -eq 0 ]; then
  echo "No schemas under true-bdd/ — nothing to validate."
  exit 0
fi

status=0
for schema in "${schemas[@]}"; do
  key=$(basename "$schema" -schema.yaml)

  doc=$(go run ./scripts/cmd/yamlkey "$CONFIG" "documents.$key")

  if [ -z "$doc" ]; then
    echo "FAIL $schema — no documents.$key in $CONFIG, so this schema enforces nothing." >&2
    status=1
    continue
  fi

  if [ ! -f "$doc" ]; then
    # A host may legitimately not carry every document yet; the schema
    # only binds a document that exists.
    echo "SKIP $doc (documents.$key) — not present in this repo."
    continue
  fi

  echo "Validating $doc against $schema"
  yamale -s "$schema" "$doc" || status=1
done

exit $status
