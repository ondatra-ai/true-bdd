#!/usr/bin/env bash
# Usage: lint-comments.sh
# Comment budget: at most 3 lines of prose, at most 15 lines of block scheme.
#
# Prose is argument, and argument compresses. A comment earns length only by
# carrying a fact learned by OBSERVATION — one a simplifier would delete
# without any gate going red — and such a fact fits in one line plus a line of
# provenance. State it once, at the line it guards; the full incident belongs
# in docs/for_further/.
#
# A block scheme is not prose. Pseudocode, a table or a diagram is already the
# compressed form, and squeezing it turns it back into the paragraphs this
# gate exists to remove — so indented lines are counted separately and allowed
# 15. Past that it is a program with worse syntax, and belongs in a README.
#
# Indentation is the test, because Go already encodes it: in a gofmt'd doc
# comment an indented line is a preformatted block, so gofmt maintains the
# distinction for free.
#
# EXEMPT FROM THE PROSE CAP — the file's own manual, which is a different
# genre from annotation:
#   * Go   — the doc comment immediately preceding `package X`. `go doc`,
#            pkgsite and gopls hover all surface it, and this repo ships gopls
#            as a plugin, so it is the one channel an agent gets for free on
#            every symbol lookup. A README beside the package is a second
#            place none of that tooling reads.
#   * sh, yaml — the header block at the top of the file, same reason: it is
#            where this repo already keeps a rule (see lint-schemas.sh).
# The 15-line block cap still applies to both.
#
# NOT LINTED: generated files (`// Code generated`, written by
# `true-bdd build tests --fix` and never hand-edited), the fixture inputs
# under tests/bdd-cli/fixtures/, and the parked tests/legacy/ tree.
set -euo pipefail

cd "$(dirname "$0")/.."

MAX_PROSE=3
MAX_BLOCK=15

check() {
	local lang=$1
	shift
	git ls-files -co --exclude-standard "$@" |
		grep -vE '^(tests/bdd-cli/fixtures/|tests/legacy/)' |
		while read -r file; do
			[ -f "$file" ] || continue
			head -n 5 "$file" | grep -q '^// Code generated' && continue
			awk -v F="$file" -v LANG="$lang" -v MAXP="$MAX_PROSE" -v MAXB="$MAX_BLOCK" '
				function flush(next_line,   exempt) {
					if (prose + block == 0) return
					exempt = (LANG == "go") ? (next_line ~ /^package /) : first_run
					if (!exempt && prose > MAXP)
						printf "%s:%d: %d lines of prose (max %d)\n", F, start, prose, MAXP
					if (block > MAXB)
						printf "%s:%d: %d-line block scheme (max %d)\n", F, start, block, MAXB
					if (LANG != "go") first_run = 0
					prose = 0; block = 0
				}
				NR == 1 && /^#!/ { next }
				/^[[:space:]]*(\/\/|#)/ {
					if (prose + block == 0) start = NR
					body = $0
					sub(/^[[:space:]]*(\/\/|#)/, "", body)
					# An indented line is a preformatted block, not prose.
					if (body ~ /^(\t|    )/) block++; else prose++
					next
				}
				{ flush($0) }
				END { flush("") }
				BEGIN { first_run = 1 }
			' "$file"
		done
}

findings=$(
	check go '*.go'
	check hash '*.sh' '*.yaml' '*.yml'
)

if [ -n "$findings" ]; then
	count=$(printf '%s\n' "$findings" | wc -l | tr -d ' ')
	printf '%s\n' "$findings" | sed 's/^/    /' >&2
	cat >&2 <<-EOF

		lint-comments: $count comment(s) over budget ($MAX_PROSE prose, $MAX_BLOCK block).
		Cut argument, keep the fact. A fact worth its line names where it was
		learned (PR #, comment id) and sits at the line it guards, once — the
		narrative goes to docs/for_further/.
	EOF
	exit 1
fi

echo "lint-comments: OK (all comments within $MAX_PROSE prose / $MAX_BLOCK block lines)"
