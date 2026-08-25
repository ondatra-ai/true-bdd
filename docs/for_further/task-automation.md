# Task automation — driving ClickUp tickets to merge unattended

Design record, and now also a description: §1–§11 are built and gated except
where a line says otherwise. Where the interview's answer and the code differ,
the code won and the section says why.

The goal is one number: **of ten tickets taken automatically, eight reach
`DONE` on the first attempt and two land in `FAILED`.** An attempt is one
`TO DO → DONE|FAILED` transition — retries *inside* a task (fixing a red
gate) do not count and are capped at 5. The lever on that number is not the
retry logic; it is how well the ticket was written before it was taken.

## 1. Division of labour

Two words collided all through the design and are now fixed (`CONTEXT.md`):
a **Ticket** is the ClickUp unit; a **Task** is the session unit, one history
file under `docs/history/`. `/task-start` starts a Task and binds a Ticket to
it. One Task, one Ticket, one branch, one PR.

The whole design is one split: **the `/task-*` family owns state, `task-handle`
owns the work, `task-loop` owns only the queue.** Everything else follows.

| Component | Owns | Does not |
| --- | --- | --- |
| a human | everything `task-loop` and `task-handle` own, when driving by hand | — |
| `task-loop` | the queue: the predicate, the ordering, iteration, the tally | anything inside one Ticket |
| `task-handle` | one Ticket end to end — grooming to §4, the work, the scope check, gates, review, retries ≤5, **deciding** `DONE` vs `FAILED`, the mandate | touch session history; write a status itself |
| `/task-start` | rolling session history; the Ticket's existence (id given — verify; none — interview and create); the binding; `TO DO → PROCESSING` | groom, branch, work, or check anything about the previous Task |
| `/task-done` | `PROCESSING → DONE`; clearing the binding | decide whether the work deserves it |
| `/task-fail` | `PROCESSING → FAILED` plus the reason as a comment; clearing the binding | decide whether the work deserves it |
| `pr-commit` | gates, recordings scan, doc universe, memory, **the branch**, commit, push, PR | — |
| `pr-merge` | CodeRabbit rounds, triage, fixes, approval, the merge, **the return to `main`** | set `DONE` |

Two of those cells are already true of the code and were briefly mistaken for
new requirements. `commit.sh` cuts the branch itself when it finds the
checkout on `main`, naming it from the staged diff via `claude -p`; and the
merge ends in `git checkout main` (`scripts/merge/land.go`). Neither belongs
to `task-handle` and neither needs writing.

**The modes are binary.** Either the loop runs and drives every step, or a
human drives every step. There is no half-attended mode in which a person
steps into the middle of an automatic run: §3's cancellation is not that — it
hands *one finished ticket* to a human to merge, and the loop carries on.

**The `/task-*` family decides nothing, and nothing else writes a status.**
Each of the three is one deterministic transition. Whether a Ticket is worth
taking, whether the work is good enough to merge, whether the previous Task is
finished, whether the tree is clean — none of that is theirs. It belongs to
whoever chose to call them, which is `task-handle` or a human. Splitting it
that way is what makes the manual path identical to the automatic one: the
same three transitions, typed instead of invoked.

## 2. The loop

One Claude Code instance, one Ticket at a time. There is no concurrency to
design around: the instance *is* the mutex.

```text
mandate granted
  └─ task-loop ────────► take the top-scoring ready Ticket
     └─ task-handle <id>
       ├─ groom it to §4, or refuse it and take the next
       ├─ /task-start <id> ─► history rolled, Ticket bound, TO DO → PROCESSING
       ├─ do the work
       ├─ scope check: actual diff vs Expected Changes
       ├─ pr-commit ──────► gates the diff needs, branch, commit, push, PR
       ├─ review (Spec axis), fix, retry ≤5
       ├─ pr-merge ───────► merged, checkout main
       └─ /task-done ─────► PROCESSING → DONE, binding cleared
  decline at any step ──► /task-fail ─► FAILED + comment, PR left open
                          the loop continues with the next Ticket
```

