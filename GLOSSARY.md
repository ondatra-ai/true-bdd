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
What a service offers to anything outside it, with its formats and the criteria it meets. The architecture document declares it for each service, in the standard form of the service's kind, and every rebuild keeps it; a build never chooses it. The supported kinds are `cli` (an OpenCLI-like document with named file, log-record and outgoing-call extensions), `http` (an OpenAPI 3 document), and `ui` (elements by role and accessible name within a page or region); other process-boundary kinds await support. Named documents are relative to the Host project root and load at command start; missing or invalid documents refuse startup before any test run, Item or AI turn. The declaration is optional for loading, but `scen check` fails every Scenario whose service has no valid declaration, without an AI turn for that check. Its fix asks the System Architect what the service offers and writes the architecture document, never the Scenario. A step definition sets up and observes a service through it from outside, across a process boundary, sharing no code or package with the service; so a library alone is never a service the Registry tests.
_Avoid_: external API, contract, surface

**Persistent data model**:
The shape of the data a service keeps between its runs, such as a database's schema, and any code that defines that shape. It is not public, but data written by one build must be read by the next, so the architecture document declares it and every rebuild keeps it. A step definition may set up or check stored data through it; how, when code defines the model, is not decided yet.
_Avoid_: storage format, schema (alone)

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
One entry in the Registry, keyed by its lineage id `<story-id>-<ac-id>` (legacy entries keep an `E2E-NNN` key until their Story renames them), carrying a description, a `service`, a `path`, its originating `user_stories` and its `merged_steps`. The unit of specification a generated test is rendered from, and named after: the key in CamelCase.
_Avoid_: test case, requirement, feature

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
Whatever a single command walks one of at a time — a Story for `us refine`, an Acceptance Criterion for `us apply`, a Scenario for `scen check`, a Catalogue phrase a go-test suite has no step definition for, for `build steps` (phrase × suite, including unused phrases). `build tests` renders files and walks no Items.
_Avoid_: subject, entity, record

**Prompt**:
One validation question in a Checklist, with its rationale, its declared reference documents and its optional `F:` fix template. A model answers a `Q:` Prompt; the engine's own code answers a `D:` Prompt, which names one of the engine's built-in checks, and an Item's `D:` Prompts are answered before any of its `Q:` Prompts. Never the rendered text sent to a model; that is the rendered prompt for a Cell.
_Avoid_: check, question, validation item

**Model-run step**:
A step a model carries out in place of a step definition. An `llm:` step performs, in a Given or When, an operation the Phrase mapping states by intent rather than by element; a `judge:` clause rules, in a Then, on meaning no comparison can settle. Every other step runs through a step definition.
_Avoid_: AI step, LLM step, smart step

**Built-in check**:
Code of the engine that a `D:` Prompt names, answering that Prompt for an Item without a model: pass or fail, with one line per finding. "Check" alone stays avoided: a Prompt is what a checklist asks, and a built-in check is one way to answer it.
_Avoid_: deterministic check, D-check, rule, validator

**Cell**:
One (Item, Prompt) pair. The unit of work an AI turn is spent on, and the unit a fix is applied to.
_Avoid_: check, cell-run

**Walk**:
One full traversal of every Cell in a run. A fix applied mid-walk restarts the traversal from the first Cell.
_Avoid_: pass, sweep, iteration

**Step coverage**:
The engine's own answer to which Scenarios have a step no definition binds: the Scenarios it examined, and the unbound steps among them. `examined` is the load-bearing half — an empty gap list says nothing about a Scenario nobody looked at. Answered by `build steps` in-process by parsing each suite's `steps/` package, never by running a test; a step two definitions match is a refusal rather than a gap.
_Avoid_: coverage report, coverage output, step report, gap list

**Catalogue**:
The product document's list of step phrases: each the shape a step may take, the block it may sit in, what it means, and the terms that meaning relies on. Every step a model does not run must match exactly one phrase.
_Avoid_: vocabulary (the words a Story must not use), phrase book, step library

**Word mapping**:
The architecture document's list of terms, each saying what the term is in the implementation and how a step definition sets it up or observes it, without naming code. Every term a catalogue phrase declares is mapped exactly once.
_Avoid_: architecture glossary, dictionary

**Phrase mapping**:
The architecture document's list of Catalogue phrases, each saying which operation of the Public interface a step of that shape performs: the operation a `when` phrase calls, how a `given` phrase sets up state, or what a `then` phrase observes and compares with. Each verbatim template is mapped once per service in `architecture.phrases`, with declared interface references and a `does` sentence, or an `intent` sentence replacing both. The `phrase-mappings` check holds phrases used by a claimed Scenario to this list; intent is accepted only for UI services until `llm:` support lands. References name CLI command paths or named file, log record and outgoing call extensions, HTTP operation IDs, or scoped UI roles and accessible names; no Go identifier or link to a step definition is stored. The Word mapping still covers the terms.
_Avoid_: step mapping, operation map

**Term document**:
A specification document that holds the words the Catalogue relies on: the product document, with the Catalogue and the glossary, and the architecture document, with the Word mapping (`architecture.terms`) and the Phrase mapping (`architecture.phrases`). A fix for a failed Closure check edits a term document, never the Scenario.
_Avoid_: vocabulary file, dictionary

