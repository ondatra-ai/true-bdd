# Task automation — driving ClickUp tickets to merge unattended

Design record. Settled by interview; nothing here is implemented yet.

The goal is one number: **of ten tickets taken automatically, eight reach
`DONE` on the first attempt and two land in `FAILED`.** An attempt is one
`TO DO → DONE|FAILED` transition — retries *inside* a task (fixing a red
gate) do not count and are capped at 5. The lever on that number is not the
retry logic; it is how well the ticket was written before it was taken.

## 1. The loop

One Claude Code instance, one task at a time. There is no concurrency to
design around: the instance *is* the mutex.

```text
mandate granted
  └─ /new-task ──────────► next ticket bound, status TO DO → PROCESSING
       ├─ branch from main
       ├─ do the work
       ├─ scope check: actual diff vs expected_diff
       ├─ gates — only those the actual diff needs
       ├─ review (Spec axis), fix, retry ≤5
       ├─ merge ──────────► status DONE
       └─ /new-task (second-to-last step) ──► next ticket
  failure at any step ──► status FAILED + comment + PR left open
                          queue continues with the next ticket
```

`PROCESSING` spans exactly `/new-task` → merge. A ticket never returns to
`TO DO` on its own; only a human moves it back out of `FAILED`.
`COMPLETED` is a human state and the loop never sets it.

## 2. Mandate and cancellation

The mandate is what authorises merging without asking. It is granted when
the run was started by an automatic process, when the operator said "do it
all without me", or when the loop asked at the start and was told to run
unattended.

**Any message from the operator while the loop is running cancels the
mandate for the current ticket only.** That ticket is driven to a PR and
stops there; the operator merges it by hand, which is still a `DONE`. The
queue then continues automatically with the next ticket.

The distinction matters: revoking the mandate wholesale would stall the
queue on every "also look at this", and the operator would spend the day
re-arming it.

Cancellation must be checked *before every merge*, not once at start.

The mandate lives in `tmp/history-cursor/`, beside the per-session turn
cursor the history hook already keeps there. It records that a mandate
exists, which ticket is bound, and the mode.

**Not** beside `docs/history/hook-state`, which is the obvious-looking
choice and is wrong: `/new-task` deletes `hook-state`, and the loop calls
`/new-task` between every two tickets. A mandate stored there would die at
exactly the boundary it has to survive. `tmp/history-cursor/` outlives
`/new-task` by construction.

Write it as a **sibling** file (`<session8>.task.json`), not as extra keys
in `<session8>.json`. The cursor is rewritten wholesale on every Stop
(`cursorWrite` marshals a two-field struct and renames over the target), so
anything else stored in it would be erased several times a minute.

Nothing prunes that directory — it holds 345 files today, one per session
since August 10th. That is untidy but safe, because the name is keyed on
the session id: a mandate left by a dead session sits under a key no live
session will ever read. It follows that **cancellation must be an explicit
write**; nothing expires on its own, and within one session the file stands
until something overwrites it.

`scripts/merge` is a separate process and reads this file for one reason:
to know which row of §7 to apply. That is the whole of the "mode" it needs.

## 3. Ticket readiness

A ticket is taken only if `Good For Agent` is set and the status is
`TO DO`. Today a human sets that checkbox after eyeballing the fields;
the intent is to compute it once the shape below proves itself.

Readiness is defined by `true-bdd/ticket-schema.yaml` — the same
`*-schema.yaml` convention `lint-schemas.sh` already validates, so no new
top-level directory is needed. Required:

| field | why it is required |
| --- | --- |
| `scope` | `FILE` · `SERVICE` · `PROJECT` · `DOCUMENTATION` |
| `triage_score` | 1–10; orders the queue |
| `expected_diff` | non-empty glob list — see §4 |
| `verification` | a runnable command; this is what decides `DONE` |
| body | four headings: Why / What to change (`file:line`) / Verification / Context |

