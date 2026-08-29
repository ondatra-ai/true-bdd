---
name: task-handle
description: Take one ClickUp Ticket from TO DO to merged and DONE, unattended — check it is complete, start, do the work, scope-check, commit, review, merge, close. Halts untouched if any field is missing. Pass the ticket id. Use when the user names one ticket to work, or says "handle this ticket". For the whole queue use task-loop.
disable-model-invocation: true
disallowed-tools: AskUserQuestion
argument-hint: "<ticket-id>"
allowed-tools: Bash(go *)
---

# Task Handle

Session history is already rolled — the injection below ran first:

!`go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/history roll`

If that block carries a `level=WARN` naming a `ticket=`, say so **before**
anything else: that Ticket is still `PROCESSING` and nothing else will ever
mention it. Then carry on — a report, not a refusal.

**Invoking this skill is the mandate to merge without asking** — the one place
that overrides CLAUDE.md's "NEVER merge a pull request without an explicit
command".

Call the command in the background and create a monitor that publishes status
every minute. To cancel, `kill -9 <pid>`; to stop it merging but let the
current step finish, `go run ./scripts/cmd/history unmandate`.

```bash
go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/task-handle $ARGUMENTS
```

It owns the whole algorithm: check the Ticket against `ticket-schema.yaml`,
start it, plan and implement it, scope-check, commit, review, merge, close,
report, and log the run. It refuses to start off `main` or on a dirty tree, and
it exits 0 for every verdict — `DONE`, `FAILED`, `halted`, `awaiting merge`
and `not started` alike.

Report its outcome line and checklist **verbatim** — also left at
`tmp/task-handle/outcome.md`. Do not re-run it, do not finish what it declined,
and do not touch ClickUp yourself.

**If it reports that the ClickUp log append failed**, print the entry it
quoted, verbatim, so a person can paste it by hand. That is the one thing the
command cannot finish for itself.
