# TrueBDD

The engine that drives Claude-mediated checklists over a host project's specification documents. This glossary fixes the words the engine, its checklists and its BDD suite all use for the same things.

## Language

**Host project**:
The project whose specification documents the engine walks and whose code it builds. This repository is its own Host project: the engine's next version is specified and built by its previous one, while for anyone else the Host project is their own application.
_Avoid_: host application, host product, target project

**Specification**:
The three documents a Host project's system is built from, and the only source of how it is built: the product document, the architecture document (with the interface documents it names) and the Registry. The tests and the code are recovered from it; a rebuild preserves the system's behaviour, not its code.
_Avoid_: spec files, sources, the docs

**Public interface**:
What a service offers to anything outside it, with its formats and the criteria it meets. The architecture document declares it for each service, in the standard form of the service's kind (`cli`, `http` or `ui`), and every rebuild keeps it; a build never chooses it. A step definition sets up and observes a service through it from outside, across a process boundary, sharing no code or package with the service; so a library alone is never a service the Registry tests.
_Avoid_: external API, contract, surface

**Persistent data model**:
The shape of the data a service keeps between its runs, such as a database's schema, and any code that defines that shape. It is not public, but data written by one build must be read by the next, so the architecture document declares it and every rebuild keeps it. A step definition may set up or check stored data through it; how, when code defines the model, is not decided yet.
_Avoid_: storage format, schema (alone)

**Model**:
The YAML data a service fills a checklist template pair with, declared in its architecture `models` by templates and a scheme. The scheme states fields and types, checked in process before rendering; Scenarios state construction rules. AI model tiers remain `engine.models`.
_Avoid_: prompt data (for a declared Model)

**Internal code**:
Everything in a service that is neither its Public interface nor its Persistent data model. A build chooses it freely, a rebuild need not keep it, and no step definition reaches it.
_Avoid_: private API, implementation details

**Build identity**:
The engine's build version and source commit, printed by `true-bdd version` as `true-bdd version <version> commit <commit>`. Each uses its link-time stamp (`-ldflags -X`) first, Go's main-module version or `vcs.revision` next, then `(devel)` or `unknown`. A VCS build may have a pseudo-version; an archive has no VCS metadata. Commands installing the engine Log record it once at start as `Engine started` with `version` and `commit` fields, including `init`; `version` prints only the newline-terminated answer and creates no file or directory. `version`, help, the hidden guard and refusals before logging have no start record.
_Avoid_: release number, build number

**Registry**:
The single YAML file holding every Scenario, resolved from `documents.scenarios_yaml` (conventionally `docs/scenarios.yaml`).
_Avoid_: requirements file, scenarios file, spec file

**Scenario**:
One entry in the Registry, keyed by its lineage id `<story-id>-<ac-id>` (legacy entries keep an `E2E-NNN` key until their Story renames them), carrying a description, a `service` the architecture document declares, a `path`, its originating `user_stories` and its `merged_steps`. The unit of specification a generated test is rendered from, and named after: the key in CamelCase.
_Avoid_: test case, requirement, feature

**Given**:
The state the system is in before the behaviour starts, put in place without the role acting. It is never an action of the role: entering a number or pressing a button is a When.
_Avoid_: setup action, precondition step

**When**:
The action a role takes, or an event, that the behaviour responds to.
_Avoid_: trigger step

**Then**:
The outcome the behaviour must produce, observed from outside the system and compared with what is expected, never read from its storage or code.
_Avoid_: verification step, assertion step

**Story**:
One user story: a goal stated as who wants what and why, plus the Acceptance Criteria that define it. Written directly, out of a discussion of a Feature, never extracted from a larger document; no command creates one, and `us refine` is the first step of the pipeline that reads it.
_Avoid_: epic story, ticket, requirement

**Feature**:
A named group of Stories that together deliver one product capability. A label a Story carries, not a document of its own.
_Avoid_: epic, theme, module

**Acceptance Criterion**:
One testable condition inside a refined user story, before it is merged into the Registry. An AC becomes a Scenario; the two are the same shape and different lifecycle stages. Its id is `<NNN>-<slug>`, written by the author, and becomes the second half of that Scenario's key.
_Avoid_: AC (in prose), criterion, condition

