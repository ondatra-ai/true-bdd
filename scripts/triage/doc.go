// Package triage scores one claim about this repository, against the tree as
// it stands.
//
// One function, three callers, one rubric. `scripts/merge` scores a CodeRabbit
// finding, its postmortem scores a proposal about the tooling, and
// `scripts/clickup` scores an existing backlog ticket — all the same question,
// answered on the same 1-10 band and written to the same ClickUp dropdown.
// Before this package they were two differently-worded scales and a third path
// that scored nothing at all.
//
// What differs between callers is only where the subject comes from and what
// is done with the Verdict, so both stay OUTSIDE: this package does not import
// scripts/clickup, which is what lets clickup import it.
//
// The turn READS the repository — Read, Glob and Grep under
// `--permission-mode plan`, which refuses writes at the permission layer
// rather than by asking. The scorer this replaces passed no allowlist at all,
// so its instruction to be skeptical of a reviewer who "can be wrong about
// this codebase" was unenforceable: it had never seen the codebase. Relevance
// is not a second axis — a finding about code that no longer exists has no
// consequence if left undone, which is what makes one band cover both.
package triage
