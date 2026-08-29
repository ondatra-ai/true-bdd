# ClickUp tickets to create

List: `901523097822`   Tag: `merge-improvements`   Source: PR #81

5 ticket(s). One ClickUp task per `## ` heading below.

---

## 1. Dedupe postmortem proposals against the open merge-improvements queue before filing

### Why

The merge postmortem raised this on PR #81; triage scored it **9/10**.

> The 08:58 run already filed 9 tickets and this run re-derived at least 5 of them verbatim, because nothing asks ClickUp what is already open.

### What to change

`.claude/skills/pr-merge/merge.py:1402`

## What the transcript shows

`tmp/merge/filed.json` records 9 tickets filed at **08:58** for PR #84 under `--tag merge-improvements`:

| id | title |
|---|---|
| 86cb8rmdx | Feed the postmortem the merge script's own run log, not the session's chat |
| 86cb8rmeu | Bound `history_extract` to the run window so it stops sweeping in other runs' turns |
| 86cb8rmfq | Stop rendering postmortem tickets as CodeRabbit findings deferred from a review round |
| 86cb8rmgf | Diff the postmortem's worktree check against a snapshot |
| 86cb8rmh7 | Consolidate SKILL.md: background invocation, PID, log path, stops-only monitor, on-STOP playbook |
| 86cb8rmmw | Write the filed-ticket record per tag |
| 86cb8rmp7 | Skip the postmortem when the run has nothing to diagnose |
| 86cb8rmqb | Poll before sleeping and back off, instead of a flat 30s first check |
| 86cb8rmr7 | Cap and de-noise the recent-commits style reference in `staged_context` |

Working this run's transcript from scratch produced **5 of those 9 again** — the `history_extract` run-window bound, the `render()` provenance fix, the SKILL.md consolidation, the poll-before-sleep change, and the `staged_context` cap. The mechanism is visible in the extract itself: `tmp/merge/history-extract.md:119` is the **08:58 `@clickup` turn**, whose embedded document is the previous postmortem's ticket list. This run read the last run's proposals and re-proposed them.

At 9 tickets per run this compounds: every merge adds a fresh copy of the same backlog for `fix-queue` to work through.

## The change

In `postmortem()`, between `parse_json_array` (line 1397) and `save` (line 1403):

1. Run `clickup.py list --tag merge-improvements` (see the companion clickup.py ticket for the machine-readable output) to get the **open** titles.
2. Inject them into `POSTMORTEM_PROMPT` as an `ALREADY OPEN — do not propose these again` block, so the model spends its turn on what is new.
3. After parsing, drop proposals whose title matches an open one case-insensitively, and `log()` each drop by name — a silent drop would read as "the run found nothing new".
4. If everything was dropped, `log("the postmortem proposed nothing not already open")` and file nothing.

Cost is one extra headless turn with `LIST_TOOLS`; it replaces 9 duplicate ClickUp tasks.

### Verification

```bash
go run ./scripts/cmd/gates run
```

### Context

Reviewer severity `postmortem`, source `postmortem`.

`Triage Score`, `Triage Date` and `Triage Commit` say what this was
judged to be worth, when, and against which commit. `clickup triage`
re-reads the oldest of those against HEAD and either refreshes this body
or retires the ticket, so a score here is never older than its stamp.

---

## 2. Stop on an unrecognized CodeRabbit acknowledgement instead of waiting out ACK_BUDGET

### Why

The merge postmortem raised this on PR #81; triage scored it **8/10**.

> The bot answered in 10 seconds with a shape the script did not know, and the script waited the full 300s and then blamed an outage that never happened.

### What to change

`.claude/skills/pr-merge/merge.py:329`

## What the transcript shows

The run stopped with:

> Bot never acknowledged within 300s. Nothing merged, PR #81 still open.

