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

## Repository vocabulary

Words for how this repository plans and executes its own work. Not the engine's domain — nothing here reaches a host project.

**Ticket**:
One unit of planned work, held in ClickUp. Carries the four headings `scripts/clickup` renders and, when it may be taken unattended, the fields `true-bdd/ticket-schema.yaml` requires.
_Avoid_: task (for this sense), issue, card, story

**Task**:
One session's unit of work — the history file under `docs/history/` that `/task-start` opens. At most one Ticket is bound to it, from `/task-start` until `/task-done` or `/task-fail`.
_Avoid_: session, run, thread

**Console**:
A program's ANSWER — bytes a caller reads verbatim in a shape it fixed, plus the engine's prompts and the reader they are answered on. Not narration: what a program did while it worked is a Log. Byte-plain forever, because the lint hook's stdout is parsed as JSON. `pkg/console` is the only implementation, and owns the three standard descriptors; `depguard` denies it to `scripts/` outright, which narrates and never answers.
_Avoid_: output, print, terminal, UI

**Log**:
One `log/slog` record: everything a program says ABOUT its own run. Severity is a level, never a choice of stream — the stream is bound once per program in `main()`, and every `scripts/` program also appends to the shared `docs/history/tools.log.json`, each record naming its writer in `tool`. `pkg/logging` is the only implementation; the engine's message strings and attribute keys are a wire contract `tests/libraries/reporter` reads.
_Avoid_: trace, diagnostic, stderr

**Disk access**:
One filesystem read or write, taken through `pkg/disk` under a Hold and returning before any handle escapes.
_Avoid_: save, dump, persist, IO

**Hold**:
The short advisory `flock` a Disk access takes on the target's parent directory and releases before returning — shared to read, exclusive to write. The parent rather than the file because a whole-file write ends in a rename, which replaces the target's inode.
_Avoid_: lock, mutex, guard
