---
name: task-start
description: Start a new Task — roll session history, bind a ClickUp Ticket to it, and move that Ticket TO DO → PROCESSING. Pass a ticket id to take an existing one; with no argument it asks which Ticket to take or creates one from what you describe. Leaves the repo and the current branch untouched.
disable-model-invocation: true
argument-hint: "[ticket-id]"
allowed-tools: Bash(${CLAUDE_SKILL_DIR}/roll-history.sh) Bash(${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh *) mcp__claude_ai_ClickUP__getTask mcp__claude_ai_ClickUP__listTasks mcp__claude_ai_ClickUP__createTask mcp__claude_ai_ClickUP__updateTask
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

### 1. Get a Ticket

**With an id in `$ARGUMENTS`** — `getTask` it. It must exist and its status
must be `TO DO`. If it is already `PROCESSING`, `DONE` or `FAILED`, stop and
say so: that Ticket is somebody's work, not a fresh start.

**With no argument** — ask the user, and do not let up until one of these two
is chosen. There is no third "no ticket" option, and no "skip for now":

- **take an existing one** — `listTasks` on list `901523097822` filtered to
  `TO DO`, show them, let the user pick;
- **create one** — ask what needs doing, then write it up and `createTask` in
  that list.

A Ticket you create carries the four headings `scripts/clickup` renders:

```text
### Why
### What to change
### Verification
### Context
```

Write it to be picked up cold by someone who was not in this conversation.
`What to change` names `file:line`. `Verification` is a command that can be
run — it is what will later decide `DONE`.

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

One line: the Ticket id, its title, and its URL. Then stop and wait — the work
itself is the next prompt's business, not this skill's.

## Rules

- **One Ticket per Task.** The binding holds until `/task-done` or
  `/task-fail`; nothing else clears it.
- **Never touch the working tree.** No `git checkout`, no `git stash`, no
  cleanup. Starting a Task must not be able to discard uncommitted work.
- **Never set any status but `PROCESSING`.** `DONE` and `FAILED` belong to
  `/task-done` and `/task-fail`; `COMPLETED` and moves out of `FAILED` are the
  user's, in the ClickUp UI.
