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
3. Run `./.claude/skills/pr-commit/review.sh` — CodeRabbit reviews the
   working tree, untracked files included, and writes findings to
   `tmp/review-findings.jsonl`. Then **triage them exactly as
   `pr-merge` §2a does**: the same table, the same three verdicts (Fix
   now / Defer → #N with the `deferred-review` label / Reject with a
   technical argument), the same rule that a verdict needs a
   code-anchored reason. Apply the fixes, re-run gates, and record the
   round:

   ```bash
   ./.claude/skills/pr-commit/ledger.py --source coderabbit-cli \
     --findings tmp/review-findings.jsonl --verdicts fix=6,defer=1,reject=1
   ```

   **One round only.** Findings are gathered once, triaged once, fixed
   once. Anything the GitHub bot raises after the push is the next round,
   entered deliberately — not a loop that re-reviews its own fixes.

   Reviewing here rather than after the push is the point: the first
   reviewer to see a change used to be the GitHub bot, and a fixture
   recording carrying a developer's home directory and connected
   integrations reached a public repository that way. A missing
   `coderabbit` CLI skips the step loudly and never blocks the commit.
4. Invoke the `sync-doc-universe` skill — audits the current state of
   the declared documents against `docs/doc-universe.{md,html}`, in both
   directions, asking the user about every inconsistency.
5. Invoke the `update-memory` skill — updates CLAUDE.md when the pending
   diff changes anything it records (repo structure, conventions,
   commands, workflows), so the update lands in this commit.
6. Run `./.claude/skills/pr-commit/commit.sh` (stages everything,
   including files steps 2–5 touched).
7. Invoke the `pr-update` skill.

## The ledger

`docs/ci/review-rounds.json` records every round — what was found, at
what severity, and what triage decided. Its point is a question that can
otherwise only be argued about: **is reviewing before the push worth its
minutes?** Compare `coderabbit-cli` rounds against the
`coderabbit-github` rounds that follow them. Findings the bot raises
after a push are the local round's misses; a local round whose findings
are mostly rejected is costing more than it saves. Record the GitHub
round too, from `pr-merge`, or the comparison has only one side.
