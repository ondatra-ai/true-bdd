#!/usr/bin/env bash
# Usage: lint-schemas.sh [FILE...]
# Validates every document that has a schema against it, with yamale. Named
# files narrow it to the schemas that bind them; a file no schema maps to
# validates nothing, which is how the lint hook skips ordinary YAML.
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

wanted=""
for file in "$@"; do
  wanted="$wanted ${file#./} "
done

status=0
for schema in "${schemas[@]}"; do
  key=$(basename "$schema" -schema.yaml)

  doc=$(go run ./scripts/cmd/yamlkey "$CONFIG" "documents.$key")

  # A scoped run answers only for the files it was given, so it also skips
  # the unmapped-schema failure below — that is a repository invariant, and
  # the whole-repo run in gates.sh is where it belongs.
  if [ -n "$wanted" ]; then
    [ -n "$doc" ] || continue

    case "$wanted" in
    *" ${doc#./} "*) ;;
    *) continue ;;
    esac
  fi

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
