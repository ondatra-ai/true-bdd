---
name: task-fail
description: Close the Ticket bound to this Task as FAILED — set the ClickUp status, comment the reason, clear the binding. Pass the reason as the argument. Leaves the branch and the PR exactly as the failure left them.
disable-model-invocation: true
argument-hint: "<reason>"
allowed-tools: Bash(${CLAUDE_PROJECT_DIR}/.claude/skills/lib/close-task.sh *)
---

# Task Fail

Whoever called you already decided the work failed. You do not.

`$ARGUMENTS` is the reason. If it is empty, ask for one — a `FAILED` Ticket
with no reason is one a human has to re-derive from the branch, which is the
cost this skill exists to avoid. Add the branch name and the PR URL if there
is one.

```bash
"${CLAUDE_PROJECT_DIR}/.claude/skills/lib/close-task.sh" FAILED "<reason, branch, PR>"
```

The script reads the bound Ticket, sets the status, adds the comment and
clears the binding — in that order, and it touches nothing else on the Ticket.

**Leave the wreckage alone.** Do not close the PR, delete the branch, revert
or stash. The state is left exactly as the failure left it so a person can see
what happened.

Report the script's output.
