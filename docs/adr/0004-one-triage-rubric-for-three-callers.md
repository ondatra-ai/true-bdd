# One triage rubric, for three callers

`scripts/triage` holds the only scale this repository scores anything by:
`Score(Subject) (Verdict, error)`, 1-10 by the consequence of leaving the
subject undone, read against the tree as it stands. Three callers reach it —
`scripts/merge` for a CodeRabbit finding, its postmortem for a proposal about
the tooling, `scripts/clickup` for an existing Ticket. What differs between
them is only where the Subject comes from and what they do with the Verdict.

## What was there before

Two rubrics and a path that scored nothing, all writing the same ClickUp
`Triage Score` dropdown and all read by the same `Floors` cutoffs:

| path | axis | bands | calibration | validation |
|---|---|---|---|---|
| `merge/prompts/rubric.txt` | consequence if left unfixed | 9-10 / 6-8 / 1-5 | six worked examples | 1-10 enforced |
| `merge/prompts/postmortem.txt` | "time or noise it would save" | none | none | none — the model's number went straight onto the Finding |
| `clickup defer` | — | — | — | *"Set no custom field"* |

A number from the second was compared against a floor calibrated for the
first. The third filed Tickets with the field `ticket-schema.yaml` calls
required left empty, which is why every hand-written deferral sorted last in
a queue `task-loop` orders by score.

## Relevance is not a second axis

The re-triage sweep that prompted this looked like it needed its own scale:
"is this still real" reads as a different question from "how bad is it". It is
not. A finding about code that no longer exists has no consequence if left
undone, so it scores 1 on the scale that was already there. One band covers
both, and `1` gained one sentence rather than the rubric gaining an axis.

The alternative — a second `Relevance` field beside `Triage Score` — was
rejected for what it does to the reader. Two numbers need a rule for combining
them at every disposition site, and `task-loop` would have to learn which one
orders the queue.

## The scorer had never seen the code

Folding relevance in forces something the old scorer could not do.
`merge.scoreAll` passed no `AllowedTools`, so its instruction — *"Be skeptical.
A reviewer can be wrong about this codebase. If the finding misreads the code,
score it 1"* — asked for a judgement it had no way to make. The shared turn
runs with `Read,Glob,Grep` under `--permission-mode plan`, which refuses writes
at the permission layer rather than by asking (`docs/for_further/headless-plan-then-implement.md` verified that plan mode
composes with `-p`).

The cost is real and was accepted deliberately: one turn per subject rather
than one turn for a whole round. A six-finding round now spends six scoring
turns. The turns are the point — a blind scorer is fast at being wrong.

## Where the seam is

`scripts/triage` does not import `scripts/clickup`, which is what lets
`clickup` import it while `merge` imports both. `Subject` is therefore its own
type rather than `clickup.Finding`, and the one branch inside the shared
function is `Subject.Refresh`: set by the callers whose body is a Ticket
carrying the four `###` headings, unset for a review comment, whose body
stays the reviewer's own words because the fix agent and the GitHub thread
reply are both anchored to it.

Disposition stays with the caller. `merge` keeps its `Floors` table — under a
mandate its ticket floor is 9, not 6 — and `triage.Floor` is only the line the
Ticket-shaped callers use: below it `clickup defer` files nothing and
`clickup triage` retires the Ticket to `not relevant`.

## What a score now carries with it

`Triage Date` and `Triage Commit` are written by every path that scores. A
score without them cannot be re-checked, because it does not say which tree it
was true of; with them, `clickup triage <N>` can walk the least-recently-judged
Tickets and advance rather than re-reading the same rows. An unset date sorts
oldest, so the Tickets filed before the fields were ever written are swept
first.

This adds a third mover of a Ticket's status, after `task-start` and
`clickup close`. It is the only one that moves a status *down*, and it moves
only to `not relevant`: a Ticket a person promoted to `to do` stays there,
because a sweep judges whether the work is still real, not whether it is
queued.
