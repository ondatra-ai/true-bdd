#!/usr/bin/env bash
# Usage: lint-markdown.sh [FILE...]
# markdownlint-cli over the markdown this repository authors, configured by
# .markdownlint.yaml. Named files narrow the run and are auto-fixed; a bare
# run mirrors CI and only reports, because a gate that rewrites is a gate
# nobody can trust the exit code of.
#
# Four exclusions, each for a different reason:
#
#	.claude/skills/<vendored>/  someone else's files (MIT, mattpocock) —
#	                            fixing them makes the next re-sync a
#	                            three-way merge instead of a copy
#	CLAUDE.md                   owned by scripts/lint-claude.md.sh, and its
#	                            KARPATHY block is a verbatim upstream mirror:
#	                            a finding inside it would be unfixable
#	*/testdata/*                golden files — the bytes ARE the assertion
#	tests/{legacy,bdd-cli/fixtures}/  parked and fixture trees, as in
#	                            lint-comments.sh
#
# The vendored list is parsed from the manifest rather than copied here, so
# taking a 25th skill needs no edit in this file.
set -euo pipefail

cd "$(dirname "$0")/.."

MANIFEST=".claude/skills/VENDORED-mattpocock.md"
CONFIG=".markdownlint.yaml"

if ! command -v markdownlint >/dev/null 2>&1; then
	echo "markdownlint not found in PATH. Install it with: npm i -g markdownlint-cli" >&2
	exit 1
fi

# The manifest names them in two prose runs ("Engineering: a, b, c" and
# "Productivity: …"), each wrapped over several lines and ended by a blank.
vendored=$(awk '
	/^(Engineering|Productivity):/ { collecting = 1 }
	collecting && !NF { collecting = 0 }
	collecting { print }
' "$MANIFEST" |
	sed -E -e 's/^(Engineering|Productivity)://' -e 's/\.$//' |
	tr ',' '\n' | tr -d ' \t' | grep -v '^$' | paste -sd'|' -)

if [ -z "$vendored" ]; then
	echo "lint-markdown: parsed no vendored skills from $MANIFEST." >&2
	exit 1
fi

EXCLUDE="^\.claude/skills/($vendored)/|^CLAUDE\.md$|/testdata/|^tests/(legacy|bdd-cli/fixtures)/"

SPECS=(".")
if [ $# -gt 0 ]; then
	SPECS=("$@")
fi

files=$(git ls-files -co --exclude-standard "${SPECS[@]}" |
	grep -E '\.md$' | grep -vE "$EXCLUDE" || true)

if [ -z "$files" ]; then
	echo "lint-markdown: OK (no markdown in scope)"
	exit 0
fi

# --fix only when files are named; the hook's message promises it happened.
# Scalar, not array: macOS ships bash 3.2, where `"${empty[@]}"` under
# `set -u` is an unbound-variable error rather than nothing.
fix=""
if [ $# -gt 0 ]; then
	fix="--fix"
fi

# shellcheck disable=SC2086
if ! markdownlint --config "$CONFIG" $fix $files 2>&1; then
	cat >&2 <<-'EOF'

		lint-markdown: fix each finding above. What --fix could rewrite is
		already applied, so what remains needs a real edit — a fence without
		a language, a heading a file genuinely lacks.
	EOF
	exit 1
fi

count=$(printf '%s\n' "$files" | wc -l | tr -d ' ')
echo "lint-markdown: OK ($count markdown files)"