**Item**:
Whatever a single command walks — a Story for `us refine`, an Acceptance Criterion for `us apply`, a Scenario for `scen check`, the Failing set for `build code`, and for `build steps` a concept in a go-test suite, or a harness suite. A concept Item holds the suite's unbound steps grouped into its concept, or the pending skeletons of that concept a walk finds, and requires a fresh unambiguous service binding before any skeleton. The Pattern bindings Checklist item checks its registered patterns against every deterministic suite step; its F proposes patterns and rewrites skeletons recording the concept. Step coverage then writes the bodies through its F, and the step-verification Checklist item judges each pattern and body. Its one F fixes the named definitions, proposing a pattern again when needed and writing its body for the new captures. With `testing.testkit`, one suite Item per distinct Steps package follows the concept Items, even when none exist; only `scenario-run` is asked about it, checking State's embedded `*testkit.ScenarioRun` and adding it through a code F at the fix menu. Concept checklist items are asked only about concept Items; unasked Cells produce no result row or pass.
_Avoid_: subject, entity, record

**Checklist**:
The checklist items a command asks about its Items, in their written order. The command decides what each is asked about.
_Avoid_: validation pipeline

**Checklist item**:
One verification `Q`, answered by a model through its `prompt` option or engine code through its `code` option, never both, with optional fix `F`, rationale, reference documents and parameters. One checklist item answers one question; its position in the selected checklist names its Cell's artifacts.
_Avoid_: Prompt, validation item

**Model-run step**:
A step a model carries out in place of a step definition. An `llm:` step performs, in a Given or When, an operation its Concept binding states by intent rather than by element; a `judge:` clause rules, in a Then, on meaning no comparison can settle. Every other step runs through a step definition.
_Avoid_: AI step, LLM step, smart step

**Built-in check**:
Engine code named by a Q's `code` option, answering it for an Item without a model: pass or fail, with one line per finding.
_Avoid_: deterministic check, D-check, rule, validator

**Cell**:
One (Item, Checklist item) pair, answered by its Q and repaired by its F.
_Avoid_: check, cell-run

**Walk**:
A traversal of every Cell, in written checklist order within each Item. Without `--fix` it reports every failure with no skip. With `--fix` an apply rechecks its Q, repeating its F within the bound until it passes, then restarts the Item's checklist; a walk applying a fix is followed by a walk of all Items.
_Avoid_: pass, sweep, iteration

**Earlier step**:
A previous command of the pipeline `us refine`, `us apply`, `build steps`, `build tests`, `build code`. This names a term, not an instruction to an F.
_Avoid_: earlier work within the current command

**Cannot-fix outcome**:
Any F's answer that it cannot repair what its Q found, at proposal, clarification reply or apply, with a reason and what a person must correct. An F fixes only what its Q found; a problem it did not find is neither its work nor a reason for cannot-fix. The optional `cannot_fix` mapping uses `reason` and `correction`; it stops the run distinctly, naming the Checklist item's section and index. A person's pending decision is a legitimate cannot-fix. `interface_cannot_serve` and `impossible` remain instances with their existing behaviour, reruns and log records.
_Avoid_: retry exhaustion, not-applied repair

**Pattern bindings**:
The `pattern-bindings` code Checklist item of `build steps`, before Step coverage, checking one concept Item's registered patterns against every deterministic suite step. Patterns compile, name one capture per group and bind only the Item's own steps, each exactly once; every pattern binds some step. Its F holds the original pattern-proposal prompt, proposes and rewrites skeletons without judging them; its Q judges them after apply, then the checklist restarts. A resumed registration takes its capture count from its regex; new proposals retain the named-capture comparison.
_Avoid_: proposal evaluation, hidden pattern check

**Step coverage**:
The engine's own answer to which Scenarios have a step no definition binds: the Scenarios it examined, and the unbound steps among them. `examined` is the load-bearing half — an empty gap list says nothing about a Scenario nobody looked at. Answered by `build steps` in-process by parsing each suite's `steps/` package, never by running a test; a step two definitions match is a refusal rather than a gap. On a concept host, distinct gap texts feed grouping per suite and block, excluding `judge:` and `llm:`; this separate model pass applies the link threshold and ambiguity rule, and the gaps grouped into one concept become that concept's Item. A step whose one definition is a pending concept skeleton is not grouped again but resumed in its concept's Item. Coverage passes once every step binds one definition with its body written; `scen check`'s step-definitions check then holds each definition a step binds to a declared concept of the step's block.
_Avoid_: coverage report, coverage output, step report, gap list