`task-loop` iterates; `/task-start` hands off to nothing. The first draft
had the recursion the other way round — the merge calling `/new-task` as its
second-to-last step to pass the baton — and that is gone. The baton belongs to
the component that owns the queue, and a merge that starts the next ticket
cannot be told apart from a merge that merely finished this one.

`PROCESSING` spans exactly `/task-start` → `/task-done`|`/task-fail`. A ticket
never returns to `TO DO` on its own; only a human moves it back out of
`FAILED`. `COMPLETED` is a human state and the loop never sets it.

**Decline** is the loop refusing to merge on the merits: review found work the
ticket did not ask for (§7), `Expected Changes` did not converge after its one
chance (§5), or the gates were still red after five retries. A decline ends in
`FAILED`. A cancelled mandate (§3) is *not* a decline — there the work is
sound and only the authority to merge it was withdrawn, so the human's merge
makes it `DONE`.

## 3. Mandate and cancellation

The mandate is what authorises merging without asking. It is granted when the
run was started by an automatic process, when the operator said "do it all
without me", or when the loop asked at the start and was told to run
unattended.

**Any message from the operator while the loop is running cancels the mandate
for the current ticket only.** That ticket is driven to a PR and stops there;
the operator merges it by hand and runs `/task-done`, which is still a `DONE`.
The queue then continues automatically with the next ticket.

The distinction matters: revoking the mandate wholesale would stall the queue
on every "also look at this", and the operator would spend the day re-arming
it.

Cancellation must be checked *before every merge*, not once at start.

The mandate is `tmp/history-cursor/mandate.json` — package `scripts/mandate` —
holding the id of the Ticket it was stamped for and nothing else.

**Not** beside `docs/history/hook-state`, which is the obvious-looking choice
and is wrong: `/task-start` deletes `hook-state` and clears the binding once
per Ticket, and a mandate spans the whole run. Stored there it would die at
exactly the boundary it has to survive. `tmp/history-cursor/` outlives
`/task-start` by construction and nothing prunes it.

**A sibling file, not extra keys in `<session8>.json`.** The cursor is
rewritten wholesale on every Stop (`cursorWrite` marshals a two-field struct
and renames over the target), so anything stored in it would be erased several
times a minute.

The interview settled on keying the file by session id, so that a mandate left
by a dead session sat under a key no live session would read. That is
unbuildable: a skill has no way to learn its session id — the hook is handed
one, `${CLAUDE_*}` does not carry it. **Keying on the bound Ticket does the
same job better.** `mandate.Active` honours the file only while
`docs/history/bound-ticket` names the same Ticket, and `/task-done` and
`/task-fail` clear that binding — so the mandate is live for exactly the
window `task-handle` merges in, and a file left by a dead run matches nothing.
`task-handle` re-stamps at every Ticket.

One consequence to expect rather than debug: a `FAILED` Ticket leaves
`mandate.json` on disk, inert, until the next stamp overwrites it. Cancellation
is still an **explicit write** (`history.sh unmandate`); nothing expires on its
own.

`scripts/merge` is a separate process and reads this for one reason: `Start`
picks `automatic` or `manual` from `mandate.Active`, which is the whole of the
"mode" it needs (§8).

## 4. Ticket readiness

A ticket is taken only if `Good For Agent` is set and the status is `TO DO`.
Today a human sets that checkbox after eyeballing the fields; the intent is to
compute it once the shape below proves itself.

Required:

| field | why it is required | on the board |
| --- | --- | --- |
| `Scope` | `FILE` · `SERVICE` · `PROJECT` · `DOCUMENTATION` | labels, all four options |
| `Triage Score` | 1–10; orders the queue | drop_down 1–10 |
| `Good For Agent` | the queue predicate | checkbox |
| `Expected Changes` | non-empty glob list — see §5 | text |
| `verification` | a runnable command; this is what decides `DONE` | **not a field** — it is the body's `### Verification` |
| body | four headings: Why / What to change (`file:line`) / Verification / Context | free text |

