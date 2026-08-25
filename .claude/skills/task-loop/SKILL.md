---
name: task-loop
description: Work the ClickUp queue unattended — take ready Tickets highest Triage Score first, hand each to task-handle, and continue until the queue is dry. Use when the user says "work the queue", "run the loop", or "task-loop". For a single named ticket use task-handle.
disable-model-invocation: true
disallowed-tools: AskUserQuestion
allowed-tools: Bash(${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh *) Bash(git *) mcp__claude_ai_ClickUP__listTasks mcp__claude_ai_ClickUP__getTask
---

# Task Loop

```!
git -C "${CLAUDE_PROJECT_DIR}" rev-parse --abbrev-ref HEAD
git -C "${CLAUDE_PROJECT_DIR}" status --short
"${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" bound
```

Design record: `docs/for_further/task-automation.md`.

You own the **queue and nothing else**. One Ticket at a time — this instance is
the mutex.

Refuse to start if the checkout is not on `main`, or the tree is dirty.

A bound Ticket above is not a refusal: `/task-start` clears a stale binding
itself. Report its id in your first line — it is still `PROCESSING` and nobody
closed it — and carry on.

## The loop

1. `listTasks` on list `901523097822`, `statuses: ["TO DO"]`.
2. Keep those with **Good For Agent** checked. Order by **Triage Score**,
   highest first. Empty → say so and stop.
3. `/task-handle <top ticket id>`.
4. Print its report line. Go to 1.

`task-handle` owns everything inside one Ticket, including the mandate and
every status write. Do not groom, branch, commit, merge or set a status here.

## Outcomes

Every outcome continues the loop — one Ticket failing is not the queue failing:

- `DONE` · `FAILED` · `not started` → take the next.
- `awaiting merge` → the user interrupted; they merge that one. Take the next.

## Stop

Queue dry, or the user says stop. Then print the tally: counts per outcome, and
every `not started` id with what it was missing.