**Red without the target**:
The state of every test while the code is not built: red, because there is no code. `build code` checks it first when no code is built, listing the tests and running the suite once, where every Scenario whose service has no code must fail at a step whose definition ran; a Scenario that passes stops it with a distinct error naming it, since correctly written steps cannot pass without the code (ADR-0040).
_Avoid_: expected failure, any failure counts

**Steps package**:
A suite's step definitions in `tests/<suite>/steps/` and code they share in the sibling `helpers/`, the one generated part of a suite that is checked. Helpers need no separate declaration and are verified as part of their calling bodies, including whether they do the product's work. `build steps` writes the Steps package before any test exists; the tests `build tests` renders import it.
_Avoid_: step folder, step library, test steps

**Step definition**:
The Go function that carries out a Scenario's step, registered against a pattern that binds step texts: the step is the Scenario's line of text, the step definition the code that runs it. The Steps package holds them.
_Avoid_: step (for the function), step implementation, glue

**Verification of step definitions**:
The `step-verification` Checklist item of `build steps`, after Step coverage, asking whether a generated step definition matches its steps under its Concept and binding. Its tool-less Q receives complete referenced code and API declarations, the semantic core, declared architecture step-argument rules and step forms with complete examples, with no fixture files. Its one F fixes only its findings, proposing a pattern and rewriting its skeleton through the same protocol as Pattern bindings, with no independent pattern check. After apply the verification Q runs again; once it passes, the checklist restarts and Pattern bindings judges any changed pattern.
_Avoid_: critique

**Catalogue**:
Retired: the product document's former `catalogue` key of step templates, each a step's shape, block, meaning and terms. Concepts replaced it (ADR-0034), and the engine refuses at startup a product document that still declares one, naming `concepts:` in its place. Older ADRs still name it.
_Avoid_: vocabulary (the words a Story must not use), phrase book, step library

**Concept**:
One way of putting state in place (Given), calling (When), or observing and comparing (Then) through one Public interface item, per block. The item is what the use is about and may be reached through several declared references. What the step definition does, as the step's wording fixes it, decides its concept; an argument's target never changes it. Its closed `means` names the item, kind of use and every admitted variation in prose, relying only on declared `terms` and plain English; a concept holds only `concept`, `means` and `terms`, so a widening rewrites its `means`, and the engine refuses any other key wherever it reads concepts. A Given concept states a state. Names are unique within a block (ADR-0036). After the link, a separate model Checklist item judges every linked meaning for undeclared domain words, per Story in `us refine` and, in `scen check`, once per walk for each linked concept, every Scenario linking it answering from that verdict. Its Product Owner menu fix declares the word in both the glossary and the concept, or rewords the meaning; a new Word mapping follows through the System Architect’s term-mapping menu.
_Avoid_: step template, phrase (for a concept)

**Concept binding**:
An `architecture.concepts` entry naming a Concept by name and block and its service, with Public interface `references` and a `does` sentence describing what its step definition does by block. An `intent` replaces both for a UI service. Each concept (name and block) and service pair has at most one binding, and every reference is declared by that service. The built-in concept-bindings check holds these declarations without linking steps. After the link, scen check requires every linked concept to be bound for the Scenario's service and judges each such binding usable once per walk, for every variation its concept admits, with every step of the walk's links that reached it; every Scenario using it answers from that verdict. An unused unbound concept is not that Scenario's finding.
_Avoid_: concept mapping, step binding

**Word mapping**:
The architecture document's list of terms, each saying what the term is in the implementation, with optional instructions for setup or observation when the Concept binding does not already fix them, without naming code. Every term a Concept declares is mapped exactly once.
_Avoid_: architecture glossary, dictionary

**Phrase mapping**:
Retired: the architecture document's former `architecture.phrases`, one entry per Catalogue template and service. Concept bindings replaced it (ADR-0035), and the engine refuses at startup an architecture document that still declares one, naming `architecture.concepts` in its place. Older ADRs still name it.
_Avoid_: step mapping, operation map

**Term document**:
A specification document holding declared meanings and their implementation: the product document, with its glossary and Concepts, and the architecture document, with the Word mapping and Concept bindings. A fix for a failed Closure check edits a Term document or a named Public interface document, never the Scenario.
_Avoid_: vocabulary file, dictionary

