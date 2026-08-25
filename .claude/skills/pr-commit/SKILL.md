---
name: pr-commit
description: Run quality gates, commit and push, and update the PR. Use when the user says "commit", "push", "commit and push", or similar.
---

# PR Commit

Invoked with the argument `auto` (only `handle-loop` does), step 3 runs
non-interactively — everything else is unchanged. Without it, ask as usual.
A trailing `--changed <base>` is passed straight through to step 1.

1. Run `./.claude/skills/pr-commit/gates.sh [--changed <base>]`. Bare it
   runs every gate; `--changed main` runs only the ones the diff needs,
   which is what `handle-loop` passes. The pipeline itself is data in
   `scripts/gates` — read that package's doc comment before changing it.
2. Run `./.claude/skills/pr-commit/scan-recordings.sh` — a deterministic
   sweep of the fixture recordings for home paths, session inventory,
   credentials and e-mail addresses. It is separate from the review
   below because a re-record changes ~475 files, four times CodeRabbit's
   per-review limit, and what recordings need is exhaustiveness rather
   than judgement. A hit means re-record after fixing the shim — never
   edit a cassette by hand.
3. Invoke the `sync-doc-universe` skill — audits the current state of
   the declared documents against `docs/doc-universe.{md,html}`, in both
   directions, asking the user about every inconsistency. Pass it `auto`
   when this skill was invoked with `auto`; it then resolves each one by
   its documented rule instead of asking, and reports what it decided.
4. Invoke the `update-memory` skill — updates CLAUDE.md when the pending
   diff changes anything it records (repo structure, conventions,
   commands, workflows), so the update lands in this commit.
5. Run `./.claude/skills/pr-commit/commit.sh` (stages everything,
   including files steps 2–4 touched).
6. Invoke the `pr-update` skill.
