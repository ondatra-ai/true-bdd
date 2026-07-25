---
name: pr-commit
description: Run quality gates, commit and push, and update the PR. Use when the user says "commit", "push", "commit and push", or similar.
---

# PR Commit

1. Run `./.claude/skills/pr-commit/gates.sh` (lint + build + tests).
2. Run `./.claude/skills/pr-commit/commit.sh`.
3. Invoke the `pr-update` skill.