**Closure check**:
One of the five Built-in checks over a Scenario and its Term documents. public-interface holds its service to a valid Public interface declaration. declared-terms and term-mappings hold every declared concept's terms to the glossary and the Word mapping, concept-bindings holds every Concept binding to a declared concept and its service's Public interface, and step-definitions holds each step of the Scenario that a step definition binds to exactly one, recording a declared concept of the step's block, a Story claiming the Scenario or not; a step no definition binds yet passes as build steps' gap, since scen check runs before build steps writes it. These four pass every Scenario of a host declaring no concepts as not checked. Model Checklist items link steps and judge their service bindings; the Closure checks answer without an AI turn.
_Avoid_: spec check, term check

**Services**:
The process a scenario exercises — the `true-bdd` binary for the CLI suite — as distinct from the tests that drive it. Its dependencies with `protocol: cli` and `scope: third_party` are the ones `pkg/proxy/cli` substitutes with CLI stand-ins; a Services caller declaring none has a vacuous mode axis.
_Avoid_: SUT, subject, system, target

**Fix loop**:
What a command run with `--fix` does for a failed Checklist item: ask any clarifying questions, propose a fix, and read the person's apply, refine or exit choice from stdin at the fix menu. Every apply requires that answer; the engine offers no way to approve fixes in advance. End of input stops the menu or an unanswered clarification with a distinct error.
_Avoid_: automatic approval, advance approval

**Failing set**:
All tests failing in one `build code` discovery, walked as one Item with one proposed repair plan. After each completed apply, including a started codex writer that exits non-zero or answers empty, or a started Claude writer that exits non-zero, returns an error result or answers empty, the engine reruns its remaining members once; passing members leave, missing results stay failing. Synthetic members include Go `package::<build>`, Jest `file::<load>` for each file that fails to load (carrying Jest's message), and Playwright `<startup>`. A load member reruns by file path and passes only when that file reports success; a missing file stays failing. A non-zero test command with no named member stops the run with its command, exit code and output tail, before any AI turn or `BUILD CODE COMPLETE`. A green discovery yields no Item; `build code` reports success only after the test command succeeds. Its code Q answers the whole set's latest runner verdict without an AI turn. Its plan and writer turns get each matched Scenario and what the Specification says its steps mean: the concepts the steps' definitions record, with their words and bindings.
_Avoid_: repair batch, failure queue

**Writable paths**:
The exact files or folders a service declares under `writable` in the architecture document, relative to the Host project root, with no patterns. Together with that service's `path` and the host's tmp they bound its build-code writer. A grant covering tests or step definitions, or lying outside the host, is refused before any test runner or AI turn. Here the tests are `tests/`, the Registry's suite directories, the in-host directory holding `testing.config`, and the in-host package directories named by the Go `testing.commands` (`./dir` or `./dir/...`, relative to the command's working directory). A broad `./...` names no separate test directory. Command package directories inside a service's own `path` remain service code; `_test.go` files there are writable by build code. The filename suffix alone never makes a file protected. The reserved `tests/`, Registry and configuration directories still take precedence. Protected tests and step definitions can only be read and run by build code. Build steps writes only those protected test directories and host tmp; grants overlapping service code or Specification documents are refused.
_Avoid_: extra roots, writable globs, implicit manifest grants

**Impossible repair**:
The build-code instance of a Cannot-fix outcome: a plan or writer verdict that no solution exists within the declared writable paths, listing the required undeclared files and a reason for each under `impossible`. A preferred undeclared change alone is insufficient. The first report keeps edits, reruns the remaining Failing set once, then fails with a distinct error naming the files, reasons, remaining failures and the remedy: change the files by hand or add them to the service's `writable`. No further attempt or COMPLETE banner follows. An empty list preserves ordinary retries, including `applied: false`.
_Avoid_: retry exhaustion, not-applied repair

**Model override**:
A repeatable `--model <tier>=<cli>:<model>` command-line flag that replaces one declared `engine.models` binding in memory for one run. Every command running AI turns accepts it. It changes neither project configuration nor spawned processes' environments; each applied change is logged once.
_Avoid_: model environment variable, model config overlay

