---
description: Start a new task — history rolls over on the next prompt; the closed task is archived to docs/context in the background
allowed-tools: Bash(*)
---
!`"${CLAUDE_PROJECT_DIR:-.}/.claude/hooks/history.py" new-task && ("${CLAUDE_PROJECT_DIR:-.}/.claude/hooks/context.py" sweep </dev/null >/dev/null 2>&1 &)`
