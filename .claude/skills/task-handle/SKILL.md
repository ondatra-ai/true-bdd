---
name: task-handle
description: Take one ClickUp Ticket from TO DO to merged and DONE, unattended — check it is complete, start, do the work, scope-check, commit, review, merge, close. Halts untouched if any field is missing. Pass the ticket id. Use when the user names one ticket to work, or says "handle this ticket". For the whole queue use task-loop.
disable-model-invocation: true
disallowed-tools: AskUserQuestion
argument-hint: "<ticket-id>"
allowed-tools: Bash(git *) Bash(gh *) Bash(go *) mcp__claude_ai_ClickUP__getTask mcp__claude_ai_ClickUP__editPage
---

# Task Handle

Ticket: `$ARGUMENTS`. One Ticket, one branch, one PR, one outcome.

```!
git -C "${CLAUDE_PROJECT_DIR}" rev-parse --abbrev-ref HEAD
git -C "${CLAUDE_PROJECT_DIR}" status --short
```

**Invoking this skill is the mandate to merge without asking** — the one place
that overrides CLAUDE.md's "NEVER merge a pull request without an explicit
command".

Refuse to start if the checkout is not on `main`, or the tree is dirty.

**Every failure halts.** This skill has exactly two recoveries — red gates in
step 5 and review findings in step 6, sharing one cap of five. Anything else
that fails, refuses, or cannot be invoked stops the pipeline where it stands
and waits for the user: see **Halting**.

## 1. Check

`getTask` the Ticket and hold it against `ticket-schema.yaml` in this
directory. Every field it lists must be filled, and the body must carry all
four headings.

**Anything missing → halt.** Report `not started` and exactly which field or
heading is empty. Write nothing: no `updateTask`, no `setCustomFieldValue`, no
comment, no status. The Ticket stays in `TO DO` untouched, so it is not a
`FAILED` — a human fills it in.

## 2. Start

Invoke `task-start` with the ticket id. It rolls the history, binds the
Ticket and moves it to `PROCESSING`.

```bash
go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/history mandate <ticket-id>
```

The mandate tells `scripts/merge` to use the automatic triage row (1–8 skip,
9–10 ticket). It is honoured only while this Ticket is bound.

## 3. Work

Only what the Ticket describes. Growing past it is a decline. If the change is
already in the code, or the Ticket is wrong about it, decline with the evidence.

## 4. Scope check

`git diff --stat main` against the Ticket's `Expected Changes` globs. One
chance: narrow to scope, or decline.

## 5. Commit

Invoke `pr-commit`. It takes no arguments — the mandate above is what
narrows its gates. Put the ClickUp URL in the PR body.

Gates red → fix and retry. **Five retries across the whole Ticket**; the sixth
is a decline.

## 6. Review

Invoke `code-review` with `main` as the fixed point and the Ticket body as the
spec, both as arguments. Spec axis only; Standards findings are logged and
block nothing.

- missing or wrong → fix and retry, same cap of five;
- present but not asked for → decline.

## 7. Merge

Has the user said anything since step 2? Then the mandate for this Ticket is
cancelled:

```bash
go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/history unmandate
```

Leave the PR open, report `awaiting merge`, stop. The user merges and runs
`/task-done`.

Otherwise invoke `pr-merge`. It ends in `git checkout main`.

## 8. Close

```bash
go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/clickup close DONE "merged: <pr url> (<sha>)"
```

The body of `/task-done`, called directly because that skill is
`disable-model-invocation` and this one runs unattended. Same three effects in
the same order: status, comment, unbind.

## Halting

Any command, skill or API call that fails, refuses, or cannot be invoked stops
the run **at that point**. Not a retry, not a workaround, not a degraded path
carried forward with a note — the pipeline waits for the user. The only
exceptions are the two recoveries the steps name for themselves, and only
inside their shared cap of five; the sixth is a halt.

```bash
go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/history unmandate
```

- **Cancel the mandate**, above. The user is coming, so nothing may merge on
  its own afterwards.
- **Write no status.** `DONE` and `FAILED` are verdicts; a halt is neither.
- **Leave the binding.** `/task-done` and `/task-fail` read it to know what
  they are closing, and which one applies is the user's call.
- **Touch nothing else** — no revert, no branch delete, no PR close, no stash,
  no `git checkout`. The state the failure left is the evidence.
- Report `halted`, name the step, and quote the failure verbatim.

A step that half-succeeded is the dangerous case: say which half landed, in
the Note column, rather than completing or undoing it.

## Declining

Refusing to merge on the merits: unasked-for work, a scope mismatch that would
not narrow, gates red after five retries, or any point needing a question.

```bash
go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/clickup close FAILED "<why, one or two sentences>"
```

Leave the branch and the PR as the failure left them — no close, no delete, no
revert.

## 9. Report

One line: id, title, outcome (`DONE` · `FAILED` · `halted` · `awaiting merge` ·
`not started`), PR URL. For `not started` name the missing field; for `halted`
name the step.

Then the checklist — every step, every run, whatever the outcome. One line
each, in order, marker then note:

```text
- 1 Check ✓
- 2 Start ✓ bound; `PROCESSING` set
- 3 Work ✓
- 4 Scope check ✓ 2 files under `scripts/merge/**`
- 5 Commit ⚠ 1 of 5 retries — gates red on `lint-comments`
- 6 Review ⚠ 3 Standards findings, block nothing
- 7 Merge ✓ PR #96 `7f50cbe`
- 8 Close ✓ `done`
```

`✓` done · `⚠` done, read the note · `✗` failed · `–` not reached.

- `✗` and `⚠` always carry a note; a bare `✗` is not a report. `✓` carries one
  only when it says something the user cannot get from the outcome line.
- `⚠` is what completed but the user should still see: a retry consumed, a
  finding logged that blocks nothing, scope deliberately left unfinished, a
  document this change made stale.
- After a halt, collapse the tail into one line — `4–8 – not reached`. A step
  that never ran did not fail, and eight `–` lines say nothing.
- Otherwise never merge two steps onto one line, and never drop a step to
  shorten the list.

## 10. Log the run

The same checklist goes to the **Task Execution log**, appended as one entry,
as the last action of *every* outcome — `DONE`, `FAILED`, `halted`,
`awaiting merge` and `not started` alike. A run nobody logged is a run nobody
can audit, and the failures are the entries worth having.

<https://app.clickup.com/90151491867/v/dc/2kyq568v-34735>

`editPage` with `editMode: "append"` — never `replace`, which would drop every
earlier entry:

```text
workspaceId  90151491867
docId        2kyq568v-34735
pageId       2kyq568v-25235
editMode     append
```

The entry is the outcome line, the same checklist, and at most one line of
anything the list could not hold:

````markdown
## <YYYY-MM-DD HH:MM UTC> · <ticket-id> · <OUTCOME>

<ticket title> — PR [#<n>](<pr url>) `<merge sha>`

- 1 Check ✓
- …
- 8 Close ✓ `done`
````

Drop the PR link when there is no PR yet, and the sha when nothing merged.

**This doc is not the Ticket.** Step 1's `not started` still writes nothing to
ClickUp's *task* — no status, no comment, no field. The log entry is a
separate artifact and is always written.

**If the append itself fails, the run is already over — there is nothing left
to halt.** Do not retry into a half-written entry and do not treat it as a
`FAILED` run. Say so in the response and print the entry verbatim, so it can
be pasted by hand.
