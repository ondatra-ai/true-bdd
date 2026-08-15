---
name: pr-merge
description: Merge the current branch's PR using squash merge, delete the remote branch, switch to main, pull latest, and clean up the local branch. Resolves blocking review threads first — validating each comment, fixing the valid ones and rejecting the invalid ones with a reason. Use this skill whenever the user wants to merge a PR, finish a branch, land changes, or says things like "merge this", "land it", "ship it", "merge the PR", or "we're done with this branch".
---

# PR Merge

## 1. Check what is blocking

```bash
./.claude/skills/pr-merge/threads.sh status
```

`main` requires the `gates` check **and** no changes-requested review. If
`state` is `BLOCKED`, work through step 2 before merging. If `gates` is
`IN_PROGRESS`, wait for it — do not merge with `--admin` to skip it.

## 2. Clear the review threads

```bash
./.claude/skills/pr-merge/threads.sh list
```

### 2a. Validate every thread, then print the triage table

Read the file each comment points at **before** judging it. A finding can
be stale (already fixed), misplaced (the line moved — the helper flags
`[OUTDATED]`), or simply wrong about this codebase. Do not assume the
reviewer is right, and do not assume it is wrong.

Print one table covering every open thread, before touching any code:

| # | Issue | File:line | Author | Verdict | Owner | Why |
|---|-------|-----------|--------|---------|-------|-----|
| 1 | one-line statement of the claim | `path:line` | reviewer | **Fix now** | this PR | what in the code makes it true — name the symbol, quote the line |
| 2 | … | … | … | **Defer → #123** | #123 | true, but out of this branch's scope — and where it now lives |
| 3 | … | … | … | **Reject** | — | what in the code makes it false — the behaviour it misreads, the constraint it missed |

There are exactly three verdicts, and **every relevant finding gets an
owner**:

- **Fix now** — repaired on this branch, before the merge.
- **Defer → #N** — real, but not this branch's job. Deferral is only
  valid *with a destination*: file the issue as part of triage — with the
  `deferred-review` label, which is what step 4 lists it by — and put the
  number in the cell. Filing it is not a separate decision to ask
  about; it is what makes a deferral something other than a silent drop.
  An issue that changes recorded fixtures says so, so the re-record is
  planned rather than discovered later.
- **Reject** — not a real finding. Needs a technical argument, not a
  dismissal.

Rules for the table:

- Every open thread gets a row. No silent omissions, no "minor" bucket,
  and no fourth verdict — "valid but not here" without an issue number
  is the failure this taxonomy exists to prevent.
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
rewriting published history, `--admin`, deleting their data, or changing
something outside this repository. Naming such a step in the plan is not
the same as having permission for it.

Filing a deferral issue is NOT one of those steps, despite being visible
on GitHub. It records a finding the review already made, in the tracker
that review lives in, and refusing to file it without asking is what
turns a deferral into a silent drop — the failure the taxonomy above
exists to prevent. Judge by consequence, not by whether a network call
is involved.

### 2c. Reply and resolve, one thread at a time

```bash
./.claude/skills/pr-merge/threads.sh reply <comment-id> "Valid — fixed …"
./.claude/skills/pr-merge/threads.sh reply <comment-id> "Rejected — …"
./.claude/skills/pr-merge/threads.sh resolve <thread-id>
```

The reply carries the table's reason plus what was done. For a rejection,
give the evidence: the API's documented behaviour, the constraint the
suggestion would break, the reason the code is already correct.

Then re-run gates (`./.claude/skills/pr-commit/gates.sh`) and commit the
fixes.

### Rules

- **Never resolve a thread you have not read and replied to.** Resolving
  claims the comment was answered. There is no bulk-resolve verb for
  exactly this reason: a reviewer bot posts a fresh review on every push,
  so `list` grows while you work, and a blind loop will sweep up comments
  that arrived seconds ago and were never looked at.
- **Re-run `list` after pushing.** A new push means a new review.
- **A rejection is a technical argument, not a dismissal.** If it cannot
  be defended in two sentences against the actual code, it is a fix.
- **Never dismiss the review itself** (`gh pr review --dismiss`) to get
  past the block. Answer the comments; the bot re-reviews and clears its
  own verdict.
- **Ask before merging with `--admin`.** That overrides branch
  protection, which is the user's call and not the default path.

## 3. Merge

```bash
./.claude/skills/pr-merge/merge.sh
```

Squash-merges, deletes the remote branch, switches to main, pulls, and
removes the local branch.

## 4. Work the deferred queue

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
