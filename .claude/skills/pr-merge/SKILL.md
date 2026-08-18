---
name: pr-merge
description: Merge the current branch's PR using squash merge, delete the remote branch, switch to main, pull latest, and clean up the local branch. Resolves blocking review threads first — validating each comment, fixing the valid ones and rejecting the invalid ones with a reason. Use this skill whenever the user wants to merge a PR, finish a branch, land changes, or says things like "merge this", "land it", "ship it", "merge the PR", or "we're done with this branch".
---

# PR Merge

## 1. Check what is blocking

```bash
./.claude/skills/pr-merge/threads.sh status
```

`main` is protected by the **"Main Protection" ruleset**, not by classic
branch protection: two green checks (`gates` and `CodeRabbit`), **one
approving review**, **every review thread resolved**, squash-only, linear
history, no force-push, no deletion. `status` prints all of it, plus — for
each `CHANGES_REQUESTED` verdict — the commit it is pinned to, how far HEAD
has moved past it, and how many of its threads are still open. That last
block is the evidence step 2d is decided on.

**The admin role can bypass every one of those rules on a PR merge**
(`bypass_actors`, `pull_request` scope). So GitHub letting the merge
through is **not** evidence the preconditions were met — `threads.sh
preflight` is. Never read a successful `gh pr merge` as approval that
never arrived.

The ruleset also sets `dismiss_stale_reviews_on_push` and
`require_last_push_approval`, so **any push voids the approval**: the
approving review has to come after the final commit. Plan the order — fix
everything, push, *then* ask for the re-review.

If `gates` is `IN_PROGRESS`, wait for it — but **bound the wait**. Poll for
up to ~15 minutes; if it has not finished by then, stop and tell the user
what it is stuck on rather than continuing to poll. Do not merge with
`--admin` to skip it.

## 2. Clear the review

```bash
./.claude/skills/pr-merge/threads.sh list
```

**`list` covers two classes of finding, and only one of them is a thread.**
CodeRabbit posts "Actionable comments" as review threads — they have a
comment id, a reply target, and a resolve verb. It posts "Nitpick
comments", "Outside diff range comments" and "Duplicate comments" **inside
the review body only**: no id, no reply target, nothing to resolve, and
invisible to any helper that queries `reviewThreads`. On PR #70 that was 12
findings of 28, and they were merged past without ever being read.

`list` ends with a reconciliation:

```
THREADS: 16 (14 unresolved, 2 resolved)   BODY-ONLY: 12   TOTAL: 28
reviews claim: actionable=16 nitpick=12 outside-diff=0 duplicate=0
✓ reconciled
```

**A `✗ MISMATCH` exits non-zero and is a stop.** It means the extractor
disagrees with what the review says about itself, so some finding is
unaccounted for. Read the review body directly, fix `render_review.py`,
and only then continue. It is never a reason to proceed.

### 2a. Validate every finding, then print the triage table

Read the file each comment points at **before** judging it. A finding can
be stale (already fixed), misplaced (the line moved — the helper flags
`[OUTDATED]`), or simply wrong about this codebase. Do not assume the
reviewer is right, and do not assume it is wrong.

Print one table covering every open finding of both classes, before
touching any code:

| # | Class | Issue | File:line | Author | Verdict | Owner | Why |
|---|-------|-------|-----------|--------|---------|-------|-----|
| 1 | thread | one-line statement of the claim | `path:line` | reviewer | **Fix now** | this PR | what in the code makes it true — name the symbol, quote the line |
| 2 | body-only | … | … | … | **Defer → #123** | #123 | true, but out of this branch's scope — and where it now lives |
| 3 | thread | … | … | … | **Reject** | — | what in the code makes it false — the behaviour it misreads, the constraint it missed |

There are exactly three verdicts, and **every relevant finding gets an
owner**:

- **Fix now** — repaired on this branch, before the merge.
- **Defer → #N** — real, but not this branch's job. Deferral is only
  valid *with a destination*: file the issue as part of triage — with the
  `deferred-review` label, which is what step 5 lists it by — and put the
  number in the cell. Filing it is not a separate decision to ask
  about; it is what makes a deferral something other than a silent drop.
  An issue that changes recorded fixtures says so, so the re-record is
  planned rather than discovered later.
- **Reject** — not a real finding. Needs a technical argument, not a
  dismissal.

Rules for the table:

- Every finding gets a row, `thread` and `body-only` alike. No silent
  omissions, no "minor" bucket, and no fourth verdict — "valid but not
  here" without an issue number is the failure this taxonomy exists to
  prevent. A nitpick is not exempt for being a nitpick; it is exempt only
  by a **Reject** with a reason, like anything else.
- A verdict without a code-anchored reason is not a verdict. "Looks fine"
  is not a reason; "`collect` returns at line 59 before reading stdin" is.
- Findings sharing one root cause stay separate rows, each naming the
  shared cause — the reviewer posted them separately and each needs a
  reply.

### 2b. Print the plan, then execute it without asking

Follow the table with a numbered plan — one entry per relevant finding,
stating the action, the evidence, and how it will be checked:

```
1. Fix <thing> in <file> — <one line on the approach>
   now:   <the offending code, 1-5 lines>
   after: <the replacement, 1-5 lines>
   verify: <the exact command that proves it>
2. Re-record <what> — <why the recording is invalid as it stands>
   verify: go test -tags bdd … -mode=replay
3. …
```

Then **do it**. Do not ask for approval between items, do not ask whether
to start, and do not stop after the plan to check in. The plan IS the
announcement; the user reads it while the work runs. Report failures as
they happen — a plan is a statement of intent, not a promise that every
step worked.

Ask only when a step needs a decision that is genuinely the user's:
rewriting published history, `--admin`, dismissing a review, deleting
their data, or changing something outside this repository. Naming such a
step in the plan is not the same as having permission for it.

