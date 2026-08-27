// Package lint is every quality gate this repository runs on its own source,
// and the dispatcher that picks which ones a given file needs.
//
// It replaces five shell scripts (lints.sh, lint-comments.sh, lint-schemas.sh,
// lint-markdown.sh, lint-claude.md.sh) and the PostToolUse hook that drove
// them — see docs/adr/0002 for why none of them could stay shell. The gates
// were the only code here that nothing gated: the awk state machine in
// Comments now has tests, and the curl-and-diff mirror check in ClaudeMD
// reports a torn read instead of a silent pass.
//
// Two behaviours are load-bearing and were preserved exactly:
//
//	accumulate     every gate runs even after one fails, so a single pass
//	               reports everything rather than the first thing
//	fix vs check   naming files fixes them; a bare run mirrors CI and must
//	               never rewrite, because a gate that edits is a gate whose
//	               exit code means nothing
package lint
