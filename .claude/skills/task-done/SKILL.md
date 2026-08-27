---
name: task-done
description: Close the Ticket bound to this Task as DONE — set the ClickUp status, comment how it landed, clear the binding. Takes the merge commit or PR as its argument. Run it after the merge has landed.
disable-model-invocation: true
argument-hint: "<merge commit or PR url>"
allowed-tools: Bash(go *)
---

# Task Done

Whoever called you already decided the work deserves this. You do not.

```bash
go -C "${CLAUDE_PROJECT_DIR}" run ./scripts/cmd/clickup close DONE "merged: $ARGUMENTS"
```

The script reads the bound Ticket, sets the status, adds the comment and
clears the binding — in that order, and it touches nothing else on the Ticket.
It exits non-zero and says why if no Ticket is bound, or if ClickUp refused.

Report its output. That is the whole skill.

A Ticket the user merged by hand after cancelling the mandate is closed the
same way.
