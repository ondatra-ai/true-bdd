---
name: pr-commit
description: Run quality gates, commit and push, and update the PR. Use when the user says "commit", "push", "commit and push", or similar.
---

# PR Commit

1. Run `./.claude/skills/pr-commit/gates.sh` (lint + build + tests).
2. Run `./.claude/skills/pr-commit/scan-recordings.sh` — a deterministic
   sweep of the fixture recordings for home paths, session inventory,
   credentials and e-mail addresses. It is separate from the review
   below because a re-record changes ~475 files, four times CodeRabbit's
   per-review limit, and what recordings need is exhaustiveness rather
   than judgement. A hit means re-record after fixing the shim — never
   edit a cassette by hand.
4. Invoke the `sync-doc-universe` skill — audits the current state of
   the declared documents against `docs/doc-universe.{md,html}`, in both
   directions, asking the user about every inconsistency.
5. Invoke the `update-memory` skill — updates CLAUDE.md when the pending
   diff changes anything it records (repo structure, conventions,
   commands, workflows), so the update lands in this commit.
6. Run `./.claude/skills/pr-commit/commit.sh` (stages everything,
   including files steps 2–5 touched).
7. Invoke the `pr-update` skill.