The field is named **`Expected Changes`** on the board; the design's working
name for it was `expected_diff`, and §5 still describes its contents. There is
no `verification` field and there should not be: the body already has a
`### Verification` heading, and two homes for one fact is how they diverge.

Readiness itself is defined by
`.claude/skills/task-handle/ticket-schema.yaml`. An earlier draft put it at
`true-bdd/ticket-schema.yaml` on the grounds that `lint-schemas.sh` already
validates that convention — which is exactly backwards: that script **fails**
any `true-bdd/*-schema.yaml` whose key names no `documents.<key>`, and a
Ticket has no document file to name. `true-bdd/` is also the host-facing
engine config, and a host does not inherit this repository's ClickUp
workflow. Beside its only consumer is where it belongs.

The schema is checked at three points, and the reaction hardens as the cost of
being wrong rises:

- **before creation** — missing fields are collected; it is still a draft.
- **after creation** — the ticket is created regardless, but an incomplete one
  is tagged `draft` and the queue predicate does not see it.
- **before taking** — refuse, comment what is missing, do not enter
  `PROCESSING`.

**Before taking** is `task-handle`'s, or the human's when driving by hand.
Called with an id, `/task-start` re-runs none of it — the Ticket is already
ready, and a refusal from it would be a second opinion nobody asked for.

The other two are where the no-id branch of §9 lives: when `/task-start`
interviews the operator and creates a Ticket, that interview **is** the
before-creation checkpoint, and the `draft` tag is the after-creation one.
This is the only sense in which `/task-start` touches readiness — it collects
the fields it is about to write, which is not the same as judging fields
someone else wrote.

`PROJECT` scope means "not clear what this touches". That is a statement that
the ticket is not thought through, and it is the worst possible candidate for
an unattended run — the readiness check should treat it as such rather than as
merely "large".

**There is one queue.** The `fix-queue` skill that drained a second one is
deleted: two kinds of ticket existed only because two skills drained them.

The `fix-now` tag survives, demoted to provenance. `scripts/clickup file`
writes the four headings and a tag and no field in the table above, so a
triage-filed Ticket is **not** ready — and completing it is exactly the
judgement §11 keeps in a human's hands, since `Good For Agent` is set by hand.
A deferred finding therefore lands as an ordinary `TO DO` Ticket that a person
finishes and admits to the queue. Teaching `clickup file` to fill the fields
would shorten that path; it would not remove the human from it.

## 5. `Expected Changes` and the scope check

`Expected Changes` is a glob list naming the blast radius the ticket expects: one
exact file for a single-function refactor, `./tests/**/*.go` for a family,
`./*` when the change really is repo-wide. Globs name **a directory or a
pattern, not an exact file** — that leaves the agent free to add a test beside
the code or split a file, while keeping the district fixed.

The check runs **after the work is done, before the commit**, comparing the
actual diff against the globs. A mismatch gets one chance: the agent is shown
what it touched against what was declared and must either narrow to scope or
declare the ticket wrong and go to `FAILED`.

This is topological and needs no model. The semantic half — "the files are
right but this does more than was asked" — is the review's Spec axis (§7). The
two are complementary; neither replaces the other.

## 6. Gate selection

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
- **Selection is local; CI keeps running everything.** What is being bought
  here is the loop's wall-clock, and CI's is not the loop's to spend. An
  exhaustive CI means no gate is ever skipped in the end, which costs nothing
  and deletes the entire class of "the selector was wrong" failures.

