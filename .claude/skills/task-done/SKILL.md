---
name: task-done
description: Close the Ticket bound to this Task as DONE — set the ClickUp status and clear the binding. Takes no argument; it acts on the bound Ticket and nothing else. Run it after the merge has landed.
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh *) mcp__claude_ai_ClickUP__getTask mcp__claude_ai_ClickUP__updateTask mcp__claude_ai_ClickUP__addTaskComment
---

# Task Done

Bound Ticket: !`"${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" bound`

## What you own, and what you do not

Move that Ticket `PROCESSING → DONE` and clear the binding. **That is the
whole job.** You do not decide whether the work deserves it — whoever called
you already did. Design record: `docs/for_further/task-automation.md`.

If the line above is empty, no Ticket is bound: stop and say so. Do not go
looking for a plausible candidate — closing a Ticket nobody worked on is worse
than closing none.

## Steps

1. `updateTask` the bound Ticket's status to `DONE`.
2. `addTaskComment` naming the merge commit or the PR, so the Ticket says how
   it was closed.
3. Clear the binding — **only after step 1 succeeded**:

   ```bash
   "${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" unbind
   ```

4. Report the Ticket id and its new status in one line.

## Rules

- **Order matters.** Unbind last. A failed status write with the binding
  already gone leaves a Ticket stuck in `PROCESSING` that nothing can close.
- **Never set `COMPLETED`.** It is a human state; this skill sets `DONE` and
  stops there.
- **A hand-merged Ticket is still `DONE`.** When the user cancelled the
  mandate and merged the PR themselves, this is the skill that closes it —
  same status, same comment.
