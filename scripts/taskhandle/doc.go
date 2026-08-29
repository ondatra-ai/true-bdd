// Package taskhandle takes one ClickUp Ticket from TO DO to merged and DONE,
// unattended. It is the algorithm the task-handle skill used to carry as 214
// lines of prose for a model to re-interpret on every run.
//
// Eight steps, then two that report: check the Ticket is formally ready, take
// it, plan and implement it, hold the diff against what it said it would
// touch, commit, review, merge, close — then the checklist and the run log.
//
// # Commit and merge are package imports
//
// Step 5 is commit.Embed() and step 7 is merge.Embed(), which are
// Start(nil).Main() with the report render suppressed. No subprocess, no
// `claude -p`, no /pr-commit and no /pr-merge: one process, one logging.Run()
// id, and therefore ONE report tree with their trees nested inside this one.
// The turns' tool allowlists are spelled verb by verb rather than Bash(go *)
// for the same reason — that blanket would let an agent shell back to either
// command. Exactly one skill is reached through claudecli, at step 6:
// code-review, which has no Go implementation to call instead.
//
// # Halting, declining, and the mandate
//
// A halt is "something broke, a person decides": the mandate goes, the binding
// STAYS so /task-done and /task-fail can read what they are closing, and
// nothing is written to ClickUp. Step 2 is the exception — a bind this run
// just wrote is undone, because a bound Ticket still in TO DO is one the queue
// predicate hands out while it is being worked.
//
// A decline is "this run judged the work must not merge": unmandate first,
// then FAILED with the reason, then unbind. Step 1 is neither — an unready
// Ticket is `not started`, and writes nothing at all.
//
// The mandate is also the stop button. A Go process cannot see the session, so
// state.MandateKey is re-read before every turn and before the merge; clearing
// it with `go run ./scripts/cmd/history unmandate` stops the run at the next
// one, with the PR left open.
//
// # Where the log lands
//
// The skill rolls the history in its ! fence before this command starts, which
// drops the `log` key, so state.TaskLog resolves the named no-task.json
// fallback. Two runs before the user's next prompt share that file;
// report.Fold filters on the run id, so their trees stay separate.
package taskhandle
