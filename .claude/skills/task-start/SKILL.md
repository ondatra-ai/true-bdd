---
name: task-start
description: Start a new Task — roll session history, bind one ClickUp Ticket to it, and move that Ticket TO DO → PROCESSING. Takes the ticket id; without one it refuses. Use when task-handle reaches its Start step, or when the user names a ticket to start. Leaves the repo and the current branch untouched.
disallowed-tools: AskUserQuestion
argument-hint: "<ticket-id>"
allowed-tools: Bash(${CLAUDE_SKILL_DIR}/roll-history.sh) Bash(${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh *) mcp__claude_ai_ClickUP__getTask mcp__claude_ai_ClickUP__updateTask
---

# Task Start

Session history is already rolled by the time you read this — the injection
below ran first:

!`${CLAUDE_SKILL_DIR}/roll-history.sh`

Ticket argument: `$ARGUMENTS`

## What you own, and what you do not

Bind exactly one ClickUp Ticket to this Task and move it to `PROCESSING`.
**That is the whole job.** Do not create a branch, do not start the work, do
not check whether the previous Task was finished, do not judge whether this
Ticket is well written. Those belong to whoever called you — `task-handle` or
the user.

## Steps

### 0. Relay an orphan, if the injection reported one

If the block above carries a `WARNING:` about a ticket that was still bound,
say so to the user **before** anything else. That Ticket is still `PROCESSING`
in ClickUp and nothing else will ever mention it. Then carry on — this is a
report, not a refusal.

### 1. Get the Ticket

`getTask` the id in `$ARGUMENTS`. It must exist and its status must be
`TO DO`. If it is already `PROCESSING`, `DONE` or `FAILED`, stop and say so:
that Ticket is somebody's work, not a fresh start.

**An empty `$ARGUMENTS` is a refusal.** Report that the ticket id is missing
and stop, having written nothing — no bind, no status. Choosing which Ticket
to take, and writing one that does not exist yet, are the caller's job: the
user picks from ClickUp, or `task-loop` reads the queue.

### 2. Bind it

```bash
"${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" bind <ticket-id>
```

### 3. Move it to PROCESSING

`updateTask` the Ticket's status to `PROCESSING`. Do this **after** the bind:
a bind with no status change is a Task that can still be closed; a status
change with no bind strands the Ticket, because `/task-done` and `/task-fail`
read the binding to know what they are closing.

### 4. Report

One line: the Ticket id, its title, and its URL. Then hand back to the caller —
the work itself belongs to `task-handle`, or to the user's next prompt, not to
this skill.

## Rules

- **One Ticket per Task.** The binding holds until `/task-done` or
  `/task-fail`; nothing else clears it.
- **Never touch the working tree.** No `git checkout`, no `git stash`, no
  cleanup. Starting a Task must not be able to discard uncommitted work.
- **Never set any status but `PROCESSING`.** `DONE` and `FAILED` belong to
  `/task-done` and `/task-fail`; `COMPLETED` and moves out of `FAILED` are the
  user's, in the ClickUp UI.