The agent then found: *"The bot **did** reply — 10 seconds in, with a ⚠️, then edited it 16s later."* The body was `⚠️ Action not completed / No files to review.`, a third shape `await_acknowledgement` did not match. So **290 of the 300 seconds bought nothing**, and the stop text pointed at the wrong thing entirely — `die()` at line 290 says *"the bot may be down"* about a bot that had answered in ten seconds. Diagnosing it took a manual read of the PR's comments.

That specific shape is now handled (`ACK_NOTHING_TO_REVIEW`, line 235). The **generic** failure is not: a fourth wording costs the same 300s and the same wrong blame.

## The change

In `await_acknowledgement` (lines 329-342), keep every bot reply seen after the baseline. Then:

- If a reply contains `Action not completed` but matches neither `ACK_NOTHING_TO_REVIEW` nor `ACK_RATE_LIMITED`, that is a definitive negative answer — but merge.py already records that **CodeRabbit edits acks in place** (PR #77, lines 361-367), and this run saw an edit at +16s. So wait one more `POLL`, re-read that comment by id via `comment_body()`, and only then return a new `"unknown"` verdict carrying the final body.
- On `"unknown"`, `request_review` dies with the comment body and its URL verbatim — the next unrecognized shape is then named in the stop message rather than dug out by hand.
- On the `"silent"` timeout path, change `die()` at lines 289-291 to print the first line of **every** bot reply seen in the window (or `"no bot comment at all"`). "The bot may be down" is then a conclusion drawn from evidence rather than a guess.

Saves ~290s per occurrence plus the manual investigation.

### Verification

```bash
go run ./scripts/cmd/gates run
```

### Context

Reviewer severity `postmortem`, source `postmortem`.

`Triage Score`, `Triage Date` and `Triage Commit` say what this was
judged to be worth, when, and against which commit. `clickup triage`
re-reads the oldest of those against HEAD and either refreshes this body
or retires the ticket, so a score here is never older than its stamp.

---

## 3. Give clickup.py list a machine-readable mode so callers can dedupe against the queue

### Why

The merge postmortem raised this on PR #81; triage scored it **6/10**.

> cmd_list only prints TSV to stdout, so merge.py cannot consume the open queue without re-parsing print formatting.

### What to change

`.claude/skills/lib/clickup.py:198`

## What the transcript shows

The postmortem filed 9 tickets at 08:58 and this run re-derived 5 of them. The fix is for `postmortem()` to ask what is already open — and `clickup.py` already knows how: `cmd_list` (line 198) runs exactly the right query, with the right tag filter and an explicit open-only, oldest-first contract.

But it only `print`s `id\tstatus\tname` (line 213) and returns 0. A caller wanting the titles has to split on tabs and hope no name contains one, and `(queue empty)` is indistinguishable from a task literally named that. `cmd_file` has the same shape problem solved properly one function up — it writes `FILED` (line 174-176) as JSON.

## The change

Give `cmd_list` the same treatment as `cmd_file`:

1. Add `OPEN_QUEUE = "tmp/merge/open-%s.json"` alongside `TICKETS_MD` and `FILED` (line 32-33).
2. After `parse_json_array`, write the parsed rows to `OPEN_QUEUE % args.tag` before printing, and print the path — same pattern, same directory, so a caller reads a file rather than scraping stdout.
3. Keep the human TSV print unchanged; `fix-queue` invokes this by path and reads it, and that is the skill's contract.

No new flag and no behaviour change for existing callers — just an artifact that `merge.py` can open.

### Verification

```bash
go run ./scripts/cmd/gates run
```

### Context

Reviewer severity `postmortem`, source `postmortem`.

`Triage Score`, `Triage Date` and `Triage Commit` say what this was
judged to be worth, when, and against which commit. `clickup triage`
re-reads the oldest of those against HEAD and either refreshes this body
or retires the ticket, so a score here is never older than its stamp.

---

## 4. Re-read the acknowledgement before accepting nothing-to-review into REVIEWED_THIS_RUN

### Why

The merge postmortem raised this on PR #81; triage scored it **6/10**.

> The terminal verdict is committed from the first body seen, on a comment the script's own docstring says CodeRabbit rewrites in place.

### What to change

`.claude/skills/pr-merge/merge.py:334`

## What the transcript shows

The run's own diagnosis records the edit: *"The bot did reply — 10 seconds in, with a ⚠️, **then edited it 16s later**."* `await_review` already defends against this and says why (lines 361-367): on PR #77 comment 5330633865 was posted at 15:48:15 saying the review was triggered and rewritten at 15:48:47 to say the quota was spent, and reading it once cost a full 900s wait.

`await_acknowledgement` has no such defence. At line 334 the first body containing `No files to review` returns `"nothing-to-review"`, and `request_review` (lines 295-298) immediately does `REVIEWED_THIS_RUN.add(head_sha())` and returns True.

The consequence is not a wasted wait but a false claim. `REVIEWED_THIS_RUN` is the guard that lets `merge()` approve a commit (line 1242), and it exists specifically because PR #76 stamped `14e327a`, a commit no review was ever posted against. Accepting it from a body that may still be rewritten to `Review rate limited` would re-open that hole from the other side.

The run's own poll cadence happened to hide this: `POLL` is 30s, so the first look at t=30 already saw the edited body.

## The change

In `await_acknowledgement`, before returning either terminal verdict (`nothing-to-review` at line 336, `accepted` at line 341), sleep one `POLL` and re-read the comment by id with `comment_body()`. Classify the **re-read** body. Cost is 30s on the ack path; it buys the same in-place-edit guarantee `await_review` already has, on the branch that feeds the approval guard.

### Verification

```bash
go run ./scripts/cmd/gates run
```

### Context

Reviewer severity `postmortem`, source `postmortem`.

`Triage Score`, `Triage Date` and `Triage Commit` say what this was
judged to be worth, when, and against which commit. `clickup triage`
re-reads the oldest of those against HEAD and either refreshes this body
or retires the ticket, so a score here is never older than its stamp.

---

## 5. Check reviewDecision before posting @coderabbitai approve, not only after

### Why

The merge postmortem raised this on PR #81; triage scored it **4/10**.

> On the nothing-to-review path CodeRabbit has already posted APPROVED, so the approve comment and its first 30s poll buy nothing.

### What to change

`.claude/skills/pr-merge/merge.py:1259`

## What the transcript shows

The run's diagnosis records that on the nothing-to-review path *"CodeRabbit still posted `APPROVED` and its check reads pass — 'Review completed'; `gates` is green too."* The PR was already `APPROVED` before `merge()` ran.

`merge()` posts `@coderabbitai approve` unconditionally at line 1260, then enters a loop (1262-1270) that sleeps `POLL` = 30s **before** its first `reviewDecision` read. So the run spent one bot comment and ≥30s establishing a state that already held. The monitor timestamps bracket it: `requesting approval` at 10:28:49, next turn 10:28:54.

This is not ticket 86cb8rmqb (poll-before-sleep across every wait loop) — the point here is skipping the request entirely, not reading the answer sooner.

## The change

At line 1259, before `log("requesting approval")`, read `reviewDecision` once. If it is already `APPROVED`, `log(f"#{PR} is already APPROVED — not asking again")` and fall through to the merge. Otherwise post the comment and poll as today.

`resolve_conversations` has already answered and resolved every thread by this point, so the approve comment's side effect of resolving threads is not needed either. Saves ~30-60s and one comment on every clean PR.

### Verification

```bash
go run ./scripts/cmd/gates run
```

### Context

Reviewer severity `postmortem`, source `postmortem`.

`Triage Score`, `Triage Date` and `Triage Commit` say what this was
judged to be worth, when, and against which commit. `clickup triage`
re-reads the oldest of those against HEAD and either refreshes this body
or retires the ticket, so a score here is never older than its stamp.
