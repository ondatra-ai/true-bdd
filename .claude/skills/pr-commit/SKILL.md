---
name: pr-commit
description: Run quality gates, commit and push, and update the PR. Use when the user says "commit", "push", "commit and push", or similar.
---

# PR Commit

**Only on the user's explicit command to commit** (CLAUDE.md).
Call the command in the background and create a monitor that publishes status
every 1 minute. If the user wants to cancel, they can do so by running
`kill -9 <pid>` where `<pid>` is the process ID of the commit command.

```bash
go run ./scripts/cmd/commit
```

No arguments: the gates narrow themselves when `task-handle` has stamped a
mandate, and `sync-doc-universe` always runs unattended. Report the decisions
it prints — they landed in the commit without anyone being asked.
