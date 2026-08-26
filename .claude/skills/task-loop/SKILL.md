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

You own the **queue and nothing else**. One Ticket at a time — this instance is
the mutex.

Refuse to start if the checkout is not on `main`, or the tree is dirty.

A bound Ticket above is not a refusal: `/task-start` clears a stale binding
itself. Report its id in your first line — it is still `PROCESSING` and nobody
closed it — and carry on.

## The loop

Keep a list of ids you have already handed over this run — call it **seen**.

1. `listTasks` on list `901523097822`, `statuses: ["TO DO"]`.
2. Keep those with **Good For Agent** checked and not in **seen**. Order by
   **Triage Score**, highest first. Empty → say so and stop.
3. Add the top id to **seen**, then `/task-handle <that id>`.
4. Print its report line. Go to 1.

**seen** is what makes the loop terminate. `task-handle` halts on an incomplete
Ticket without writing anything, so a Ticket that came back `not started` is
still `TO DO` with `Good For Agent` set and would otherwise be the top of the
queue again, forever.

`task-handle` owns everything inside one Ticket, including the mandate and
every status write. Do not check, groom, branch, commit, merge or set a status
here — and do not touch a Ticket it declined to start.

## Outcomes

Every outcome continues the loop — one Ticket failing is not the queue failing:

- `DONE` · `FAILED` → take the next.
- `not started` → the Ticket is incomplete and untouched. Take the next; a
  human fills it in.
- `awaiting merge` → the user interrupted; they merge that one. Take the next.

## Stop

Queue dry — every remaining Ticket is in **seen** — or the user says stop. Then
print the tally: counts per outcome, and every `not started` id with the field
it was missing, so a human knows exactly what to fill.
