---
name: task-fail
description: Close the Ticket bound to this Task as FAILED — set the ClickUp status, comment the reason, and clear the binding. Pass the reason as the argument. Leaves the branch and the PR exactly as the failure left them.
disable-model-invocation: true
argument-hint: "<reason>"
allowed-tools: Bash(${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh *) mcp__claude_ai_ClickUP__getTask mcp__claude_ai_ClickUP__updateTask mcp__claude_ai_ClickUP__addTaskComment
---

# Task Fail

Bound Ticket: !`"${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" bound`

Reason given: `$ARGUMENTS`

## What you own, and what you do not

Move that Ticket `PROCESSING → FAILED`, record why, and clear the binding.
**That is the whole job.** You do not decide that the work failed — whoever
called you already did. Design record:
`docs/for_further/task-automation.md`.

If the line above is empty, no Ticket is bound: stop and say so.

## Steps

1. If `$ARGUMENTS` is empty, ask for the reason. A `FAILED` Ticket with no
   reason is one a human has to re-derive from the branch, which is the whole
   cost this skill exists to avoid.
2. `addTaskComment` with the reason, the branch name, and the PR URL if there
   is one.
3. `updateTask` the bound Ticket's status to `FAILED`.
4. Clear the binding — **only after step 3 succeeded**:

   ```bash
   "${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" unbind
   ```

5. Report the Ticket id, its new status, and where the work was left.

## Rules

- **Leave the wreckage alone.** Do not close the PR, do not delete the branch,
  do not revert or stash anything. A stop is a stop, and the state is left
  exactly as the failure left it so a person can see what happened.
- **Order matters.** Unbind last, for the same reason as `/task-done`.
- **`FAILED` is terminal for this loop.** Only the user moves a Ticket back
  out of it, in the ClickUp UI.
