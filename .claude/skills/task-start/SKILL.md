---
name: task-start
description: Start a new Task — roll session history and, given a ticket id, bind that ClickUp Ticket to it and move it TO DO → PROCESSING. The id is optional; without one the Task carries no Ticket. Use when task-handle reaches its Start step, or when the user starts a Task. Leaves the repo and the current branch untouched.
disallowed-tools: AskUserQuestion
argument-hint: "[ticket-id]"
allowed-tools: Bash(go *) mcp__claude_ai_ClickUP__getTask mcp__claude_ai_ClickUP__updateTask
---

# Task Start

Session history is already rolled by the time you read this — the injection
below ran first:

!`go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/history roll`

Ticket argument: `$ARGUMENTS`

## What you own, and what you do not

Bind at most one ClickUp Ticket to this Task and move it to `PROCESSING`.
**That is the whole job.** Do not create a branch, do not start the work, do
not check whether the previous Task was finished, do not judge whether this
Ticket is well written. Those belong to whoever called you — `task-handle` or
the user.

## Steps

### 0. Relay an orphan, if the injection reported one

If the block above carries a `level=WARN` record naming a `ticket=`, that
Ticket was still bound. Say so to the user **before** anything else. That Ticket is still `PROCESSING`
in ClickUp and nothing else will ever mention it. Then carry on — this is a
report, not a refusal.

### 1. Get the Ticket

**An empty `$ARGUMENTS` is a Task with no Ticket.** Skip the rest of this step
and steps 2 and 3 — bind nothing, move no status, call ClickUp not at all —
and go to step 4. Never pick a Ticket to fill the gap: choosing which one to
take is the caller's job, the user naming it or `task-loop` reading the queue.

`getTask` the id in `$ARGUMENTS`. It must exist and its status must be
`TO DO`. If it is already `PROCESSING`, `DONE` or `FAILED`, stop and say so:
that Ticket is somebody's work, not a fresh start.

### 2. Bind it (with an id)

```bash
go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/history bind <ticket-id>
```

### 3. Move it to PROCESSING (with an id)

`updateTask` the Ticket's status to `PROCESSING`. Do this **after** the bind:
a bind with no status change is a Task that can still be closed; a status
change with no bind strands the Ticket, because `/task-done` and `/task-fail`
read the binding to know what they are closing.

**Refused → undo the bind, then refuse.** Half a Task is worse than none: the
Ticket stays in `TO DO`, where `task-loop`'s queue predicate hands it out
again while it is already being worked.

```bash
go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/history unbind
```

Report the refusal verbatim and stop — never substitute a status that does
exist. `ITEM_114 Status does not exist` means the list's status set has
drifted from the one this workflow assumes; a human fixes ClickUp.

### 4. Report

One line. With a Ticket: its id, its title, and its URL. Without one: that this
Task has no Ticket bound, so `/task-done` and `/task-fail` will refuse it —
`clickup close` reads the binding and there is nothing to close. Then hand back
to the caller — the work itself belongs to `task-handle`, or to the user's next
prompt, not to this skill.

## Rules

- **At most one Ticket per Task.** A binding holds until `/task-done` or
  `/task-fail`; nothing else clears it.
- **Never touch the working tree.** No `git checkout`, no `git stash`, no
  cleanup. Starting a Task must not be able to discard uncommitted work.
- **Never set any status but `PROCESSING`.** `DONE` and `FAILED` belong to
  `/task-done` and `/task-fail`; `COMPLETED` and moves out of `FAILED` are the
  user's, in the ClickUp UI.
