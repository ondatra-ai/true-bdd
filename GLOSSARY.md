# TrueBDD

The engine that drives Claude-mediated checklists over a host project's specification documents. This glossary fixes the words the engine, its checklists and its BDD suite all use for the same things.

## Language

**Host project**:
The project whose specification documents the engine walks and whose code it builds. This repository is its own Host project: the engine's next version is specified and built by its previous one, while for anyone else the Host project is their own application.
_Avoid_: host application, host product, target project

**Registry**:
The single YAML file holding every Scenario, resolved from `documents.scenarios_yaml` (conventionally `docs/scenarios.yaml`).
_Avoid_: requirements file, scenarios file, spec file

**Scenario**:
One entry in the Registry, keyed by id, carrying a description, a `service`, a `path`, its originating `user_stories` and its `merged_steps`. The unit of specification a generated test is rendered from.
_Avoid_: test case, requirement, feature

**Story**:
One user story: a goal stated as who wants what and why, plus the Acceptance Criteria that define it. Written directly, out of a discussion of a Feature, never extracted from a larger document; no command creates one, and `us refine` is the first step of the pipeline that reads it.
_Avoid_: epic story, ticket, requirement

**Feature**:
A named group of Stories that together deliver one product capability. A label a Story carries, not a document of its own.
_Avoid_: epic, theme, module

**Acceptance Criterion**:
One testable condition inside a refined user story, before it is merged into the Registry. An AC becomes a Scenario; the two are the same shape and different lifecycle stages.
_Avoid_: AC (in prose), criterion, condition

**Item**:
Whatever a single command walks one of at a time — a Story for `us refine`, an Acceptance Criterion for `us apply`, a Scenario for `scen check`.
_Avoid_: subject, entity, record

**Prompt**:
One validation question in a Checklist, with its rationale, its declared reference documents and its optional `F:` fix template. A model answers a `Q:` Prompt; the engine's own code answers a `D:` Prompt, which names one of the engine's built-in checks, and an Item's `D:` Prompts are answered before any of its `Q:` Prompts. Never the rendered text sent to a model; that is the rendered prompt for a Cell.
_Avoid_: check, question, validation item

**Cell**:
One (Item, Prompt) pair. The unit of work an AI turn is spent on, and the unit a fix is applied to.
_Avoid_: check, cell-run

**Walk**:
One full traversal of every Cell in a run. A fix applied mid-walk restarts the traversal from the first Cell.
_Avoid_: pass, sweep, iteration

**Step coverage**:
The engine's own answer to which Scenarios have a step no definition binds: the Scenarios it examined, and the unbound steps among them. `examined` is the load-bearing half — an empty gap list says nothing about a Scenario nobody looked at. Resolved in-process by parsing each suite's `steps/` package, never by running a test; a step two definitions match is a refusal rather than a gap.
_Avoid_: coverage report, coverage output, step report, gap list

**Services**:
The process a scenario exercises — the `true-bdd` binary for the CLI suite — as distinct from the tests that drive it. Its `third_party` CLI dependencies are the ones the harness shims; a Services caller declaring none has a vacuous mode axis.
_Avoid_: SUT, subject, system, target

## Repository vocabulary

Words for how this repository plans and executes its own work. Not the engine's domain — nothing here reaches a host project.

**Console**:
A program's ANSWER — bytes a caller reads verbatim in a shape it fixed, plus the engine's prompts and the reader they are answered on. Not narration: what a program did while it worked is a Log. Byte-plain forever, because the lint hook's stdout is parsed as JSON. `pkg/console` is the only implementation, and owns the three standard descriptors; `depguard` denies it to `scripts/` outright, which narrates and never answers.
_Avoid_: output, print, terminal, UI

**Log**:
One `log/slog` record: everything a program says ABOUT its own run. Severity is a level, never a choice of stream — the stream is bound once per program in `main()`, and every `scripts/` program stamps each record with its writer in `tool` and its process in `run`. `pkg/logging` is the only implementation; the engine's message strings and attribute keys are a wire contract `pkg/testkit/reporter` reads.
_Avoid_: trace, diagnostic, stderr

**Run**:
One process of a `scripts/` program, from its first record to its last, named by the 8-hex `run` attribute `pkg/logging` stamps.
_Avoid_: invocation, execution, session

**Disk access**:
One filesystem read or write, taken through `pkg/disk` under a Hold and returning before any handle escapes.
_Avoid_: save, dump, persist, IO

**Hold**:
The short advisory `flock` a Disk access takes on the target's parent directory and releases before returning — shared to read, exclusive to write. The parent rather than the file because a whole-file write ends in a rename, which replaces the target's inode.
_Avoid_: lock, mutex, guard