The schema is checked at three points, and the reaction hardens as the cost
of being wrong rises:

- **before creation** — missing fields are collected; it is still a draft.
- **after creation** — the ticket is created regardless, but an incomplete
  one is tagged `draft` and the queue predicate does not see it.
- **before taking** — refuse, comment what is missing, do not enter
  `PROCESSING`.

`PROJECT` scope means "not clear what this touches". That is a statement
that the ticket is not thought through, and it is the worst possible
candidate for an unattended run — the readiness check should treat it as
such rather than as merely "large".

## 4. `expected_diff` and the scope check

`expected_diff` is a glob list naming the blast radius the ticket expects:
one exact file for a single-function refactor, `./tests/**/*.go` for a
family, `./*` when the change really is repo-wide. Globs name **a
directory or a pattern, not an exact file** — that leaves the agent free to
add a test beside the code or split a file, while keeping the district
fixed.

The check runs **after the work is done, before the commit**, comparing the
actual diff against the globs. A mismatch gets one chance: the agent is
shown what it touched against what was declared and must either narrow to
scope or declare the ticket wrong and go to `FAILED`.

This is topological and needs no model. The semantic half — "the files are
right but this does more than was asked" — is the review's Spec axis (§6).
The two are complementary; neither replaces the other.

## 5. Gate selection

Measured on this repository, warm. Three steps are ~90% of the total:

| gate | cost | run when the diff touches |
| --- | --- | --- |
| `alint check` | 0.02s | **always** — it reads the whole file tree |
| `lint-claude.md.sh` | 0.33s | `CLAUDE.md` |
| `lint-schemas.sh` | 1.02s | `true-bdd/**`, `docs/{architecture,product}/**`, `docs/scenarios.yaml`, `scripts/cmd/yamlkey/**` |
| `go vet -tags bdd ./tests/...` | 1.01s | `tests/**` |
| bdd-web `TestScenarioCoverage` | 2.05s | `docs/scenarios.yaml`, `tests/bdd-web/**` |
| `lint-comments.sh` | 4.95s | any `*.go`, `*.sh`, `*.yaml` |
| `go build ./services/bdd-cli` | 1.77s | `services/bdd-cli/**` |
| `golangci-lint run` | **27.8s** | any `*.go` outside the fenced trees |
| `go test ./...` | **39–48s** | any `*.go` in the module |
| bdd-cli replay | **62.1s** | `services/bdd-cli/**`, `true-bdd/**`, `templates/**`, `docs/scenarios.yaml`, `tests/bdd-cli/**`, `tests/libraries/{bddgo,runner,aiproxy}/**` |

Two properties are not optional:

- **Fail-safe.** A path matching no rule runs everything. Without it a new
  directory will one day slip through unchecked, and nothing will say so.
- **One selector, both places.** `gates.sh` and `.github/workflows/ci.yml`
  must run the same thing. They already drift: CI runs neither
  `alint check` nor `lint-comments.sh`. A gate that exists only on one
  machine is not deterministic enforcement.

What this buys, honestly:

- a `DOCUMENTATION` ticket: **~2s instead of ~140s**;
- a ticket in `scripts/`: **~70s** — replay drops out entirely;
- a ticket in `services/bdd-cli/**`: **nothing** — replay rebuilds the
  binary from that tree, so it cannot be skipped.

The saving is concentrated in documentation and tooling. That is worth
knowing when choosing which tickets to automate first.

## 6. Review

`.claude/skills/code-review` runs two axes as parallel sub-agents:
**Standards** (documented repo standards plus a fixed Fowler smell
baseline) and **Spec** (what the spec asked for and is missing, what is
present but unasked, what looks wrong).

**In automatic runs only the Spec axis runs.** This repository documents
its standards in `CLAUDE.md` and `.claude/rules/*.md`, and enforces most of
them mechanically — `golangci-lint` with ~90 linters plus three lint
scripts. The skill itself says to skip what tooling already enforces, which
leaves the Standards axis with little to say here. Manual runs keep both.

