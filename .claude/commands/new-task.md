---
description: Start a new task — the repo resets to a clean up-to-date state on the current branch (docs/context writes survive), history rolls over on the next prompt, and the closed task is archived to docs/context in the background
allowed-tools: Bash(*)
---
!`"${CLAUDE_PROJECT_DIR:-.}/.claude/commands/new-task.sh"`
