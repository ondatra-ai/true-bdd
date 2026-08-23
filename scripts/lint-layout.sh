#!/usr/bin/env bash
# Usage: lint-layout.sh
# Two shape rules: this repository is Go and sh, and its Go lives in three
# roots.
#
# NO PYTHON. PR #85 ported the last three Python scripts to Go, and the same
# commit shipped a stale `__pycache__/merge.cpython-314.pyc` — bytecode of the
# very file it deleted — because the ignore rule went in that commit while the
# artifact was still on disk and `commit.sh` stages with `git add -A`. Nothing
# caught it. This does. Untracked files count too, so running a Python tool in
# the tree fails here rather than at the next `git add -A`.
#
# Three roots and no fourth. `services/<name>/` is product code, `tests/<name>/`
# is what exercises it, and `scripts/` is the tooling that drives the
# repository itself — the merge loop, the ClickUp interface, the history hook,
# the lint gates. A fourth root is not a new kind of code, it is the same code
# somewhere a reader has to learn about, so this refuses one.
#
# Untracked-but-not-ignored files are checked too, so a stray package fails
# here rather than at review: the gate runs before the commit that would
# introduce it.
#
# Scope is Go only, deliberately. Shell lives wherever its caller does — the
# skill entry scripts under .claude/, `start.sh` at the root, the fixture
# prep/teardown scripts beside their fixtures — and a shell script sitting next
# to the thing it runs is not the problem this gate exists to prevent.
set -euo pipefail

cd "$(dirname "$0")/.."

ROOTS='services tests scripts'
PATTERN="^($(echo "$ROOTS" | tr ' ' '|'))/"

python=$(git ls-files -co --exclude-standard '*.py' '*.pyi' '*.pyc' '*.pyo' |
	grep -vE '^tests/(legacy|bdd-cli/fixtures)/' || true)
pycache=$(git ls-files -co --exclude-standard | grep '__pycache__/' || true)

if [ -n "$python$pycache" ]; then
	echo "lint-layout: Python in a Go and sh repository:" >&2
	printf '%s\n%s\n' "$python" "$pycache" | grep -v '^$' | sort -u | sed 's/^/    /' >&2
	echo >&2
	echo "    Port it to Go under scripts/, or delete it. Bytecode: rm -rf the __pycache__." >&2
	exit 1
fi

stray=$(git ls-files -co --exclude-standard '*.go' | grep -vE "$PATTERN" || true)

if [ -n "$stray" ]; then
	echo "lint-layout: Go files outside the three roots ($ROOTS):" >&2
	echo "$stray" | sed 's/^/    /' >&2
	cat >&2 <<-'EOF'

		Move each one into the root that owns it:
		    services/<name>/   product code
		    tests/<name>/      what exercises it
		    scripts/           tooling that drives this repository
	EOF
	exit 1
fi

count=$(git ls-files -co --exclude-standard '*.go' | wc -l | tr -d ' ')
echo "lint-layout: OK ($count Go files, all under $ROOTS)"
