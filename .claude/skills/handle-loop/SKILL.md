---
name: handle-loop
description: Drive ClickUp Tickets to merge, one at a time and unattended — take the top-scoring ready Ticket, groom it, start it, do the work, commit, review, merge, and close it DONE or FAILED, then take the next. Pass a ticket id to run just that one. Use when the user says "work the queue", "run the loop", "take the next ticket", or "handle-loop".
disable-model-invocation: true
disallowed-tools: AskUserQuestion
argument-hint: "[ticket-id]"
allowed-tools: Bash(${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh *) Bash(git *) Bash(gh *) Bash(go *) mcp__claude_ai_ClickUP__listTasks mcp__claude_ai_ClickUP__getTask mcp__claude_ai_ClickUP__updateTask mcp__claude_ai_ClickUP__addTaskComment mcp__claude_ai_ClickUP__setCustomFieldValue
---

# Handle Loop

Argument: `$ARGUMENTS` — a Ticket id runs exactly that one and stops. Empty
runs the queue until it is dry.

Repository state at invocation:

```!
git -C "${CLAUDE_PROJECT_DIR}" rev-parse --abbrev-ref HEAD
git -C "${CLAUDE_PROJECT_DIR}" status --short
"${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" bound
```

**Design record: `docs/for_further/task-automation.md`.** Read it before
departing from anything below; it carries the reasoning this file only
carries the conclusions of.

## What you own

The queue, the grooming, the work, the scope check, the gates, the review,
the retries, and the **decision** of `DONE` versus `FAILED`. You do not write
a status yourself — `/task-start`, `/task-done` and `/task-fail` do, and
nothing else does, in either mode.

## The mandate

Invoking this skill **is** the mandate: merging without asking is authorised
for the whole run. Two things bound it:

- **A message from the user cancels the mandate for the current Ticket only.**
  Drive that one to a PR and stop there; the user merges it and runs
  `/task-done` themselves — that is still a `DONE`. Then carry on with the
  next Ticket. Do not treat an interruption as "stop the queue"; if the user
  wants that, they will say so.
- **`AskUserQuestion` is removed from your tools.** There is nobody to ask, so
  a point where you would have asked is a **decline** — see below. Note that
  the frontmatter restriction lapses the moment the user *does* send a
  message, which is exactly the cancellation case above: from then on the tool
  is back and this rule is prose, not machinery. Follow it anyway.

## Preconditions, checked once

**The checkout must be on `main` and clean.** A dirty tree means somebody's
uncommitted work would ride into the first Ticket's branch. Refuse and say so.

A **bound Ticket** in the block above is not a refusal. `/task-start` clears a
stale binding itself and warns about it, so refusing here would fire on a
state step 3 is about to fix. Relay the orphan's id in your first report —
that Ticket is still `PROCESSING` in ClickUp and nobody closed it — and carry
on. Recovering *it* is a human's job (§10 of the design record); noticing it
is yours.

## The loop

Repeat until the queue is empty, or forever-once when given an id.

### 1. Take a Ticket

```text
listTasks  list 901523097822, statuses ["TO DO"]
```

Keep only those with **Good For Agent** checked. Order by **Triage Score**,
highest first. Take the top one.

Skip and log any Ticket whose **Scope** is `PROJECT`: that value means "not
clear what this touches", which is a statement that the Ticket is not thought
through — the worst possible candidate for an unattended run, not merely a
large one.

Queue empty → say so and stop.

### 2. Groom it

Read the Ticket in full — `getTask` returns the untruncated description — and
hold it against `ticket-schema.yaml` in this skill's own directory. That file
is the definition of ready; it names every field, what each is for, and what
makes one unfixable.

Anything missing, **fix it in the Ticket before starting**: you can read the
code the Ticket names, so derive what is derivable and write it back with
`updateTask` (body) or `setCustomFieldValue` (`Expected Changes`, `Scope`,
`Triage Score`). Grooming is repair, not judgement.

What you cannot derive — a `Verification` command for a Ticket that describes
no observable outcome — is a Ticket that cannot be judged done. Comment what
is missing, uncheck `Good For Agent`, leave it in `TO DO`, and take the next
one. It never enters `PROCESSING`, so it is not a `FAILED`.

**Re-validate before working.** The Ticket records what somebody claimed when
it was filed, possibly several merges ago. Read the file it names. Already
fixed, or wrong about the code → comment the evidence (the commit that fixed
it, or the technical argument), **uncheck `Good For Agent`**, leave it in
`TO DO`, and take the next.

Do not set a status on it. Only `/task-start`, `/task-done` and `/task-fail`
write statuses, and all three need a bound Ticket in `PROCESSING` — which this
one never was. Unchecking the predicate is what takes it out of the queue; a
human sweeps it to `COMPLETED` or reopens it, which is the same division of
labour as everywhere else here.

### 3. Start it

```text
/task-start <ticket-id>
```

That rolls history, binds the Ticket and moves it to `PROCESSING`. Do not do
any of those three yourself.

Then stamp the mandate for this Ticket, so `scripts/merge` — a separate
process that cannot see a skill invocation — applies the automatic triage row
(1–8 skip, 9–10 ticket) instead of the manual one:

```bash
"${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" mandate <ticket-id>
```

It is honoured only while that same Ticket is bound, so it cannot leak into a
later manual merge. Re-stamp it for every Ticket.

### 4. Do the work

Keep it to what the Ticket describes. A fix that grows past its Ticket is a
new decision, and a new decision under a mandate is a **decline**.

### 5. Scope check

Compare `git diff --stat main...HEAD` against the Ticket's declared blast
radius. One chance: shown what you touched against what was declared, either
narrow to scope or decline.

The declaration is the Ticket's **`Expected Changes`** field: a glob list, one
per line or comma-separated. Globs name a directory or a pattern, not an exact
file — that leaves you free to add a test beside the code or split a file
while the district stays fixed.

If a Ticket somehow reached here with the field empty, grooming (step 2)
failed to do its job. Fill it from `Scope` and the `What to change` heading,
`updateTask` it, and say in your report that the radius was inferred rather
than declared. Never skip the check silently.

### 6. Commit

Invoke the `pr-commit` skill with the arguments `auto --changed main`. That
runs only the gates the diff needs — a documentation Ticket costs ~2s instead
of ~140s — then the recordings scan, `sync-doc-universe` in its
non-interactive mode, the memory update, the commit (which cuts the branch off
`main` itself) and the PR.

Put the ClickUp URL in the PR body, so the merge and the Ticket are linked
from both ends.

Gates red → fix and retry, **at most five times across the whole Ticket**.
The sixth is a decline.

### 7. Review

Invoke `code-review` with `main` as the fixed point and the Ticket body as
the spec, both passed as arguments so it never stops to ask. **Only the Spec
axis matters here**; this repository enforces its standards mechanically, and
Standards findings are logged and block nothing.

- missing or wrong → fix and retry, same cap of five;
- **present but not asked for** → decline. The Ticket is not describing the
  work, and that is a human's call.

### 8. Merge

Invoke `pr-merge`. It runs the CodeRabbit rounds, the triage, the approval,
the merge, and ends in `git checkout main` itself.

Before invoking it, check whether the user has said anything **since this
Ticket was started** — not since the run began; the window resets per Ticket,
because cancellation is per Ticket. If they have, the mandate for this one is
cancelled:

```bash
"${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" unmandate
```

Leave the PR open, report it, and go to the next Ticket without merging. The
user merges that one and runs `/task-done` themselves. Cancellation is an
explicit write — nothing here expires on its own.

### 9. Close it

```text
/task-done
```

Then loop. Given an id in `$ARGUMENTS`, stop here instead.

## Declining

A decline is refusing to merge **on the merits**: work the Ticket did not ask
for, a scope mismatch that would not narrow, gates still red after five
retries, or any point where you would have had to ask the user a question.

```text
/task-fail <why, in one or two sentences>
```

Leave the branch and the PR exactly as the failure left them. Do not close the
PR, do not delete the branch, do not revert. Then take the next Ticket — one
Ticket failing is not the queue failing.

## Report as you go

After every Ticket, one line: id, title, outcome, PR URL. At the end, the
tally and anything that ran degraded (see below). A silent loop is one nobody
can audit afterwards.

## Say when the run was weaker than designed

Every piece this loop depends on is built. Two things still make a particular
run weaker than the design, and neither is a reason to refuse — both are
reasons to say so in the final report:

- **you had to infer a radius** because a Ticket reached step 5 with
  `Expected Changes` empty (step 2 should have caught it);
- **`Good For Agent` was set by a human who was not you**, which is by design
  (§11) and is the single judgement the loop does not make. If a Ticket looked
  ready and was not, that is the number this whole design is measured by:
  eight of ten reaching `DONE` on the first attempt.
