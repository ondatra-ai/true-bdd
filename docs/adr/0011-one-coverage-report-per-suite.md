# A coverage report per suite, in a directory the engine hands over

**Superseded by ADR 0012.** The engine reads step coverage from source now;
`coverage:`, `$TRUEBDD_COVERAGE_REPORT_DIR` and the report format below are all
gone. Kept for the reasoning, which ADR 0012 answers rather than repeats.

`build tests` asks the host how its steps bind before it walks anything, through
the one `architecture.testing.commands.coverage` command. That command used to
be handed a single file path in `$TRUEBDD_COVERAGE_REPORT`. It is now handed a
directory in `$TRUEBDD_COVERAGE_REPORT_DIR`, every suite the command starts
writes its own report into it, and the engine merges all of them.

## What the single file cost

One command can start several suites — `go test ./tests/...` builds one binary
per package — and every one of them wrote to the same path. The last writer won,
and nothing could tell: a half-answer is a well-formed report. So a host could
only ever name one test tree, and this repository named `./tests/bdd-cli/`.

The consequence is not a smaller answer, it is a bill. A scenario the ask never
examined is walked by a model to learn what the ask knows in under a second. On
2026-09-04 that was 35 of the 244 bdd-web scenarios: fully bound, and each one
holding an Opus turn for ~36 seconds to be told so.

It also gets worse with success, which is what makes it a defect rather than a
limit. Every scenario `build tests --fix` finishes becomes a fully-bound
scenario the next run pays a turn for. bdd-cli is the suite that costs nothing,
and it is the only one the command asked.

## Why a directory rather than the alternatives

**A command per suite** — `coverage:` becoming a list — would work and was
rejected: `architecture.testing` is one section for the whole repository (ADR
0009), and a list of commands re-grows the per-suite block that ADR removed.

**Keeping one file and refusing when two suites write it** makes the loss
visible without making the answer available. The suites can already answer; it
was only the transport that could not carry them.

**Appending records to one file** would need every writer to be atomic against
every other. A report of 1081 unbound steps is ~95KB, far past any width a
concurrent append is atomic at.

## Why the variable was renamed rather than reinterpreted

Both halves of a version mismatch have to fail loudly, and the name is what
makes them. A suite written against the file protocol sees no variable, writes
nothing, and the engine refuses with `ErrNoReport` naming the command that was
supposed to answer. A suite written against this protocol under an older engine
does the same. Reusing the name would instead have failed inside the suite as
`open …: is a directory`, which names nothing a person can act on.

## Consequences

- A host implementing the old protocol must move; there is no compatibility
  shim, and the refusal above is the migration notice.
- The file name is the suite's own (`<suite>.json`), but identity is read from
  the report's `suite` field, so a `coverage:` pointed at the wrong package is
  still caught by `ErrWrongSuiteReported`.
- Two reports claiming one suite is a new refusal (`ErrDuplicateSuiteReported`)
  rather than a merge: they disagree about a tree only one can have examined.
- Nothing had to be re-recorded. No fixture declares `coverage:` and no registry
  scenario covers the ask — which is itself a gap, and unrelated to this change.