The first draft demanded one selector in both places instead, on the grounds
that a gate existing on one machine only is not deterministic enforcement.
That argument is about a gate *existing*, not about it being *chosen*, and it
survives in the weaker form that actually matters: the **list** of gates must
not drift. It already has — `gates.sh` runs `lint-comments.sh` and CI does
not. (The same draft claimed CI skips `alint check`. It does not, under the
step named "Lint repository shape".)

So: the glob-to-gate table above is data in Go, carrying the fail-safe rule;
`gates.sh` becomes a thin `go run ./scripts/cmd/gates run [--changed <base>]`;
and a unit test parses `.github/workflows/ci.yml` and fails when the set of
gates in the table and the set of steps in the `gates` job disagree. That is
cheaper than restructuring CI, and it is the shape this repository already
uses for exactly this problem — `TestScenarioCoverage`,
`TestFixtureTreesArePaired`.

What this buys, honestly:

- a `DOCUMENTATION` ticket: **~2s instead of ~140s**;
- a ticket in `scripts/`: **~70s** — replay drops out entirely;
- a ticket in `services/bdd-cli/**`: **nothing** — replay rebuilds the binary
  from that tree, so it cannot be skipped.

The saving is concentrated in documentation and tooling. That is worth knowing
when choosing which tickets to automate first.

## 7. Review

`.claude/skills/code-review` runs two axes as parallel sub-agents:
**Standards** (documented repo standards plus a fixed Fowler smell baseline)
and **Spec** (what the spec asked for and is missing, what is present but
unasked, what looks wrong).

**In automatic runs only the Spec axis runs.** This repository documents its
standards in `CLAUDE.md` and `.claude/rules/*.md`, and enforces most of them
mechanically — `golangci-lint` with ~90 linters plus three lint scripts. The
skill itself says to skip what tooling already enforces, which leaves the
Standards axis with little to say here. Manual runs keep both.

Three things must be supplied so the skill never stops to ask:

1. the fixed point — always `main`;
2. the spec — the ClickUp ticket body, passed as an argument so its lookup
   never reaches "ask the user where the spec is";
3. `docs/agents/issue-tracker.md`, which the skill's preamble requires and
   which **does not exist in this repository today**.

The skill deliberately produces prose and refuses to rank across axes, so the
verdict is the loop's to make:

- Spec finds something missing or wrong → **fix and retry** (cap 5).
- Spec finds something that was not asked for → **`FAILED`**. This is the same
  class as a mismatched `Expected Changes`: the ticket is not describing the
  work, and that is a human's call.
- Standards findings are logged and block nothing.

### `sync-doc-universe` under a mandate

`pr-commit`'s third step audits the declared documents against the doc
universe and **asks the user about every inconsistency** — its own rule is
"never resolve an inconsistency without asking, even an obvious one". Under a
mandate there is nobody to ask, and the skill has no notion of a recommended
answer to fall back on, so it gains an argument that switches it to resolving
each inconsistency by a fixed rule:

- **the document is truth, the universe is updated.** The universe
  *describes*; the ticket just changed the thing being described.
- **`doc-universe.md` is truth, the `.html` is updated** — the html is its
  rendering, and the md is where content is written.
- a `lint-schemas.sh` failure is not an inconsistency to resolve at all. It is
  a red gate, and it already fails `gates.sh`.

Enforcement is not left to good intentions. Both loop skills declare
`disallowed-tools: AskUserQuestion`, which the skills documentation names for
precisely this case — "autonomous skills that should never call certain tools,
such as `AskUserQuestion` for a background loop".

## 8. Triage and postmortem thresholds

CodeRabbit keeps reviewing and approving. It is not dropped: `main`'s ruleset
requires an approving review, and `@coderabbitai approve` is what supplies it
— without it every merge would need `--admin`, discarding the only external
check. For a ticket touching nothing under `tests/**` it answers "No files to
review" in about 30 seconds, so it is nearly free.

| | triage within a round | postmortem |
| --- | --- | --- |
| **manual** | 1–5 skip · 6–8 ticket · 9–10 fix now | tickets 6–10 |
| **automatic** | 1–8 skip · 9–10 ticket | tickets 9–10 |

