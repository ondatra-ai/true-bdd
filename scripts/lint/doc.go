// Package lint is every quality gate this repository runs on its own source.
//
// It replaces five shell scripts (lints.sh, lint-comments.sh, lint-schemas.sh,
// lint-markdown.sh, lint-claude.md.sh). The gates were the only code here that
// nothing gated: the awk state machine in Comments now has tests.
//
// It no longer dispatches. Which file selects which gate is .alint.yml's
// answer now, and scripts/cmd/linters is the closure alint calls with it
// Two behaviours survived that move and are load-bearing:
//
//	accumulate     a gate reports everything it found, not the first thing —
//	               alint runs each rule whether or not another failed
//	fix vs check   a scoped run fixes and a bare one reports, because a gate
//	               that edits is a gate whose exit code means nothing
package lint