Three things must be supplied so the skill never stops to ask:

1. the fixed point — always `main`;
2. the spec — the ClickUp ticket body, passed as an argument so its lookup
   never reaches "ask the user where the spec is";
3. `docs/agents/issue-tracker.md`, which the skill's preamble requires and
   which **does not exist in this repository today**.

The skill deliberately produces prose and refuses to rank across axes, so
the verdict is the loop's to make:

- Spec finds something missing or wrong → **fix and retry** (cap 5).
- Spec finds something that was not asked for → **`FAILED`**. This is the
  same class as a mismatched `expected_diff`: the ticket is not describing
  the work, and that is a human's call.
- Standards findings are logged and block nothing.

## 7. Triage and postmortem thresholds

CodeRabbit keeps reviewing and approving. It is not dropped: `main`'s
ruleset requires an approving review, and `@coderabbitai approve` is what
supplies it — without it every merge would need `--admin`, discarding the
only external check. For a ticket touching nothing under `tests/**` it
answers "No files to review" in about 30 seconds, so it is nearly free.

| | triage within a round | postmortem |
| --- | --- | --- |
| **manual** | 1–5 skip · 6–8 ticket · 9–10 fix now | tickets 6–10 |
| **automatic** | 1–8 skip · 9–10 ticket | tickets 9–10 |

The manual row is what the code does today (`ticketFloor = 6`,
`fixFloor = 9`), so only the automatic row is new.

The postmortem stays on in automatic runs, at a high floor. It costs one
`claude -p` and it is the only step that notices the loop degrading; at a
floor of 9 it will usually stay silent.

## 8. Binding a session to a ticket

The machinery already exists; only the wiring is missing.

`/new-task` is `new-task.md`, whose body is `` !`new-task.sh` ``. The `!`
prefix runs the script and **injects its stdout into the prompt as
context** — an injection point already in use. `new-task.sh` deletes
`docs/history/hook-state` and nothing else.

The `/new-task` prompt itself is filtered by the history hook
(`handlers.go`: `fields[0] == "/new-task"` → return), so no file opens
then. The next real prompt hits `logPrompt` with `loadCurrent() == ""` and
opens the task file. **That is the only moment the system knows a new task
has begun, and the hook has the first prompt's text in hand.**

Two injection points follow, one per mode:

- **under a mandate** — `new-task.sh` prints the next ticket (id, title,
  body, `expected_diff`) and it lands as context. The model starts working;
  nobody is asked anything.
- **without a mandate** — the hook injects `additionalContext` at
  `filename == ""`: "no ticket bound — ask whether to create one or take an
  existing one, and do not let up". `settings.json` already proves the
  pattern: the commit reminder is exactly this.

The binding lives beside `hook-state`, written through the same atomic
`saveCurrent` path — same directory, same lifecycle, same question ("what
are we on now"). Its lifetime is `[first prompt after /new-task, merge]`,
which coincides with `PROCESSING` exactly.

One gap to close: `new-task.md` invokes the script **with no arguments**,
so `/new-task 86cb8hjf7` passes the id nowhere. It needs `$ARGUMENTS`.

## 9. A run that dies mid-ticket

Nothing recovers it. A ticket left in `PROCESSING` by a dead run means
something went wrong enough that repairing it automatically costs more than
abandoning it. The loop stops and waits for a human.

This is deliberate and it is the same principle the merge loop already
holds: a stop is a stop, and the state is left exactly as the failure left
it so a person can see what happened. There is no timeout, no sweep, no
"probably safe to retake".

## 10. Settled and not to be revisited

- `Good For Agent` is set by hand. Computing it is out of scope until the
  shape above has proven itself in use.
- `TO DO`, `PROCESSING`, `DONE`, `FAILED` and `COMPLETED` all exist as
  statuses today; `done` and `failed` were confirmed against the board.
