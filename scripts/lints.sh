#!/usr/bin/env bash
# Usage: lints.sh [FILE...]
# Run the lint gates that apply to the named files; with no argument, all of
# them. The PostToolUse hook (.claude/hooks/lint-changed.sh) calls it with the
# file Claude just wrote, so a convention breach surfaces at that edit instead
# of at commit time.
#
# alint always runs: its rules are about where a file sits rather than what is
# inside it, so no single file selects them, and the whole tree answers in
# ~25ms. Everything else is selected by extension, and a gate two files select
# still runs once:
#
#	*.go           golangci-lint on its package, comments
#	*.sh           comments
#	*.yaml|*.yml   schemas, comments
#	*.md           markdownlint — CLAUDE.md excepted, .alint.yml owns it
#	anything else  nothing beyond alint
#
# Every gate is handed the named files and answers for those alone. Go is the
# exception: `golangci-lint run <file>.go` typechecks that file ALONE, so all
# 17 package-mates in review.go came back "undefined" — the package is the floor.
set -euo pipefail

cd "$(dirname "$0")/.."

comments=0
schemas=0
markdown=0
whole_repo_go=0
go_dirs=""

# A sentinel go.mod fences its subtree out of the root module, and
# golangci-lint asked to lint inside one exits 5 with "no go files to
# analyze" — so a .go file under one selects no Go gate at all.
select_go_package() {
	local dir
	dir=$(dirname "$1")

	local walk=$dir
	while [ "$walk" != "." ]; do
		[ -f "$walk/go.mod" ] && return
		walk=$(dirname "$walk")
	done

	case " $go_dirs " in
	*" ./$dir "*) ;;
	*) go_dirs="$go_dirs ./$dir" ;;
	esac
}

select_for() {
	case "$1" in
	*.md) markdown=1 ;;
	*.go)
		comments=1
		select_go_package "$1"
		;;
	*.sh) comments=1 ;;
	*.yaml | *.yml)
		comments=1
		schemas=1
		;;
	esac
}

if [ $# -eq 0 ]; then
	comments=1
	schemas=1
	markdown=1
	whole_repo_go=1
else
	for file in "$@"; do
		select_for "$file"
	done
fi

status=0

# Named files get `fix`, bare gets `check` — the same split golangci and
# markdownlint use below: bare mirrors CI and must not rewrite. alint applies
# only fixes a rule declares, and exits non-zero on what it could not fix.
if [ $# -gt 0 ]; then
	alint fix || status=1
else
	alint check || status=1
fi

if [ "$comments" = 1 ]; then
	./scripts/lint-comments.sh "$@" || status=1
fi

if [ "$schemas" = 1 ]; then
	./scripts/lint-schemas.sh "$@" || status=1
fi

if [ "$markdown" = 1 ]; then
	./scripts/lint-markdown.sh "$@" || status=1
fi

# `level=warning` is golangci's own chatter — nine lines of exclusion-rule
# bookkeeping per run, which buries the finding in what the hook hands back.
# sed, not grep: grep exits 1 when it filters everything, faking a failure.
hush() { sed '/^level=warning/d'; }

if [ "$whole_repo_go" = 1 ]; then
	golangci-lint run 2>&1 | hush || status=1
elif [ -n "$go_dirs" ]; then
	# --fix only when files are named: bare, this gate mirrors CI and must not
	# rewrite; named, fixing as the code is written is the point (whole package).
	# shellcheck disable=SC2086
	golangci-lint run --fix $go_dirs 2>&1 | hush || status=1
fi

exit "$status"
