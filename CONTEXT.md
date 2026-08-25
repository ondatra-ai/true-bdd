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
