---
name: pr-commit
description: Run quality gates, commit and push, and update the PR. Use when the user says "commit", "push", "commit and push", or similar.
---

# PR Commit

1. Run `./.claude/skills/pr-commit/gates.sh` (lint + build + tests).
2. Invoke the `sync-doc-universe` skill — audits the current state of
   the declared documents against `docs/doc-universe.{md,html}`, in both
   directions, asking the user about every inconsistency.
3. Invoke the `update-memory` skill — updates CLAUDE.md when the pending
   diff changes anything it records (repo structure, conventions,
   commands, workflows), so the update lands in this commit.
4. Run `./.claude/skills/pr-commit/commit.sh` (stages everything,
   including files steps 2–3 touched).
5. Invoke the `pr-update` skill.
