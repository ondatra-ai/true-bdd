---
name: pr-merge
description: Merge the current branch's PR with one command — three bounded CodeRabbit review rounds, triage, fixes, ClickUp deferrals, approval and merge. Use whenever the user wants to merge a PR, finish a branch, land changes, or says "merge this", "land it", "ship it", "we're done with this branch".
---

# PR Merge

**Only on the user's explicit command to merge** (CLAUDE.md, CRITICAL).

```bash
python3 ./.claude/skills/pr-merge/merge.py
```

One script, start to finish, **no arguments and no flags**. The repository
and the PR both come from the current checkout; passing a PR number is a
refusal, not an override. Run it in the **background** — a full run takes
the better part of an hour, and longer if it has to wait out the review
quota. Then report what it printed: the per-round table, what was fixed,
what was ticketed, and the merge result.

It refuses before doing anything if you are on `main`/`master`, on a
detached HEAD, or on a branch whose PR is closed or does not exist.

## What it does

Three rounds, then the merge:

| round | reviews | fixes | tickets |
|---|---|---|---|
| 1, 2 | yes | score ≥ 9 | score 6-8 |
| 3 | yes | **none** | score ≥ 6 |

Round 3 fixing nothing is the point: it commits nothing, so the commit it
reviewed is the commit it approves. Everything below 6 is recorded to
`tmp/merge/round-N/ignored.json` and dropped. Every thread is answered and
resolved at the end of every round.

All three rounds always run. There is no early exit and no resume: the loop
in `main()` is the whole algorithm, and a round that changes no files still
buys the review that pins the approval to HEAD.

## When it stops

It stops, it does not improvise. Any of these exits non-zero with a reason:

- A fix left the gates red, or failed. Its files are reverted; nothing is
  committed. A finding scored ≥ 9 that cannot be fixed needs a person.
- The gates are red at commit time although every fix reported them green.
- Scoring returned no verdict for a finding, or an unparseable answer.
- CodeRabbit accepted a review request and never posted the review.
- Ticket filing failed — a thread cannot be answered with a destination
  that does not exist.

`Review rate limited` is **not** a stop. The script recognises it, sleeps 15
minutes and asks again, for as long as it takes. Do not post
`@coderabbitai review` yourself while it is waiting.

## Rules

- Never merge without an explicit user command, whatever the checks say.
- The `fix-now` tickets it files are worked by `fix-queue`, on a separate
  command. Landing this PR authorises nothing more.
- The post-mortem at the end reads this session's history and files
  improvements to ClickUp under `merge-improvements`. It has read-only
  tools and asserts a clean worktree afterwards.
