---
name: pr-merge
description: Merge the current branch's PR with one command — up to three bounded CodeRabbit review rounds, triage, fixes, ClickUp deferrals, approval and merge. Use whenever the user wants to merge a PR, finish a branch, land changes, or says "merge this", "land it", "ship it", "we're done with this branch".
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

Up to three rounds, then the merge:

| round | reviews | fixes | tickets |
|---|---|---|---|
| 1, 2 | yes | score ≥ 9 | score 6-8 |
| 3 | yes | **none** | score ≥ 6 |

Round 3 fixing nothing is the point: the commit it reviewed is the commit it
approves. Everything below 6 is recorded to `tmp/merge/round-N/ignored.json`
and dropped. Every thread is answered and resolved at the end of every round.

**The loop ends as soon as a round changes nothing.** A round that fixes nothing
resolves its conversations, files its tickets, and ends the loop — the next
round would review a byte-identical tree, and a review costs a quarter of
the hourly quota. So a clean PR spends one review, not three. Round 3 always
ends the loop by construction, because its fix band is empty by definition.
There is still no resume: `main()`'s loop is the whole algorithm.

## When it stops

It stops, it does not improvise. Any of these exits non-zero with a reason:

- **The working tree is dirty, the branch has no upstream, origin does not
  have HEAD, or local HEAD is not the PR's head commit.** It names what to
  run and stops. It will not commit or push on your behalf — publishing
  work you did not decide to publish is not something a merge command gets
  to do.
- **The worktree is dirty when the merge is reached.** Checked as a merge
  precondition, because the merge ends in `git checkout main`, which refuses
  on a dirty tree *after* the PR is squashed and the branch deleted —
  stranding the checkout on a branch that no longer exists.
- **The commit to be approved is not the commit that was reviewed.**
  Checked against the review record immediately before approving, because
  `@coderabbitai approve` analyses nothing and stamps whatever HEAD is.
- A fix left the gates red or failed. Nothing is reverted and nothing is
  committed — the worktree keeps the round's work, the failed fix included,
  and the message lists it.
- **The worktree was dirty before the fixes ran**, so a path dirty
  afterwards could not be told from a fix.
- **A fix changed no file, or named a file git does not report changed.**
  A stop rather than a shrug: the thread would otherwise be answered "Fixed
  on this branch" when it was not, and triage scoring the finding 9-10 while
  the fixer sees nothing to do is a disagreement a person has to settle.
- The gates are red at commit time although every fix reported them green.
- Scoring returned no verdict for a finding, or an unparseable answer.
- CodeRabbit accepted a review request and never posted the review.
- Ticket filing failed — a thread cannot be answered with a destination
  that does not exist.

`Review rate limited` is **not** a stop. The script recognises it, sleeps for
as long as the bot says it needs ("your next included review will be
available in 18 minutes"), and asks again — for as long as it takes. It
keeps re-reading the acknowledgement while waiting, because CodeRabbit
rewrites that same comment from "triggered" to "rate limited" seconds after
posting it. Do not post `@coderabbitai review` yourself while it is waiting.

## Rules

- Never merge without an explicit user command, whatever the checks say.
- The `fix-now` tickets it files are worked by `fix-queue`, on a separate
  command. Landing this PR authorises nothing more.
- The post-mortem at the end reads this session's history and files
  improvements to ClickUp under `merge-improvements`. It has read-only
  tools and asserts a clean worktree afterwards.
