# TrueBDD

The engine that drives Claude-mediated checklists over a host project's specification documents. This glossary fixes the words the engine, its checklists and its BDD suite all use for the same things.

## Language

**Registry**:
The single YAML file holding every Scenario, resolved from `documents.scenarios_yaml` (conventionally `docs/scenarios.yaml`).
_Avoid_: requirements file, scenarios file, spec file

**Scenario**:
One entry in the Registry, keyed by id, carrying a description, a `service`, a `path`, its originating `user_stories` and its `merged_steps`. The unit of specification a generated test is rendered from.
_Avoid_: test case, requirement, feature

**Acceptance Criterion**:
One testable condition inside a refined user story, before it is merged into the Registry. An AC becomes a Scenario; the two are the same shape and different lifecycle stages.
_Avoid_: AC (in prose), criterion, condition

**Item**:
Whatever a single command walks one of at a time — a Story for `us create`/`us refine`, an Acceptance Criterion for `us apply`, a Scenario for `scen check`.
_Avoid_: subject, entity, record

**Prompt**:
One validation question in a Checklist — the `Q:` block, its rationale, its declared reference documents and its optional `F:` fix template. Never the rendered text sent to a model; that is the rendered prompt for a Cell.
_Avoid_: check, question, validation item

**Cell**:
One (Item, Prompt) pair. The unit of work an AI turn is spent on, and the unit a fix is applied to.
_Avoid_: check, cell-run

**Walk**:
One full traversal of every Cell in a run. A fix applied mid-walk restarts the traversal from the first Cell.
_Avoid_: pass, sweep, iteration

**Coverage report**:
What one test tree writes when `build tests` asks which of its Scenarios have a step no definition binds: the Scenarios it examined, the unbound steps among them, and any step two definitions match. `examined` is the load-bearing half — an empty gap list says nothing about a Scenario the tree never looked at. One file per tree in the directory the engine hands over, because one command may start several and the engine merges every report it finds.
_Avoid_: coverage output, step report, gap list

**Services**:
The process a scenario exercises — the `true-bdd` binary for the CLI suite — as distinct from the tests that drive it. Its `third_party` CLI dependencies are the ones the harness shims; a Services caller declaring none has a vacuous mode axis.
_Avoid_: SUT, subject, system, target

## Repository vocabulary

Words for how this repository plans and executes its own work. Not the engine's domain — nothing here reaches a host project.

**Ticket**:
One unit of planned work, held in ClickUp. Carries the four headings `scripts/clickup` renders and, when it may be taken unattended, the fields `.claude/skills/task-handle/ticket-schema.yaml` requires.
_Avoid_: task (for this sense), issue, card, story

**Subject**:
One claim that something in this repository should change, on its way to being judged: a review finding, a postmortem proposal, or an existing Ticket. What a caller hands `scripts/triage`, and the only thing that differs between its three callers besides what they do with the answer.
_Avoid_: item, candidate, claim, finding (for this sense)

**Verdict**:
What one Triage answered about one Subject: a score, a code-anchored reason, and — only when the Subject asked to be refreshed — the body rewritten against the tree it was read against.
_Avoid_: result, judgement, assessment

**Triage**:
Scoring one Subject 1-10 by the consequence of leaving it undone, read against the tree as it stands. One rubric for the whole repository (`scripts/triage`), because relevance is not a second axis: a claim about code that no longer exists has no consequence, and scores 1. The disposition is the caller's — merge fixes, tickets or drops by its own Floors; `scripts/clickup` retires below 6 or refuses to file.
_Avoid_: scoring, grooming, prioritisation, ranking

**Corpus**:
Every Ticket in the tracker, dumped to one markdown file each under `tmp/dupes/corpus/` for a model to read. Written fresh on every gate run and never cached: one merge files `fix-now` and then the postmortem minutes later, so the second Corpus has to hold what the first filing just created. Holds every status, because a proposal recurring after it was retired is a duplicate too.
_Avoid_: index, cache, snapshot, dump

**Duplicate gate**:
The check every filing path runs before the render: a cheap 60-rune title prefix against open Tickets, then the same within the queue, then one model turn per candidate scoring it 1-10 for identity against the Corpus. Files only at 3 or lower. Dropping before the render is load-bearing — the heading count, the field plan and the filed-count check all have to agree on one shortened queue.
_Avoid_: dedupe, filter, similarity check

**Keeper**:
The copy of a duplicated proposal that survives, the rest retired to `not relevant` with a comment naming its URL. Chosen by rule, not by hand: what shipped, then what a person promoted, then the higher Triage Score, the fresher Triage Date, and failing all of those the older Ticket, whose URL anything outside ClickUp would already cite.
_Avoid_: primary, canonical, original, master

**Task**:
One session's unit of work — the history file under `docs/history/` that `/task-start` opens. At most one Ticket is bound to it, from `/task-start` until `/task-done` or `/task-fail`.
_Avoid_: session, run, thread

**Console**:
A program's ANSWER — bytes a caller reads verbatim in a shape it fixed, plus the engine's prompts and the reader they are answered on. Not narration: what a program did while it worked is a Log. Byte-plain forever, because the lint hook's stdout is parsed as JSON. `pkg/console` is the only implementation, and owns the three standard descriptors; `depguard` denies it to `scripts/` outright, which narrates and never answers.
_Avoid_: output, print, terminal, UI

**Log**:
One `log/slog` record: everything a program says ABOUT its own run. Severity is a level, never a choice of stream — the stream is bound once per program in `main()`, and every `scripts/` program also appends to the Task's shared log under `docs/history/task_logs/`, each record naming its writer in `tool` and its process in `run`; `scripts/report` folds one `run`'s tree back out of it. `pkg/logging` is the only implementation; the engine's message strings and attribute keys are a wire contract `pkg/testkit/reporter` reads.
_Avoid_: trace, diagnostic, stderr

**Report**:
The tree a Run walked, folded back out of the Task's log and rendered as a table: every Node and Leaf, what it resulted in, and how long it took. Structure is WRITTEN and naming DERIVED — a writer stamps only `tree` and `duration_ms`; the nesting, the numbering and the status column are `scripts/report`'s, computed from the order the records arrived in. Nothing is read from prose.
_Avoid_: summary, timeline, trace

**Run**:
One process of a `scripts/` program, from its first record to its last, named by the 8-hex `run` attribute `pkg/logging` stamps. Not a Task, which outlives many of them: a Task's log holds every Run in it, and merge's fix agents spawn `gates` as a separate Run into the same file, so a Report folds one `run` at a time or the tree it recovers is corrupt.
_Avoid_: invocation, execution, session

**Node**:
One operation or sub-operation of a Run, opened and closed by a pair of `tree=start` / `tree=end` records and nesting by call order. `report.Open` writes both; a Node left open never finished, because `dief`'s `os.Exit` skips every `defer`.
_Avoid_: step, stage, span, phase

**Leaf**:
One measured thing inside a Node — a single record carrying its own `duration_ms`, and no markers. What the three dispositions, every gate and every AI turn emit — a thing measured rather than a span nested inside other spans.
_Avoid_: event, point, sample

**Disk access**:
One filesystem read or write, taken through `pkg/disk` under a Hold and returning before any handle escapes.
_Avoid_: save, dump, persist, IO

**Hold**:
The short advisory `flock` a Disk access takes on the target's parent directory and releases before returning — shared to read, exclusive to write. The parent rather than the file because a whole-file write ends in a rename, which replaces the target's inode.
_Avoid_: lock, mutex, guard