**Closure check**:
One of the six Built-in checks that hold a Scenario to its Term documents: its service declares a valid Public interface, every step matches one Catalogue phrase, every matched phrase has one valid Phrase mapping for its service, the glossary defines and the Word mapping maps every term those phrases declare, and the phrases and the suite's step definitions correspond one to one. The five checks about phrases and terms pass a Scenario no Story claims, or a Host project with no Catalogue, unchecked; the Public interface check always runs.
_Avoid_: spec check, term check

**Services**:
The process a scenario exercises — the `true-bdd` binary for the CLI suite — as distinct from the tests that drive it. Its `third_party` CLI dependencies are the ones the harness shims; a Services caller declaring none has a vacuous mode axis.
_Avoid_: SUT, subject, system, target

**Fix loop**:
What a command run with `--fix` does for a failed Prompt: ask any clarifying questions, propose a fix, and read the person's apply, refine or exit choice from stdin at the fix menu. Every apply requires that answer; the engine offers no way to approve fixes in advance. End of input stops the menu or an unanswered clarification with a distinct error.
_Avoid_: automatic approval, advance approval

**Failing set**:
All tests failing in one `build code` discovery, walked as one Item with one proposed repair plan. After each completed apply, including a started codex writer that exits non-zero or answers empty, or a started Claude writer that exits non-zero, returns an error result or answers empty, the engine reruns its remaining members once; passing members leave, missing results stay failing. Synthetic members include Go `package::<build>`, Jest `file::<load>` for each file that fails to load (carrying Jest's message), and Playwright `<startup>`. A load member reruns by file path and passes only when that file reports success; a missing file stays failing. A non-zero test command with no named member stops the run with its command, exit code and output tail, before any AI turn or `BUILD CODE COMPLETE`. A green discovery yields no Item; `build code` reports success only after the test command succeeds. Its `D:` Prompt answers the whole set's latest runner verdict without an AI turn.
_Avoid_: repair batch, failure queue

**Writable paths**:
The exact files or folders a service declares under `writable` in the architecture document, relative to the Host project root, with no patterns. Together with that service's `path` and the host's tmp they bound its build-code writer. A grant covering tests or step definitions, or lying outside the host, is refused before any test runner or AI turn. Here the tests are `tests/`, the Registry's suite directories, the in-host directory holding `testing.config`, and the in-host package directories named by the Go `testing.commands` (`./dir` or `./dir/...`, relative to the command's working directory). A broad `./...` names no separate test directory. Command package directories inside a service's own `path` remain service code; `_test.go` files there are writable by build code. The filename suffix alone never makes a file protected. The reserved `tests/`, Registry and configuration directories still take precedence. Protected tests and step definitions can only be read and run.
_Avoid_: extra roots, writable globs, implicit manifest grants

**Impossible repair**:
A plan or writer verdict that no solution exists within the declared writable paths, listing the required undeclared files and a reason for each under `impossible`. A preferred undeclared change alone is insufficient. The first report keeps edits, reruns the remaining Failing set once, then fails with a distinct error naming the files, reasons, remaining failures and the remedy: change the files by hand or add them to the service's `writable`. No further attempt or COMPLETE banner follows. An empty list preserves ordinary retries, including `applied: false`.
_Avoid_: retry exhaustion, not-applied repair

**Model override**:
A repeatable `--model <tier>=<cli>:<model>` command-line flag that replaces one declared `engine.models` binding in memory for one run. Every command running AI turns accepts it. It changes neither project configuration nor spawned processes' environments; each applied change is logged once.
_Avoid_: model environment variable, model config overlay

**Library**:
Code supplied apart from a service's own source that the service or its tests build with, named in the architecture document with its exact version; what a named library itself needs to build comes with it. The service entry and `testing` declare `libraries` as lists of `{module, version}`. Before a writer turn, the run downloads every named Go module and its build graph; a download failure stops the run, naming the library and error. Writers read only that graph and its exact version metadata from the module cache. A rebuild may use nothing else, and a service may not use a library named only for its tests. For non-Go hosts the lists remain empty; existing Jest and Playwright paths are unchanged. Only codex enforces the access and network boundary for subprocesses; Claude's file tools obey it, but its replay remains unconfined with network for model trials.
_Avoid_: dependency (a service's dependencies are the processes and CLIs it talks to while it runs), package, framework

## Repository vocabulary

Words for how this repository plans and executes its own work. Not the engine's domain — nothing here reaches a host project.

**Console**:
A program's ANSWER — bytes a caller reads verbatim in a shape it fixed, plus the engine's prompts and the reader they are answered on. Not narration: what a program did while it worked is a Log. Byte-plain forever, because the lint hook's stdout is parsed as JSON. `services/true-bdd/internal/infrastructure/console` implements the engine's Console; `pkg/console` implements it independently for tests and tooling. Each owns its program's three standard descriptors; `depguard` denies it to `scripts/` outright, which narrates and never answers.
_Avoid_: output, print, terminal, UI

**Log**:
One `log/slog` record: everything a program says ABOUT its own run. Severity is a level, never a choice of stream — the stream is bound once per program in `main()`, and every `scripts/` program stamps each record with its writer in `tool` and its process in `run`. `services/true-bdd/internal/infrastructure/logging` implements the engine's Log; `pkg/logging` implements it independently for tests and tooling; the engine's message strings and attribute keys are a wire contract `pkg/testkit/reporter` reads.
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
