# Nothing is filed that a Ticket already covers

Every path that creates a Ticket now asks a model whether the tracker already holds
the same proposal, and files only what scores 1-3 on a 1-10 identity band. The gate
runs before the render, so the heading count, the field plan and `report`'s check all
agree on one shortened queue — the property `dropAlreadyOpen` already had and the
reason it drops where it does.

## What the title-prefix filter could not see

`FileDeduped` matched a queued finding against an open Ticket on the first 60 runes of
the title (`scripts/clickup/file.go`, `matchWidth`). That catches a finding filed twice
under one title and nothing else, because a duplicate is routinely filed under a
different one. On 2026-09-02 the list held one proposal five times —

- 86cb8tqug "Check before sleeping in all three poll loops, and read reviewDecision…"
- 86cb9fu9a "Read once before the first sleep in the ack, review and approval waits"
- 86cba69ha "Read before sleeping in the three poll loops, and report elapsed time…"
- 86cbaeppv "Poll before sleeping in approve, awaitAcknowledgement and awaitReview"
- 86cbb1cck "Poll before the first sleep in the ack, review and approval waits"

— and six more clusters like it. No two share a 60-rune prefix; all five ask for the
same edit. A prefix is a cheap first pass and stays one, but it cannot be the only one.

## Why `File` is gated too, against what its own comment said

`File` was ungated on the argument that "a review finding recurring across PRs is
news". The premise is that recurrence means a *different* pull request. It does not:
`86cbcw2zk` / `86cbcy51v`, `86cbcw317` / `86cbcy54d` and `86cbcy55q` / `86cbd5az9` are
three byte-identical title pairs, all tagged `fix-now`, all filed by
`scripts/merge/tickets.go` from repeated runs against pull request #114. The comment
described a case the tooling does not produce, so it is rewritten rather than left to
drift, and both entry points are gated.

They keep separate names because they still differ, in one thing: what a gate that
cannot answer means.

- `File` files the queue **ungated** and logs the failure. `scripts/merge/tickets.go`
  calls `dief` on a `File` error, and a merge that aborts leaves review threads
  unanswerable — more expensive than a duplicate the next `clickup triage` retires.
- `FileDeduped` and `FileDocument` file **nothing**, wrapped in `ErrNotFiled`. That is
  the rule `withoutOpen` stated and this inherits: not knowing what is already filed is
  exactly the state the gate exists to refuse. Their callers warn and continue, and
  `clickup defer` is a person at a keyboard who can be told to try again.

## Why the corpus holds every status

The similarity turn reads `tmp/dupes/corpus/`, one markdown file per Ticket, dumped
fresh on every gate run — never cached, because one merge files `fix-now` and then the
postmortem minutes later, and the second corpus must contain what the first filing just
created.

Closed and retired Tickets are in it. A proposal that recurs after being judged
pointless would otherwise be filed and then retired again by the next sweep, which
costs two turns to reach the answer this reaches in none. A candidate matching a `done`
Ticket may be a genuine regression rather than a duplicate — the block names the
match's status precisely so `duplicate of a done Ticket` reads as something to look at
by hand, and reopening the closed Ticket beats filing a second one either way.

The prefix pass sees only `backlog` and `to do`. Sixty runes is enough to drop a filing
on work somebody may pick up, and not enough to drop one on a Ticket nobody will.

## Why a duplicate is marked with a comment

ClickUp's own linked-task relation is unreachable from this repository. There is no
link or dependency tool in the MCP roster, the REST mirror this session can reach is
GET-only, and a `tasks`-type custom field refuses every value shape the layer can send
— `{"add":[id]}`, `{"add":[id],"rem":[]}`, `[id]` and `id` all drew
`400 {"err":"Invalid Value","ECODE":"FIELD_342"}` when probed on 2026-09-02. That is
the failure `scripts/clickup/fields.go` already records for the `labels` field `Scope`
(FIELD_144): the MCP layer stringifies the value, so a structured field cannot be
written through it.

So a retired duplicate carries its keeper's URL in a comment, which `Status` writes in
the same turn as the status change. A `short_text` field would be machine-readable and
remains available — text fields do write — but it cannot be created by any tool here,
and nothing programmatic walks retired Tickets: `clickup triage` walks `backlog` and
`to do` only. The marker is for people, and a comment serves people.

## The threshold

A candidate is filed when its highest identity score is 3 or lower, and blocked at 4 or
higher — `fileableCeiling`, not overridable. Retiring an *existing* Ticket needs a
stronger claim than blocking a new one, because the work is already recorded and losing
it costs more than a re-file: `clickup dupes` groups only at 8 or above (`dupeFloor`).

A blocked candidate is a normal skip, logged with the id, URL, status and score of what
it duplicates. Never an error — the proposal exists in the tracker, which is the
outcome filing it was for.
