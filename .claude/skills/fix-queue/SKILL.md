---
name: fix-queue
description: Work the deferred review backlog — ClickUp tasks tagged `fix-now`, filed by the pr-merge triage. Takes the oldest one, fixes it on its own branch and opens a PR, then stops; merging is a separate, explicitly-commanded step. Use when the user says "work the fix queue", "work the backlog", "fix the deferred findings", "take the next fix-now ticket", or names a ticket from that queue.
---

# Fix Queue

The `pr-merge` loop fixes only findings scored 9-10 and files everything
scored 6-8 to ClickUp tagged `fix-now`. This skill is the other half: it
turns that backlog back into merged changes, **one ticket per PR**.

**One ticket, then stop.** Each ticket is its own PR and its own merge, and
each merge needs its own explicit user command. A single "work the queue"
does not authorise draining it — that is the same rule that keeps `pr-merge`
from opening PRs on its own momentum.

## 1. Read the queue

```bash
python3 ./.claude/skills/lib/clickup.py list --tag fix-now
```

Oldest first — the sort is explicit because a queue worked newest-first
leaves the oldest finding permanently last. It reaches ClickUp through the
MCP server inherited by `claude -p`, not through a REST token, so there is
no credential to be missing and no fallback path to take; a non-zero exit
means the call itself failed and the output says how.

If the queue is empty, say so and stop.

## 2. Take the first ticket

Read it in full — `getTask` returns the untruncated description, and these
tickets are written to be picked up cold: **Why**, **What to change** with
`file:line`, **Verification**, **Context**.

**Re-validate before fixing.** The ticket records what a reviewer claimed
when it was filed, which may be several merges ago. Read the file it names.
Three outcomes:

- **Still valid** → fix it.
- **Already fixed** → close the ticket with a comment naming the commit that
  fixed it. Do not open a PR.
- **Wrong about the code** → close it with the technical argument, the same
  standard `pr-merge` holds a rejection to. Do not open a PR.

Only the first outcome continues to step 3.

## 3. Fix it on its own branch

Derive the slug yourself — lowercase, `[a-z0-9-]` only — rather than
pasting the ticket title, which is text ClickUp accepted and git may not:

```bash
git checkout main && git pull origin main
SLUG=$(printf '%s' "<ticket title>" | tr '[:upper:]' '[:lower:]' \
       | sed -e 's/[^a-z0-9]\+/-/g' -e 's/^-//' -e 's/-$//' | cut -c1-50)
git check-ref-format --branch "fix/$SLUG"   # refuses anything git cannot name
git checkout -b "fix/$SLUG"
```

Apply the change. Keep it to what the ticket describes — a fix that grows
past its ticket is a new decision, so **stop and report** rather than
widening it.

Then the ordinary loop: the `pr-commit` skill (gates, recordings scan, local
review round, doc-universe, memory, commit, push, PR).

Put the ClickUp URL in the PR body, so the merge and the ticket are linked
from both ends.

## 4. Land it

Invoke the `pr-merge` skill from step 1 — **on the user's command**, not
automatically. It is the same bounded loop: local round, at most two fixing
rounds, then resolve and merge.

## 5. Close the ticket, report, stop

Set the ClickUp task to complete with a comment naming the merge commit.
Then report: what landed, what remains in the queue, and stop.

Do not take the next ticket. Wait to be asked.

## Rules

- **One ticket per PR**, unless two share a single root cause — then one PR
  fixes the cause and closes both.
- **Re-validate first.** A stale ticket fixed anyway is churn; a wrong
  ticket fixed anyway is a regression.
- **Stop and report** when a fix turns out larger than its ticket, when it
  needs a decision that is the user's, or when the queue is empty.
- A ticket that forces re-recording fixtures pays that cost in its own PR
  and says so in the PR body.
- Tickets tagged `merge-improvements` are **not** this queue. Those are
  post-mortem findings about the merge process itself; they change tooling,
  not product code, and are worked deliberately rather than in sequence.