Both rows are `merge.Floors` values in `scripts/merge/run.go`, chosen once in
`Start` from `mandate.Active`. The automatic row's `Fix` floor is 11 — above
the scale, so nothing is fixed inline: a fix nobody reviewed is what the
Ticket was for. Whatever the triage files lands in the one queue, waiting for
a human to complete it and check `Good For Agent` (§4).

The postmortem stays on in automatic runs, at a high floor. It costs one
`claude -p` and it is the only step that notices the loop degrading; at a
floor of 9 it will usually stay silent.

## 9. The `/task-*` family

Three skills, one per transition, and between them the only writers of a
Ticket's status:

| skill | transition | argument |
| --- | --- | --- |
| `/task-start` | `TO DO → PROCESSING`, after making sure a Ticket exists and binding it | a Ticket id, or none |
| `/task-done` | `PROCESSING → DONE`, binding cleared | none — it acts on the bound Ticket |
| `/task-fail` | `PROCESSING → FAILED` plus the reason as a comment, binding cleared | the reason |

Neither `/task-done` nor `/task-fail` takes an id: the bound Ticket is the
only one they could mean, and letting them name a different one would reopen
the possibility of closing a Ticket nobody worked on. Both clear the binding,
which is what makes its lifetime exactly `[/task-start, /task-done|/task-fail]`
and therefore exactly `PROCESSING`.

Splitting the two terminal transitions out of `task-handle` costs nothing and
buys the thing §1 is built on: the manual path and the automatic path perform
the *same three writes*, so a human picking up an abandoned run (§10) closes
the Ticket the same way the loop would have.

`/new-task` becomes **`/task-start`** and moves from
`.claude/commands/new-task.{md,sh}` to `.claude/skills/task-start/`; the other
two are new, `.claude/skills/task-done/` and `.claude/skills/task-fail/`.
Command and skill are one mechanism — the documentation is explicit that
"custom commands have been merged into skills", and that
`.claude/commands/deploy.md` and `.claude/skills/deploy/SKILL.md` "both create
`/deploy` and work the same way". What the skill form adds is what this needs:
a directory to keep the script beside the prose, as `pr-commit` keeps
`gates.sh`; `disable-model-invocation: true`, so nothing starts or closes a
Task on its own; and a body that is instructions rather than a single injected
line. The old command files are deleted with no alias kept, because the one
thing an alias would preserve is the path this change exists to close —
starting work with no Ticket.

The rename costs one mechanical edit plus this document: the history hook's
filter (`handlers.go`, `fields[0] == "/new-task"`). `settings.json` names
`/new-task` nowhere — it wires the hook, not the command. Only `/task-start`
belongs in that filter; `/task-done` and `/task-fail` are ordinary prompts and
belong in the Task's history file.

`/task-start`'s whole job, in order:

1. roll the history state, exactly as `new-task.sh` does today — the state
   file goes, so the next prompt opens a fresh file under `docs/history/`;
2. make sure a Ticket exists;
3. bind it;
4. `TO DO → PROCESSING`.

Step 2 has two branches, and they are the two ways work begins here:

- **an id was passed** — `/task-start 86cb8hjf7`: verify it exists, bind it.
  This is the loop's branch, and it is why `new-task.md`'s open gap has to
  close. It invokes its script **with no arguments**, so `/new-task 86cb8hjf7`
  passes the id nowhere. It needs `$ARGUMENTS`.
- **no id** — interview the operator: take an existing Ticket, or describe the
  work and create one. There is no third "no ticket" option, and the skill
  does not let up until one of the two is chosen. This is the human's branch;
  under a mandate it is never reached, because the loop always passes an id.

Both branches end in the same state, and that is what makes the rest work: a
Ticket exists, it is bound, and it is `PROCESSING`.