Filing a deferral issue is NOT one of those steps, despite being visible
on GitHub. It records a finding the review already made, in the tracker
that review lives in, and refusing to file it without asking is what
turns a deferral into a silent drop — the failure the taxonomy above
exists to prevent. Judge by consequence, not by whether a network call
is involved.

### 2c. Reply and resolve

Threads are answered one at a time, on the thread:

```bash
./.claude/skills/pr-merge/threads.sh reply <comment-id> "Valid — fixed …"
./.claude/skills/pr-merge/threads.sh reply <comment-id> "Rejected — …"
./.claude/skills/pr-merge/threads.sh resolve <thread-id>
```

**Body-only findings have no reply target**, so they are answered in
**one** PR conversation comment (`gh pr comment`) listing each by file,
line and verdict, in the same table shape as 2a. Say in that comment that
they were body-only and therefore not resolvable individually — otherwise
the next reader sees findings with no reply and assumes they were missed.

The reply carries the table's reason plus what was done. For a rejection,
give the evidence: the API's documented behaviour, the constraint the
suggestion would break, the reason the code is already correct.

Then re-run gates (`./.claude/skills/pr-commit/gates.sh`) and commit the
fixes.

### 2d. Ask for the re-review, and bound the wait

A push does not clear a `CHANGES_REQUESTED` verdict — the bot has to post
a new review object. Ask for one, then wait for it with a bound:

```bash
gh pr comment --body "@coderabbitai review"
# run this in the BACKGROUND — it sleeps
./.claude/skills/pr-merge/threads.sh await-review 900
```

- **exit 0** — a new review landed. Re-run `list` and go back to 2a; a new
  review means new findings, in both classes.
- **exit 3** — the bot acknowledged and never delivered. This is exactly
  what happened on PR #70 at 23:38, and it is where the 4h37m stall came
  from. **Stop. Print `threads.sh status`, and ask the user** whether to
  dismiss the stale verdict or keep waiting. Do not keep polling, and do
  not decide it yourself.

### Rules

- **Never resolve a thread you have not read and replied to.** Resolving
  claims the comment was answered. There is no bulk-resolve verb for
  exactly this reason: a reviewer bot posts a fresh review on every push,
  so `list` grows while you work, and a blind loop will sweep up comments
  that arrived seconds ago and were never looked at.
- **Re-run `list` after pushing.** A new push means a new review — and it
  re-reconciles, which is the only thing that notices a body-only finding
  arriving in a section that was empty last round.
- **A rejection is a technical argument, not a dismissal.** If it cannot
  be defended in two sentences against the actual code, it is a fix.
- **Dismissing a review is the user's call, and only for a stale verdict.**
  Never dismiss to avoid answering a finding. Dismissal is defensible only
  when *all* of these hold, and the user has said to do it:
  1. every thread from that review is answered and resolved,
  2. HEAD is ahead of the commit the verdict is pinned to (`status` prints
     both, and flags the combination as `→ STALE`),
  3. `gates` is green on HEAD,
  4. a re-review was requested and `await-review` returned exit 3.

  The dismissal message states that evidence — what was fixed, what was
  deferred and where, what was rejected and why, and that the re-review
  was requested and did not arrive.
- **Never use `gh pr review --dismiss` as a shortcut past an unanswered
  review.** Answer the comments; the bot re-reviews and clears its own
  verdict.
- **Ask before merging past a red preflight.** The admin bypass means no
  `--admin` flag is needed and GitHub will raise no objection — which is
  exactly why this has to be a deliberate question rather than a thing
  that just happens. Overriding the ruleset is the user's call.

## 3. Record the round

`pr-commit` records its local CodeRabbit round; this skill records the
GitHub one, or the comparison the ledger exists for has only one side:

```bash
./.claude/skills/pr-commit/ledger.py --source coderabbit-github \
  --pr <n> --count 28 --verdicts fix=6,defer=5,reject=17
```

`--count` is the `TOTAL` from `list` — **threads plus body-only
findings**. Counting only threads is what made PR #70's round record
`found=15` for a review that raised 28, which is why the miss left no
trace in the one place built to notice it. The verdict counts must sum to
that total, since every row in the 2a table has a verdict.

## 4. Merge

```bash
./.claude/skills/pr-merge/merge.sh
```

Runs `threads.sh preflight` first and refuses by name if the approval, the
`gates` check, or thread resolution is missing. Then squash-merges, deletes
the remote branch, switches to main, pulls, and removes the local branch.

## 5. Work the deferred queue

Landing on main is the middle of the job, not the end. Every issue filed
as **Defer → #N** in step 2a is now the backlog, and it is worked without
being asked again:

1. `gh issue list --label deferred-review --state open --json number,title \
   --jq 'sort_by(.number)'` — the queue, oldest first. Explicit ordering,
   because the default is newest-first and a queue worked backwards
   leaves the oldest finding permanently last.
2. Take the first issue. Branch, fix it, and run the same loop the
   original change ran: gates, then the `pr-commit` skill, then this
   skill from step 1. The PR body carries a closing keyword — `Closes
   #N` — so the issue is closed by the merge rather than by someone
   remembering to.
3. When it merges, take the next.

Rules:

- **One issue per PR** unless two share a single root cause, in which
  case one PR fixes the cause and closes both — the same rule step 2a
  applies to threads.
- **Stop and report** rather than continue when a fix turns out to be
  larger than its issue described, when it needs a decision that is the
  user's (published history, outward-facing behaviour, deleting data), or
  when the queue is empty.
- A deferred issue that re-records fixtures pays the recording cost in
  its own PR, and says so in the PR body — the cost belongs to the change
  that caused it.
