---
name: pr-merge
description: Merge the current branch's PR with one command — up to three bounded CodeRabbit review rounds, triage, fixes, ClickUp deferrals, approval and merge. Use whenever the user wants to merge a PR, finish a branch, land changes, or says "merge this", "land it", "ship it", "we're done with this branch".
---

# PR Merge

**Only on the user's explicit command to merge** (CLAUDE.md, CRITICAL).
Call in command in background and create monitor that publish status every 1 minute. If the user wants to cancel, they can do so by running `kill -9 <pid>` where `<pid>` is the process ID of the merge command.
```bash
go run ./scripts/cmd/merge
```