**Library**:
Code supplied apart from a service's own source that the service or its tests build with, named in the architecture document with its exact version; what a named library itself needs to build comes with it. The service entry and `testing` declare `libraries` as lists of `{module, version}`. Before a writer turn, the run downloads every named Go module and its build graph; a download failure stops the run, naming the library and error. Writers read only that graph and its exact version metadata from the module cache. A rebuild may use nothing else, and a service may not use a library named only for its tests. A suite's folder holds only its fixtures and what the pipeline generates from the Specification; anything else its tests use is a library. For non-Go hosts the lists remain empty; existing Jest and Playwright paths are unchanged. Only codex enforces the access and network boundary for subprocesses; Claude's file tools obey it, but its replay remains unconfined with network for model trials.
_Avoid_: dependency (a service's dependencies are the processes and CLIs it talks to while it runs), package, framework

**Suite wiring**:
Retired (ADR-0040, Q21): the former name of the tests' main file, `main_test.go`, which builds the suite, registers the step definitions, creates each Scenario's state and hands each test its run. `build tests` writes it with the tests, which call `New(t, id)` directly.
_Avoid_: glue, shim, bootstrap, wiring

**Test runtime**:
The same mechanisms for every Host project: `pkg/testkit` exposes the harness through its root API and keeps its runner and reporter under `internal/`; `pkg/proxy/cli` supplies the CLI SDK and `utilities/cliproxy` its stand-in binary. Product facts come from the Specification and the Steps package; this repository's report and coverage tooling lives in `scripts/`.
_Avoid_: engine harness, product runtime

**CLI stand-in**:
The program `utilities/cliproxy` that `pkg/proxy/cli` builds from the host's `pkg` commit and puts first on PATH under a declared dependency's name. It records, replays or forwards calls using the dependency's recording rules, and records spawn observations whose paths a Given supplies through the SDK; Then steps judge them. Vendor invocation rules stay in the stand-in; it knows no Host project's product.
_Avoid_: AI proxy, aiproxy (for the runtime component)

**Scenario run**:
The Test runtime's state for one Scenario, carried by `testkit.ScenarioRun`: its test, Scenario, prepared fixture and result, with methods for the next command's environment and PATH, its mode and work before the initial snapshot. The runtime privately owns proxy setup, recording and grading; step definitions use the `pkg/testkit` root and `pkg/proxy/cli` SDK. A suite declaring `architecture.testing.testkit` embeds `*testkit.ScenarioRun` in `State` as its one harness field (ensured even when no new skeleton is written) and adds only the definitions' own data.
_Avoid_: world (for the harness's type), harness state, test context

## Repository vocabulary

Words for how this repository plans and executes its own work. Not the engine's domain — nothing here reaches a host project.

**Console**:
A program's ANSWER — bytes a caller reads verbatim in a shape it fixed, plus the engine's prompts and the reader they are answered on. Not narration: what a program did while it worked is a Log. Byte-plain forever, because the lint hook's stdout is parsed as JSON. `services/true-bdd/internal/infrastructure/console` implements the engine's Console; `pkg/console` implements it independently for tests and tooling. Each owns its program's three standard descriptors; `depguard` denies it to `scripts/` outright, which narrates and never answers.
_Avoid_: output, print, terminal, UI

**Log**:
One `log/slog` record: everything a program says ABOUT its own run. Severity is a level, never a choice of stream — the stream is bound once per program in `main()`, and every `scripts/` program stamps each record with its writer in `tool` and its process in `run`. `services/true-bdd/internal/infrastructure/logging` implements the engine's Log; `pkg/logging` implements it independently for tests and tooling; the engine's message strings and attribute keys are a wire contract `scripts/reporter` reads.
_Avoid_: trace, diagnostic, stderr

**Run**:
One process of a `scripts/` program, from its first record to its last, named by the 8-hex `run` attribute `pkg/logging` stamps.
_Avoid_: invocation, execution, session

**Disk access**:
One filesystem read or write, taken through the engine's `services/true-bdd/internal/infrastructure/disk`, or tests/tooling's independent `pkg/disk`, under a Hold and returning before any handle escapes.
_Avoid_: save, dump, persist, IO

**Hold**:
The short advisory `flock` a Disk access takes on the target's parent directory and releases before returning — shared to read, exclusive to write. The parent rather than the file because a whole-file write ends in a rename, which replaces the target's inode.
_Avoid_: lock, mutex, guard
