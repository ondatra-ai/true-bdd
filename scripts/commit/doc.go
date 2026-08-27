// Package commit is the whole pr-commit workflow as one program.
//
// It replaces a six-step checklist the model executed by hand — gates.sh,
// scan-recordings.sh, two skill invocations, commit.sh and pr-update.sh — with
// the shape scripts/merge already has: one command, run to completion or to a
// diagnosis, with the judgement calls delegated to headless `claude -p` turns
// rather than to the session driving it.
//
//	1 gates          the quality pipeline, narrowed under a mandate
//	2 recordings     a deterministic sweep of the committed cassettes
//	3 doc-universe   sync-doc-universe, in unattended mode
//	4 memory         update-memory, so CLAUDE.md lands in this commit
//	5 stage          git add -A, refusing an empty commit
//	6 branch         a generated name, only when standing on the trunk
//	7 message        generated from the staged diff and the recent style
//	8 commit + push
//	9 pull request   created or edited, and its URL printed
//
// Step 3 always runs `auto`. A headless program has nobody to ask, so the
// alternative to resolving by the skill's documented rules is not resolving
// at all; every decision is printed, and lands in the diff to be reviewed
// there instead of before the fact.
package commit