The binding lives beside `hook-state`, in `docs/history/bound-ticket`, written
through the same atomic rename — same directory, same lifecycle, same question
("what are we on now"), and `docs/history/` is gitignored, so it never reaches
a commit. `/task-start` writes it through `history.sh bind <id>`; `/task-done`
and `/task-fail` read it with `bound` and clear it with `unbind`. **Unbind is
always last**: a status write that fails with the binding already gone leaves
a Ticket stuck in `PROCESSING` that nothing can close.

That moment is earlier than the first draft had it. There, binding waited for
the **first real prompt**: the `/new-task` prompt is filtered by the history
hook (`handlers.go`: `fields[0] == "/new-task"` → return), so no file opens
then, and the next prompt reaching `logPrompt` with `loadCurrent() == ""` was
"the only moment the system knows a new task has begun". It no longer is —
with the id in hand at invocation, and an interview that refuses to end
without one, `/task-start` knows before any prompt does. The hook's
`additionalContext` injection at `filename == ""` survives only as a backstop
for work someone started without `/task-start` at all.

All three reach ClickUp through the **MCP tools directly**, not through
`scripts/cmd/clickup`. That reverses the interview's answer, and the reason is
what the code turned out to be: `scripts/clickup` does not talk to ClickUp
either — it spawns `claude -p` with an MCP allowlist. A skill routing through
it would be a session shelling out to a second model turn to make a call the
first one can make itself, which is slower, costs tokens, and is *less*
deterministic, because it puts a model where a single tool call would do.

The "single ClickUp interface" argument stands for its actual constituency:
`scripts/merge` and anything else running outside a session, which has no MCP
server to inherit. Nothing there writes a status — the loop closes Tickets
through `/task-done` and `/task-fail`, both of which run inside a session — so
`scripts/cmd/clickup` needs no new subcommand for this design, and gets none.

## 10. A run that dies mid-ticket

Nothing recovers it. A ticket left in `PROCESSING` by a dead run means
something went wrong enough that repairing it automatically costs more than
abandoning it. The loop stops and waits for a human.

This is deliberate and it is the same principle the merge loop already holds:
a stop is a stop, and the state is left exactly as the failure left it so a
person can see what happened. There is no timeout, no sweep, no "probably safe
to retake".

## 11. Settled and not to be revisited

- `Good For Agent` is set by hand. Computing it is out of scope until the
  shape above has proven itself in use.
- `TO DO`, `PROCESSING`, `DONE`, `FAILED` and `COMPLETED` all exist as
  statuses today; `done` and `failed` were confirmed against the board.
- `/task-start` never interviews anyone under a mandate, because the loop
  always passes an id. The interview is the human path and only that.
- The transitions this design performs — `TO DO → PROCESSING → DONE|FAILED` —
  go through `/task-start`, `/task-done` and `/task-fail` and through nothing
  else, in either mode. `FAILED → TO DO` and `COMPLETED` stay human moves in
  the ClickUp UI; no skill writes them.
- The branch and the return to `main` are already implemented, in `commit.sh`
  and `scripts/merge/land.go`. They are not `task-handle`'s to build.
- `fix-queue` is deleted rather than converged with `task-loop`, and the
  `CLAUDE.md` line naming it goes with it.
- Invoking `task-handle` *is* the mandate. The file in §3 exists only so
  `scripts/merge`, which cannot see a skill invocation, can read it.
- `task-loop` and `task-handle` run with `disallowed-tools: AskUserQuestion`, and a point
  where it would have had to ask is a decline, not a stall. The frontmatter is
  best-effort only: the restriction lapses on the user's next message, which
  is the cancellation case itself — so the rule lives in the prose and the
  frontmatter merely enforces it until then.
- A Ticket rejected during grooming — incomplete, already fixed, or wrong
  about the code — gets a comment and has `Good For Agent` unchecked. It is
  never given a status: it was never `PROCESSING`, and the `/task-*` family is
  the only status writer.
