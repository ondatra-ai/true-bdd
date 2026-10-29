# TrueBDD

An aspirational **Spec-as-Source** CLI: Gherkin-style behavioural specs
plus a system-architecture description are the source of truth, and code
is a regeneratable artifact whose *observable behaviour* — not its
byte-for-byte shape — survives a rebuild. Today the tool operates one
level down — **Spec-Anchored** — driving Claude-mediated checklists over
user stories.

Built by [Peter Ovchyn](https://ovchyn.me).

## Table of contents

- [Background — the three levels of SDD](#background--the-three-levels-of-sdd)
- [Vision](#vision)
- [Status](#status)
- [Install](#install)
- [Usage](#usage)
- [Configuration](#configuration)
- [Testing](#testing)
- [How it compares](#how-it-compares)
- [References](#references)
- [Contributing](#contributing)
- [License](#license)

## Background — the three levels of SDD

The de-facto taxonomy in the SDD literature (arXiv 2602.00180,
Piskala 2026) splits spec-driven development into three patterns:

| Level | Source of truth | Code edits | Canonical tools |
|---|---|---|---|
| **Spec-First** | Code (after release). Spec only starts the process. | Hand-edited. | Cursor + rules, early Spec Kit. |
| **Spec-Anchored** | Code, but spec is a living contract. CI validates code against spec. | Hand-edited; spec updates via review. | GitHub Spec Kit, Kiro, BMAD, OpenSpec, current Tessl, LeanSpec, Augment Intent. |
| **Spec-as-Source** | Spec. Code is derived. | Forbidden — edit the spec, regenerate the code. | Tessl (historically, via `tessl build`), **TrueBDD** (aspirational). |

The test that distinguishes them: *Can you delete all the code,
regenerate from the spec, and have the new build pass every behavioural
assertion the spec carries?* For Spec-as-Source the answer is **yes by
design**. The regenerated code is free to differ in structure, naming,
even in which auxiliary endpoints exist — what survives a rebuild is
the **behaviour**, not the bytes.

## Vision

Pre-AI, code was inevitably the only authoritative artifact: developers
discover detail mid-implementation, work around third-party limits,
absorb UAT feedback, and the spec quietly diverges. The arrival of
coding agents doesn't fix this — agents drift too, and a stakeholder
can still ask for a checkbox where a dropdown was specified. Code
remains the only place the system's real behaviour is pinned.

Spec-as-Source flips that contract. To make it work, the spec system
has to carry enough information for an AI to reconstruct the code:

- **Behavioural spec** — Gherkin (or a Gherkin-shaped DSL) describing
  user-visible behaviour, *not* code structure. Every scenario becomes
  an executable test that any regenerated build must pass.
- **Architectural spec** — services, data models, transport protocols
  (REST / GraphQL / etc.), endpoints, and the persistent contract
  (what survives a rebuild). Docker Compose YAML is a natural fit for
  the service shape.
- **Regeneration loop** — the AI is allowed to invent absent endpoints
  and internal code paths to satisfy the spec, so two rebuilds from
  the same spec will not produce byte-identical code. What it *cannot*
  invent are the persistent contracts (data models, exposed endpoints)
  declared by the architecture — those are pinned across rebuilds.
- **BDD tests as oracle** — derived from the behavioural spec, they
  are the *definition* of a correct rebuild. If every scenario passes,
  the rebuild is acceptable, regardless of how the code differs from
  the previous build.

The contract Spec-as-Source promises, then, is **behaviour preservation
under regeneration** — not byte-identical regeneration. The behavioural
spec defines observable outputs; the architectural spec pins the
persistent contracts; the implementation in between is free to drift
between rebuilds, as long as both contracts are honoured.

`true-bdd` is the substrate this vision is being built on. The `us`
subcommand suite manages the spec lifecycle, starting from a Story the
Product Owner writes directly, out of a discussion of a Feature, into
`docs/product/stories/`; the `build` subcommands
implement the pipeline `us refine`, `us apply`, `build tests`, `build steps`, `build code`. `build tests` renders Registry tests, `build steps` checks their definitions, and `build code` repairs production code until the tests pass.

## Status

| Subcommand | State |
|---|---|
| `us refine <id>` | **Working** — the pipeline's first step: walks a hand-written Story against `us-refine`, saving it in place when all Prompts pass. Every shipped Prompt has an `F:`; `--fix` proposes added or rewritten Acceptance Criteria or a Catalogue entry, applied only after the Product Owner answers apply at the menu. Stories still need 3 to 7 criteria. Each real failure mode needs its own criterion; a Story with none passes with a context line explaining why. When a Catalogue is declared, each step must match exactly one phrase in its declared block; `D: step-phrases` runs first and its fix adds a missing phrase to the product document while the Story keeps its words. A custom checklist missing an `F:` still refuses `--fix`. |
| `us apply <id>` | **Working** — walks every AC in a refined story, validates against `us-apply`, and merges scenarios into the central `docs/scenarios.yaml` registry. |
| `build tests` | **Working** — deterministically renders Registry test files. Without `--fix`, stale files refuse startup; `--fix` writes them. Registry, path, renderer, step-shape and Catalogue phrase refusals precede rendering. No AI turn, fix menu or test runner. |
| `build steps` | Lists missing phrase definitions and checks Step coverage in-process. `--fix` writes every skeleton first, then walks each phrase through the Fix loop to write only its body and shared state. After each apply it checks compilation, one binding and a written body, and, once those pass, separately critiques the definition's body against its phrase and mapping; after all Items it checks whole-suite coverage, lists every generated test and requires every Scenario to fail at an executed step in a copy without the service paths. Green Scenarios return their written Items to the bounded Fix loop; bring-up failures, skipped tests and missing results fail the check. Writes only protected test paths: a codex writer through its sandbox, a Claude writer through the `claude-guard` hook, as `build code`'s writer is. An interface that cannot serve a phrase stops distinctly; incomplete walks exit non-zero. Success prints `BUILD STEPS COMPLETE`. |
| `build code` | **Working** — runs the architecture's declared replay command and walks the whole failing set as one Item. An engine-owned `D:` verdict needs no validation AI turn. With `--fix`, one plan and one writer repair the set; the writer receives Specification context, service paths and exact `writable` paths, and codex or Claude whole-suite and selected replay forms. After every completed apply the engine reruns the remaining set in one start; passing members leave and missing results stay failing. A successful writer response missing its confirmation warns once with the result path and follows the same rerun as `applied: false`, keeping files and consuming the same allowance. The terminal apply is rerun but still stops at the existing bound even if it passes. Started codex writers that exit non-zero or answer empty, and Claude writers that exit non-zero, return an error result or answer empty, follow the same recovery without a stall watch. Start failures and engine refusals before spawning remain fatal, as do other roles' failures. The writer ends its final message with the confirmation block; the engine saves the block at the heading's result path, which the writer never writes. Fresh discovery on each outer walk catches regressions. Tests and the Registry remain unchanged. |
| `scen check [id...]` | **Working** — walks registry scenarios against the `scen-check` checklist, one cell per (scenario, prompt): each prompt reads one scenario's own fields — description, service, path, lineage, steps — and rules on that scenario alone, so no prompt can ask a cross-registry question. Exits non-zero when any scenario fails, so it can gate a change. With no argument every scenario is walked; ids narrow the walk to those scenarios, in ascending id order. |

Every command accepts `--fix` for an interactive loop in which
Claude proposes edits for each failed check and the user applies,
refines, or exits. `--fix` is refused at startup when a prompt the walk
would visit carries no `F:` fix template; every prompt of the shipped
`scen-check` checklist carries one. A `scen check` fix for a failed
closure check defines the missing term in the product document, or maps
it in the architecture document, never in the Scenario; a fix for a
failed model question rewrites the Scenario's own registry entry, or the
meaning of the catalogue phrase it uses (see "The step-phrase catalogue"
below).
`build tests`, `build steps` and `scen check` also take
`--requirements <path>` and `build code` takes `--architecture <path>`
to override the configured spec locations (`documents.scenarios_yaml`
and `documents.architecture_yaml` in `true-bdd.yaml`).

## Install

<!-- install-guide:begin -->
### Install TrueBDD

A release is the tag `services/true-bdd/vX.Y.Z`. Its release notes repeat this guide with the version and the release's commit `<commit>` filled in.

**Machine requirements.** Go 1.25 or later, `git`, `bash`, and a signed-in `claude` and `codex` on `PATH`. A Jest host also needs Node and npm, a Playwright host Chromium as well, and a host whose tests start containers Docker as well.

**From a release archive.** Download `true-bdd_vX.Y.Z_<os>_<arch>.tar.gz` for your platform (`<os>` is `darwin` or `linux`, `<arch>` is `amd64` or `arm64`) and `checksums.txt` from the release. Check the archive, unpack it and put `true-bdd` on your `PATH`:

```sh
grep " true-bdd_vX.Y.Z_<os>_<arch>.tar.gz$" checksums.txt | shasum -a 256 -c -
tar -xzf true-bdd_vX.Y.Z_<os>_<arch>.tar.gz true-bdd
mkdir -p "$HOME/.local/bin" && mv true-bdd "$HOME/.local/bin/"
true-bdd version
```

`$HOME/.local/bin` must be on your `PATH`. `true-bdd version` prints `true-bdd version vX.Y.Z commit <commit>`.

**Or with `go install`.** It installs a binary named `true-bdd` into `GOBIN`, or `$(go env GOPATH)/bin`. A module build carries no commit, so its `true-bdd version` reports `vX.Y.Z` and `commit unknown`:

```sh
go install github.com/ondatra-ai/true-bdd/services/true-bdd@vX.Y.Z
```

### Set up the project

In the project's root, write the configuration, the checklists and the templates. `init` never overwrites a file that exists:

```sh
true-bdd init
```

Then name, in the architecture document, the libraries the code may use: each service's `libraries`, and `testing.libraries` for the tests, each a `module` with an exact `version`. The engine downloads them before a writer turn, and writers read only those.

**A Go host on TrueBDD's test harness.** The test library is not released on its own. Take it at the release's commit, read the version Go resolved, and name that module and version in `testing.libraries`:

```sh
go get github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo@<commit>
go list -m github.com/ondatra-ai/true-bdd/pkg
```

A Jest or Playwright host installs its own test dependencies as usual.

### A first run

The project must be a Git repository:

```sh
git init
true-bdd build code --fix
```

`build code --fix` runs the tests, proposes a fix for what fails and stops at each proposed fix for you to answer. Nothing is applied before you choose to apply it, and there is no way to approve fixes in advance.
<!-- install-guide:end -->

### Build from source

In a checkout of this repository, with Go 1.25 or later:

```bash
mkdir -p ./bin && GOWORK=off go -C services/true-bdd build -o ../../bin/true-bdd .
```

## Usage

The tool spawns `claude` as a subprocess. If you invoke it from inside
a Claude Code session, unset `CLAUDECODE` first so the child has a
clean environment:

```bash
env -u CLAUDECODE ./bin/true-bdd us refine scen-check --fix
env -u CLAUDECODE ./bin/true-bdd us apply  scen-check --fix
./bin/true-bdd build tests --fix
env -u CLAUDECODE ./bin/true-bdd build steps --fix
env -u CLAUDECODE ./bin/true-bdd build code  --fix
env -u CLAUDECODE ./bin/true-bdd scen check scen-check-003-unknown-id-refused
```

`us refine` issues many sequential Claude calls and typically takes
~5 minutes end-to-end. Don't abort early.

## Configuration

Run `true-bdd init` in the host project's root to create its configuration,
all checklists and all prompt templates in `templates/`. It needs no configuration,
reports every created or existing file, and never overwrites an existing file,
even an empty or invalid one. Run it again to add missing files. It creates no
Specification documents or schemas and detects no test framework.

The binary carries copies only for `init`. Commands read the project's files
exclusively, with no embedded runtime fallback. Before any test run, Item or AI
turn, checklist commands load their configuration and checklist. Commands running AI parse their system and
user validation templates; their `--fix` runs also check fix-generator and fix-applier
templates. `build tests` needs no checklist or prompt templates; `build steps --fix` reads its checklist, body-writing templates and `build_steps_critique` / `build_steps_critique_system` templates. A missing file stops startup, naming the file and `true-bdd init`.
An invalid file stops startup with its name, parse error and advice to fix the
file; `init` preserves existing files. `version` and `init` need none of these files.

`true-bdd/` holds the initialized files and optional host-authored schemas:

- `true-bdd.yaml` — the model tiers (below), filesystem paths under
  `paths:` (`stories_dir`, `checklists_dir`, `tmp_dir`,
  `tmp_glob`), per-command prompt-template paths, and a `documents:`
  map naming the files a check may cite (`product`, `architecture_yaml`,
  `scenarios_yaml` — the scenario registry). Each check in a checklist
  lists the document keys it needs under `docs:`, and the engine points
  the prompt at those files.
- `checklists/` — one checklist per checklist command, named by hyphenating the
  command path (`us refine` → `us-refine.yaml`, `build steps` →
  `build-steps.yaml`, …). Each prompt is a `Q:` a model answers or a
  `D:` the engine answers itself (below).
- `*-schema.yaml` — Yamale schemas pinning the shape of the spec
  documents (`product.yaml`, the scenario registry, `architecture.yaml`);
  the host project validates them in its lint step, outside the engine
  itself. These schemas are optional host-authored files; `init` does not create them.

The host default configuration omits `scripts:`. This repository keeps that section
in its own `true-bdd/true-bdd.yaml` for its tooling. The `Init defaults drift` gate
holds the init copies to the repository checklists and prompt templates byte for
byte and to the configuration values after removing `scripts`.

`engine.scen_check_concurrency` bounds the number of Scenarios validating at once in
`scen check`. `init` writes `1`; an older configuration omitting it retains the
sequential walk. Set a positive integer such as `10` to overlap validation turns.
Within each Scenario, all `D:` Prompts precede sequential `Q:` Prompts, and a failed
`D:` skips its `Q:` Prompts. Above 1, fix menus run after validation, one at a time
in ascending Scenario id order. An applied fix discards pending verdicts and
rejudges the whole walk, within `max_apply_attempts`. At 1, the existing cell-by-cell
validation and fix walk is preserved.

### Deterministic prompts

A checklist prompt is either `Q:`, a question a model answers, or
`D: <check>`, which names one of the engine's built-in checks and is
answered by the engine's own code, with no AI turn (ADR 0015). A check's
parameters go under `with:`; `rationale`, `docs` (the documents the
check reads) and `F:` keep their meaning, as do `fix_model` and
`apply_model`:

```yaml
validation_prompts:
  - D: given-when-then   # scen check: the scenario has given, when and then steps
    rationale: "…"
  - Q: "…"
```

Within an item every `D:` prompt runs first, in file order, then every
`Q:` prompt, and an item failing a `D:` prompt skips its `Q:` prompts,
reported as SKIP, so no turn is spent on it. A `D:` verdict has a
model's shape (pass or fail plus context lines), and under `--fix` its
`F:` drives the same fix loop. The engine refuses at startup a prompt
carrying both `D:` and `Q:` or neither, a `D:` naming a check the
command does not hold, a `with:` parameter the check does not read, and
`model:` on a `D:` prompt, since no model answers it, and a `D:` whose
`docs:` does not list a document its check reads. `scen check` holds seven
checks, `given-when-then`, `step-phrases`, `declared-terms`,
`term-mappings`, `phrase-mappings`, `public-interface` and `step-definitions` (below); the shipped `scen-check`
checklist walks the last six, its six Closure checks.

### Parking a prompt

`skip: true` parks a prompt, `Q:` or `D:`: the walk leaves it out as if
it were not in the file, so it costs no AI turn, takes no report row and
no position in the artifact names, and none of the startup checks above
look at it. `skip: false`, or no `skip:`, walks the prompt as usual:

```yaml
validation_prompts:
  - Q: "…"
    skip: true    # parked: not walked, no AI turn
  - D: given-when-then
    skip: false   # walked, as with no skip: at all
```

`skip:` takes only a YAML boolean. Any other value is refused at
startup, naming the prompt, since the engine would have to guess what it
meant: the quoted `"false"`, which once parked the prompt it was meant
to keep, and `yes`, which YAML reads as a string, among them. A
checklist whose every prompt is parked is refused like one with no
prompts, rather than passing every item unchecked.

### The step-phrase catalogue

The product document may declare a `catalogue:`, the shape each Given,
When and Then step may take and what it means. `D: step-phrases` (with
`docs: [product]`) holds every step of a Scenario to it:

```yaml
catalogue:
  - template: 'the engine dispatched {turns:number} AI turn[s]'
    block: then            # given, when or then: where such a step may sit
    means: >-
      The engine sent exactly that many AI turns during the run; "no"
      means none, so the run cost no model call.
    terms: [engine, AI turn]   # the glossary words the meaning relies on
```

A template is literal text plus slots: `"{name}"` (a quoted run, between
literal double quotes), `{name:number}` (digits, or `no` for zero),
`{name:role}` (one of the product's `roles[].name`) and `{name:pattern}`
(the rest of the step, last token only); `[a|b]` is one of the
alternatives, `[s]` an optional `s`, and `\{ \} \[ \] \\` are literal. A
step's whole text, without its `and:`/`but:` keyword, must match exactly
one template in the whole catalogue, declared for the step's block. A
step matching none or several, or one declared for another block, fails
its Scenario with a context line naming the step and the templates; a
template the grammar refuses fails every checked Scenario. Steps opening
with `llm:` or `judge:` are exempt, since a model runs them.

`build steps` lists missing definitions for phrases mapped to the suite's service or used by its Scenarios, including mapped unused phrases. It checks every concrete Registry step separately; a definition covering only part of a phrase still fails when a Scenario uses an uncovered value. Without `--fix`, it reads definitions without compiling or running the suite. With `--fix`, it writes all skeletons before AI, then writes each body through the Fix loop and checks compilation, binding and completion after each apply. Once an apply's compilation, binding and body checks pass, a separate AI critique turn reads the applied definition's body (the engine's registration, pattern and signature are not judged) with the whole phrase meaning, Phrase mapping entry, term glossary entries and Word mappings, Public interface declaration and document, and every suite Scenario using the phrase. It checks the mapping by block (when calls its named operation, given sets up the stated state, then observes and compares as stated), assertions against the phrase meaning, and that the definition does not compute what it asserts. Failures name the definition and discrepancy and return to the same Fix loop within its configured attempts. The turn uses build-steps checklist engine.critique_model, falling back to engine.default_prompt_model, and states no time, turn, token or cost budget. After all bodies, whole-suite coverage and a generated-test listing pass, then it runs the suite once in a Host project copy without any service path. Every Scenario must fail at an executed step definition; compilation, bring-up, skipped and missing results fail the check and name the Scenario. Green Scenarios return all Items written in this run for their phrases to the Fix loop together, with the finding and existing attempt counts. If none of those Items can repair a green Scenario, or attempts are exhausted, the command fails distinctly. The original Host project is preserved. The writer changes only protected tests and shared state; a declared interface that cannot serve a phrase stops with its missing setup or observation.

`D: declared-terms` (also `docs: [product]`) holds the words those
meanings rely on to the product document's `glossary:`, each entry a
`term` and what it `means`. Every term the one template a step matches
declares must be defined exactly once, spelled exactly as the catalogue
spells it: a term the glossary lacks fails the Scenario naming the
template and the term, and a term defined twice fails it naming the
count. A term only an unmatched template declares is no failure of the
Scenario.

`D: term-mappings` (`docs: [product, architecture_yaml]`) holds the same
terms to the architecture document's word mapping, `architecture.terms:`,
which says how each product word is built and how a step definition
observes it, in words that name no code identifier:

```yaml
architecture:
  terms:
    - term: AI turn
      technical: >-
        One request through the engine's AI router to claude or
        codex, logged once as msg="Dispatching AI turn".
      observed_by: >-
        Count the "Dispatching AI turn" records in the run's JSON log file.
```

Every term must be mapped exactly once: an unmapped term fails the
Scenario naming the term, and a term mapped twice fails it naming the
count.

`D: phrase-mappings` (`docs: [product, architecture_yaml]`) holds each matched phrase to `architecture.phrases`, one entry per verbatim Catalogue template and service. An operation entry has `references` and a `does` sentence saying what the step does by block: a `when` calls an operation, a `given` sets up state, and a `then` observes and compares. Each reference has exactly one selector: `command` is the CLI command path (the exact `commands` key), `file`, `log_record` and `outgoing_call` name keys under the CLI document's `extensions.files`, `extensions.log_records` and `extensions.outgoing_calls`; `operation_id` names an HTTP `operationId`; `element` names a UI element by page or region, role and accessible `name`.

```yaml
architecture:
  phrases:
    - template: 'the number is halved'
      service: calculator
      references:
        - command: number halve
      does: Call number halve once and keep its exit code and standard output.
```

An entry can instead carry `intent`, a sentence replacing `does` and `references`. The check accepts intent only for UI services until `llm:` support lands. Missing or duplicate entries, empty operation entries and references absent from the service's Public interface fail the claimed Scenario, naming its phrase and reference, without an AI turn. The check uses the same Catalogue and Story exemptions as the term checks. Its `F:` first asks the System Architect what the step does, proposes the entry at the fix menu, with any declaration the answers name that the service's Public interface document lacks, and after apply writes the architecture document and that document. Each fix to one of these two checks works on run working copies of the architecture and of every cli or http document it names; a changed document must still parse in the form of its kind, and is committed before the architecture. The architecture stores no Go identifier or link to a step definition. The Word mapping continues to map terms; `build steps --fix` embeds Phrase mappings in each Item’s body-writing context.

`D: step-definitions` (`docs: [product]`) holds the catalogue to the
suite's step definitions, the `steps/` package beside the Scenario's test
file, read without running it as `build steps` reads it. Over the steps
of every Scenario a Story claims in that suite, whatever ids the walk
selected, the two must correspond one to one: a template whose steps bind
two definitions, a definition two templates reach, and a step two
definitions bind each fail the Scenario, naming the definitions by
pattern, file and line. A step no definition binds yet is a gap `build
tests --fix` authors, reported but not failed. A pattern that does not
fold to a constant stops the walk with `msg="Check could not answer"`.

Adoption is opt-in and gradual. With no `catalogue:` key every Scenario
passes "not checked: the product document declares no catalogue". With
one, a Scenario is checked once a Story claims it (a non-empty
`user_stories`); until then it passes "not checked: no Story claims this
Scenario yet". `build tests` is stricter: once a catalogue is declared it
holds every registry step to it, claimed or not, and refuses to start on
a step outside it. `us refine` always checks the Story it walks, one
context line per acceptance criterion. Under `--fix` a failed check's
`F:` drives the usual clarification loop, and the fix is written to a
scratch copy of the product or architecture document in the run
directory, which the engine puts in place of the document once it
parses: the Scenario, the Story and the step definitions are never
edited.

What no check can settle, a model answers. The shipped `scen-check`
checklist follows the six closure checks with two `Q:` prompts that
read the product and the architecture document: whether the meaning of
each phrase a Scenario's steps use says what is set up or observed (a
pattern slot's phrase must say which output its pattern is held
against: an answer line, or a log record's message or fields), and
whether the Scenario uses each phrase in that meaning. They pass a
Scenario no Story claims, or a product with no catalogue, as "not
checked", and they cost a turn each, so a Scenario failing a closure
check skips them. Its eight quality questions follow, on the
description and the steps. Under `--fix` a model question's fix
rewrites the meaning in the product document, or the Scenario's own
entry in a scratch copy of the registry, which the engine puts in place
once it loads with the same Scenarios; the walk then restarts on the
Scenario as it now reads, and `build tests --fix` re-renders its test.

### Model tiers

Every checklist cell runs up to three AI turns with genuinely different
needs: **validating** a `Q:` wants strong reasoning, turning a failure
into a **fix prompt** wants a mid model, and **writing the fix** wants a
cheap high-context coder. The best model for each may live behind a
different CLI, so the engine names four tiers and binds each to a
`"<cli>:<model>"` pair — then gives each of the three roles its own
default, because one fallback tier cannot serve turns this different:

```yaml
engine:
  models:
    xhigh: "claude:claude-fable-5"
    high: "claude:claude-opus-5"
    coder: "codex:gpt-6.1-sol"
    quick: "claude:claude-sonnet-5"
  default_prompt_model: high
  default_fix_model: high
  default_apply_model: coder
```

**quick** is the fourth tier: a cheap, fast graded answer with a short
reason. No role defaults to it — the BDD harness's judge resolves its
model from it, and a checklist may name it for a low-stakes cell.

The value splits on the **first** colon, so hyphenated and
provider-qualified model ids survive intact. Supported CLIs are
`claude` and `codex`; each must be on `$PATH`.

Every command that runs AI turns (`us refine`, `us apply`, `scen check`,
`build steps`, `build code`) accepts repeated `--model <tier>=<cli>:<model>`
flags. Each replaces only that declared tier's binding for this run, after
validating the loaded config. An undeclared tier, malformed reference,
unknown CLI or repeated tier refuses startup with exit 1 before any turn.
References use the same first-colon split and validation as the config.
Each applied override logs once at INFO, `msg="Model tier overridden"`,
with `tier`, `from` and `to`. Overrides live in memory; apart from logs,
they write no file and add nothing to a spawned process's environment.
The resolved model reaches its AI CLI through that CLI's model argument.

```sh
true-bdd build code --fix --model coder=claude:claude-sonnet-5-5
```

The drill accepts repeated `--model <tier>=<cli>:<model>` pairs after
`build steps --fix` or `build code --fix`, records them in the run's
arguments and forwards their values unchanged through typed options.
It refuses other extra arguments, and any after `build tests --fix`; the engine validates the values.
Deterministic `build tests` dispatches no AI turn and has no `--model` flag.

A checklist picks a tier per role, and any single prompt overrides it:

```yaml
engine:
  prompt_model: xhigh   # the validation turn
  fix_model: high       # failure → fix prompt
  apply_model: coder    # writes the file
  critique_model: high  # build-steps only: reviews every applied definition

sections:
  - id: test-passes
    validation_prompts:
      - Q: "…"
        model: high         # overrides prompt_model for this cell
        fix_model: coder    # optional
        apply_model: coder  # optional
```

Resolution runs **prompt → checklist → the role's engine-level
default**, so a checklist that names nothing still gets `coder` for the
turn that writes files and `high` for the one that validates. A tier
name that is not configured, an unknown CLI, a missing role default, and
a default naming an unconfigured tier are all startup errors — the
engine never silently substitutes a different model than the checklist
asked for.

**Permissions across CLIs.** Tool permissions are declared once, as the
engine's execution mode, and projected onto each CLI:

| | `claude` | `codex` |
|---|---|---|
| System prompt | native flag | prepended to the prompt |
| Write gate | `--allowedTools`; generated hook for the build-code writer | named profile for every apply; `-s read-only` for plan/validation |

Only `build code --fix`'s writers receive whole-suite and selected replay forms. Claude's guard matches the canonical command exactly, with an optional nonempty single-quoted selector, in the resolved suite directory. An argument containing a single quote, newline or NUL withholds the grant with one warning. Claude requires foreground execution; codex polls its native process sessions. Tests execute writer code with the machine's permissions under Claude; codex commands additionally run inside its permission profile.

The default writer is `codex:gpt-6.1-sol`; `default_apply_model` remains `coder`. Every codex apply turn uses a generated home and permission profile scoped to its own roots and scratch (ADR-0024). Plan and validation turns stay `read-only`. Only build code's writer receives replay command forms and started-turn recovery.

For a Claude writer, use a tier bound to `claude:claude-sonnet-5-5` (native
1M context on Claude Code 2.1.284 or later). The confined writer requires
Claude Code 2.1.285's `--restricted` mode. It uses generated `--settings`
instead of safe mode: only the engine's `PreToolUse` hook grants Bash, with
ADR-0018's exact command check and background execution denied. Both Bash timeouts
are 600,000 ms. The writer has no engine turn deadline; cancelling the command
stops its process group. Its file tools reach the host root, installed Go
toolchain/module cache except `github.com/ondatra-ai` copies outside the host,
and only its own session's tool-result spill directory. Writes reach scratch
and declared service roots. Searches outside the host reject ancestors of excluded module copies; Glob also checks its fixed pattern root and denies parent traversal. The host root always remains accessible, including
when it lies under `~/work`. User/project memory, skills, installed plugins,
commands, workflows and MCP remain disabled. Existing sign-in is used; a fresh
`CLAUDE_CONFIG_DIR` was signed out in the live probe. No credentials are copied.
A started Claude writer's non-zero exit, error result (rate/usage limits included)
or empty answer preserves edits, warns once and reruns the set. Available cost
and token counts are logged even on failure. Start failures remain fatal.

Every codex apply turn uses a generated `CODEX_HOME` and
named permission profile (ADR-0021 and ADR-0024), with high reasoning and a configured
1,050,000-token window. This configuration does not verify 1M capacity.
The home copies only sign-in credentials and is denied to writer commands;
user config, MCP, web search, history and AGENTS.md are suppressed. Reads
reach the host, OS runtime, Go toolchain and module cache excluding
`github.com/ondatra-ai` (including download archives) and the entire module-cache `cache/vcs` tree. All generated homes share a denied `tmp/codex-homes` subtree; authentication is removed after each turn, including failures. Writes reach the turn's single-file, directory or configured-glob roots and host `tmp/`; `HOME`, `TMPDIR`, `GOCACHE` and controlled Go
flags are set there. Subprocess network is off and `GOPROXY=off`.
The profile is calibrated on macOS with codex-cli 0.162.1 and has no weaker
fallback. Codex's `--json` events are saved beside its transcript and read
by drill audit as references, not proof that reads succeeded. The final
message still comes through `-o`. Only build-code started non-zero or empty codex writer
turns preserve edits and rerun the set with one warning; start failures
stay fatal. No codex stall watch is added. Plan and validation retain their
`read-only` sandbox and recorded requests. Other commands' applies remain fatal on failure.
Optional installed node, npm (including npx) and Docker clients have explicit runtime read grants. npm stays offline, runtime state stays in scratch, and no Docker daemon/socket is granted. Configured globs allow creating matching files even when no matches exist before the turn. Literal files and terminal `/**` subtrees keep exact grants; other globs temporarily grant their narrowest literal directory and enforce the original patterns after every turn, including failures. Newly created non-matching files are removed. Changes, deletions, renames or permission changes to pre-existing non-matching files fail the turn with one warning naming the file; changed contents remain for diagnosis. Overlapping grants authorize their union. This is a post-turn boundary: non-matching files under the granted directory can be changed during the turn. Malformed patterns and grants of the host root, protected generated homes or escaping paths are refused. ADR-0024 lists library and installation-link exceptions.

The host project's documents live under `docs/`:

- `docs/architecture/architecture.yaml` — the architectural spec that
  scopes `build code` (which test suites get walked, and how each one
  runs — see below), tells `build tests` which suite owns a scenario,
  and maps each product word onto the implementation under `terms:`.
- `docs/product/product.yaml` — the product document, including its
  `roles:`, the BDD `vocabulary:` (forbidden qualifiers and forbidden action verbs) the `us refine` and `scen check` Prompts cite, its step-phrase `catalogue:` and the `glossary:` defining the terms the catalogue declares. These are roles (responsibilities), not personas (invented
  individuals) — one person commonly holds two of them.
- `docs/product/stories/*.yaml` — the stories, written by hand out of a
  discussion of a Feature, that `us refine` / `us apply` read, one
  `<id>-<slug>.yaml` each. No command creates one. A story id is
  a slug (`scen-check`, `96.2`), and a story may carry a `feature:` label
  that `us refine` keeps through every rewrite.
- `docs/scenarios.yaml` — the scenario registry `us apply` merges into,
  `build tests` renders, and the test suites RUN: each scenario is a test,
  its steps bound to code by the suite that owns it. A scenario is keyed
  by its lineage, `<story-id>-<ac-id>`, and the author writes each
  acceptance criterion's id as `<NNN>-<slug>` (ADR 0014), so the key
  `scen-check-001-unknown-id-refused` survives reordering the story;
  `us apply` refuses a criterion id of any other shape.

Prompt templates live in [`templates/`](templates/) (Go `text/template`
with sprig).

### How a test suite runs

The architectural spec has one `testing:` section for the whole project,
beside the services it exercises — how a project runs its tests is a
property of the project, not of any one service. `build code` does not
know how to run your tests; the spec says. There are no suites: which
service a test belongs to and which file it lives in are the scenario's
own `service:` and `path:`, and its tests live in `tests/<service>/`.
The `commands:` block carries one complete command line per
AI-dependency mode:

```yaml
architecture:
  testing:
    framework: go-test            # go-test | jest | playwright
    commands:
      record: "go test -tags bdd -json -count=1 ./tests/true-bdd/ -mode=services:record,tests:record"
      replay: "go test -tags bdd -json -count=1 ./tests/true-bdd/ -mode=services:replay,tests:replay"
      live: "go test -tags bdd -json -count=1 ./tests/true-bdd/ -mode=services:live,tests:live"
  services:
    - name: true-bdd
      path: services/true-bdd
      language: go
```

Each service may declare `public_interface`. It is optional in the schema so an older architecture loads, but `scen check`'s `D: public-interface` (`docs: [architecture_yaml]`) fails every Scenario whose service has no valid declaration, naming the service without an AI turn. This check runs even without a Catalogue or Story. Its `F:` asks the System Architect what the service offers, then writes the architecture document, and an existing document it names where the answers say that document lacks what the service offers, never the Scenario.

```yaml
public_interface:
  kind: cli
  document: docs/greeter-cli.yaml
```

`document` is relative to the Host project root. CLI documents use this supported OpenCLI-like draft shape (YAML or JSON): `opencli` and `info: {title, version}` identify the document; `commands` maps command names to `parameters` (with `name` and optional `in: argument|flag|option`, `schema: {type: ...}`, `description` and `required`) and `responses` (exit-code keys mapped to objects with optional `description` and `content`). This follows the [OpenCLI reference](https://github.com/nrranjithnr/open-cli-specification/blob/main/opencli.spec.json); the referenced project currently advertises `opencli: "1.0.0"`. Every command declares responses for its exit codes. Optional `extensions` holds maps keyed by stable names the Phrase mapping can reference:

```yaml
opencli: "1.0.0"
info: {title: Greeter, version: "1.0.0"}
commands:
  greet:
    parameters:
      - {name: person, in: argument, required: true, schema: {type: string}}
      - {name: loud, in: flag, schema: {type: boolean}}
    responses:
      '0': {description: Greeting printed}
      '1': {description: Invalid input}
extensions:
  files:
    greeting-config: {path: greeter.yaml, format: YAML mapping, access: read}
  log_records:
    greeted: {message: Greeting printed, format: logfmt with person field}
  outgoing_calls:
    notify: {command: notifier, arguments: [--person, "{person}"]}
```

File `access` is `read`, `write` or `read-write`; each file has a path and format, each log record a message and format, and each outgoing call a command and optional argument list. `kind: http` names an OpenAPI 3 document instead. `kind: ui` has inline `elements`, each with `role`, accessible `name` and a `page` or `region` (both may be present); the same role and name must be unique within that scope. CLI/HTTP declarations have no elements; UI declarations have no document. There is no library kind; another process-boundary kind is reported by name as not supported yet.

TrueBDD declares its own Public interface the same way: the `true-bdd` service names `docs/architecture/true-bdd-cli.yaml` (`kind: cli`), and `architecture.phrases` maps every phrase of its Catalogue that a Scenario uses to that document. The document lists every command with its arguments, options and exit codes, the hidden `claude-guard` hook, and, under `extensions`, the files the engine reads and writes in a Host project, the log records its Scenarios observe and its outgoing calls to `claude`, `codex`, the declared suite command and the Go toolchain. A statement the architecture already holds keeps that home, and the document points to it: the log vocabulary under `architecture.terms`, the IO contract under `patterns.owned_io`, the AI CLIs' invocation contracts under the service's `dependencies`, and the templates' inputs under `prompt_data`. The drill keeps the document in every cut because the declaration names it.

Named documents are part of the Specification. Commands reading the architecture load and parse them at start, refusing a missing or invalid document with its filename and error before any test run, Item or AI turn. HTTP documents and their local `$ref` documents are validated with the architecture's exact-version OpenAPI library; references outside the Host project or to network URLs are refused. TrueBDD's own declaration names `docs/architecture/true-bdd-cli.yaml`, described above.

The rules, all enforced at startup rather than discovered mid-run:

- **All three modes are mandatory.** The engine carries no built-in
  invocation to fall back on: a command the spec does not state is one
  no reader can audit. `build code` runs `replay` — no model, no cost;
  `record` and `live` are reserved for commands that do not exist yet.
- **Each command is complete and framework-native**, including the flag
  that makes its output machine-readable (`-json` for go-test, `--json`
  for jest, `--reporter=json` for playwright). Omit it and the runner
  parses prose, finds no failures, and the walk goes falsely green —
  so a command without it is refused.
- **Quoting is honoured, nothing else.** `-run '^TestGreen$'` survives
  as one argument; there is no expansion, substitution or piping. A
  command needing a shell belongs in a script the spec then names.
- **The working directory** is the one holding the suite's `config:`
  file — which is what lets `npx` resolve a suite's own local install.
  A suite that declares no `config:` inherits the directory `true-bdd`
  itself was run from, so its command should be written relative to the
  repo root and the CLI run from there.
- **`service:` names one entry in `services[]`**, and that service's
  `path:` is the only root `build code --fix` may write. A name that
  resolves to nothing grants nothing, so it is refused at startup rather
  than discovered as a fix that never lands. A suite covering two
  services is two suites.
- **Re-running one test** appends only a name filter (`-run`,
  `--testNamePattern`, `--grep`) to that same command, so a rerun keeps
  the build tags and flags that made the test discoverable.

## Testing

```bash
# unit tests
go run ./scripts/cmd/gates test

# end-to-end BDD scenarios — offline replay, no AI calls
GOWORK=off go test -tags bdd -timeout=180m ./tests/true-bdd/... -mode=services:replay,tests:replay
```

The suite's contents come from `docs/scenarios.yaml`: every scenario the
architectural spec assigns to it gets one top-level Go test named from
its key in CamelCase (`scen-check-003-unknown-id-refused` gives `TestScenCheck003UnknownIdRefused`),
written into the file the scenario's `path:` names
by `true-bdd build tests --fix` and by nothing else. Its steps are bound
by regexp to the definitions in `tests/true-bdd/steps/`. A scenario's
Given step names the fixture tree it drives.

**A fixture is a directory, not a document — there is no manifest.**
Trees under `tests/true-bdd/fixtures/<name>/` hold the designed
host-project content under `input/` (`docs/` at minimum, plus project
sources, a per-fixture `CLAUDE.md`, or engine-config overrides under
`true-bdd/` when the scenario needs them) and the recording under
`cassettes/`. What the run DOES — the invocation, the exit code, the
stdout patterns, the interactive answers, the clauses it is judged
against — all comes from the scenario. Three optional files sit at the
fixture root beside `input/`: `prep.sh` (run in the tmpdir before the
pre-run snapshot, e.g. `npm install`), `teardown.sh` (best-effort
cleanup after the post-run snapshot, e.g. stopping a compose stack), and
`checklist-prompts.yaml` (which prompts of a shipped checklist the walk
evaluates). The runner builds the CLI, pre-populates a tmpdir with the
live engine ingredients (checklists and prompt templates), overlays the
fixture's input tree, snapshots, runs what the scenario's When step
names, and asks Claude to score the resulting diff against the
scenario's `judge:` clauses. `-mode` names a mode per caller —
`services:` for the engine under test, `tests:` for that scoring call —
so a run can replay the engine while judging for real, or replay both.
A scenario skips when `claude`, or any CLI a model tier names, is not on
`$PATH`; under `-mode=services:replay,tests:replay` nothing skips, because
no model is spawned at all.

## How it compares

Within the Spec-Anchored tier, comparable projects: **Spec Kit**
(GitHub) leans on a `constitution.md` and a four-phase workflow;
**Kiro** (AWS) bundles specs with steering files inside an agentic
IDE; **BMAD-METHOD** is a 12-role multi-agent framework; **OpenSpec**
treats every change as a spec proposal needing approval; **Tessl**
operates a spec registry over MCP; **LeanSpec** keeps living docs
under 2K tokens with a `validate` command. Of these, only Tessl ever
shipped a true Spec-as-Source mode (`tessl build`, retired Jan 2026).

TrueBDD's bet is that **Gherkin-grade behavioural specs + an
explicit architectural contract** are enough to make Spec-as-Source
tractable again — pinning observable behaviour and persistent
contracts tight enough that the regenerated code's shape can vary
freely between rebuilds without the system's observable behaviour
drifting.

## References

- Piskala, *Spec-Driven Development: From Code to Contract in the Age
  of AI Coding Assistants*, arXiv 2602.00180 (Feb 2026).
- *Constitutional SDD*, arXiv (Feb 2026).
- Augment Code, *6 Best Spec-Driven Development Tools* (Mar 2026).
- ThoughtWorks Technology Radar Vol 33 (2025).
- GitHub Spec Kit — <https://github.com/github/spec-kit>
- Kiro — <https://kiro.dev>
- BMAD-METHOD — <https://github.com/bmad-code-org/BMAD-METHOD>
- OpenSpec — <https://github.com/Fission-AI/OpenSpec>
- Tessl — <https://tessl.io>
- LeanSpec — <https://lean-spec.dev>

## Contributing

Pre-1.0 and direction-finding. Open an issue before non-trivial work.
Lint all three modules with `go run ./scripts/cmd/alint`, using the root `.golangci.yaml`.

## Release rehearsal (repository tooling)

The repository has three Go modules: the root (tests and scripts), `pkg/` (shared libraries and the test library), and `services/true-bdd/` (the engine). Local development uses the root module's `replace` for `pkg/`; release engine builds use `GOWORK=off` and no replacements. Only the engine is released: a release is the engine module's tag `services/true-bdd/vX.Y.Z`, the only form under which Go resolves `go install …/services/true-bdd@vX.Y.Z`. No `pkg/` or root tag is pushed; a Go host takes the test library at the release's commit.

A person pushes the tag. `.github/workflows/release.yml` then runs `release build --tag <tag>`, which refuses any other tag form or a tag that does not name the checked-out commit, the zero-cost rehearsal, and `release notes --tag <tag>`, which writes the install guide above with the version and commit filled in. It creates a draft GitHub release with the four archives and `checksums.txt`, and never publishes it.

This repository's `scripts/cmd/release` repeats the installation path against local artifacts; it does not publish a release or a tag. Run from a checkout:

```sh
go run ./scripts/cmd/release build --version 0.0.0-rehearsal --out ./tmp/release-artifacts
go run ./scripts/cmd/release rehearse --artifacts ./tmp/release-artifacts
```

`build` exports HEAD with `git archive`, cross-compiles with `-trimpath` for Darwin/Linux on amd64/arm64, stamps each binary with the version and the commit through `-ldflags -X` (#220), and writes four binary archives, `checksums.txt` and `release.json` (the source commit and archive version). There is no resources archive: `true-bdd init` supplies the host configuration, checklists and templates.

`rehearse` checks every archive, then follows the install guide above one command for one, each run as written with its placeholders filled: it checks, unpacks and installs the native archive on a private `PATH` (a private `HOME`), holds `true-bdd version` to the stamped version and commit, and installs the engine with `go install` from its module proxy, which must yield a `true-bdd` reporting that version. It copies the calculator and (when Node and npm are installed) Jest fixture inputs into a fresh private folder outside the checkout, deletes their service and resource copies, runs `true-bdd init`, installs test requirements (for the calculator, the guide's `go get` and `go list -m`) and runs `git init`; its first run is the guide's `true-bdd build code --fix`. Unit checks hold the guide's commands in this README to the ones the rehearsal runs. By default its fake Claude and Codex CLIs exit 3; success requires the engine's dispatch log, their sentinel and a non-zero engine exit. Earlier failures name the install, configuration, Specification or suite-discovery step. The copied Jest host also gets a service-load test so a missing module is a failing test member rather than only a suite-load error; the fixture tests stay intact. It retains that folder for inspection and writes the step results and evidence paths to `<artifacts>/rehearsal.json`.

The default harness requirement uses a local `file://` module proxy containing the artifact's exported source, the test library and the engine, at its commit-time pseudo-version, resolving the guide's `@<commit>` and answering `go install`'s `latest` query. The host's `go get`, the engine's download and the `go install` channel use that proxy, with the public Go proxy serving transitive modules. The copied host's `testing.libraries` names the exact installed version, and the writer's read grants name that version in the module cache. No pushed commit is required. To exercise a user's proxy install, add `--harness-revision <proxy-served-revision>`; the host installs through the real proxy and declares the resulting exact version. No release or install guide is published by this tool.

Every rehearsal has separate fresh private module caches for installation and engine execution. The zero-cost calculator pass requires the engine's `Libraries downloaded` record before the first AI turn and retains its module versions and cache paths. Another fresh host declares a nonexistent library; it must stop before any AI turn and name that library. An empty file proxy makes that refusal deterministic.

`rehearse --artifacts ./tmp/release-artifacts --live` runs real AI CLIs with `build code --fix`, answering `1` at each fix menu through stdin. It requires exit 0 and `BUILD CODE COMPLETE`, then runs each host's declared test command. This costs money. The report includes wall time, Claude spend and per-turn denied-call records with available targets; values the engine does not report, including Codex spend, are `unavailable`. The gate runner builds and runs the default zero-cost rehearsal on every pull request; CI keeps no separate gate list.

## License

TBD. Not yet released for external use.

## Build-code writable paths

A service may add `writable: [go.mod, go.sum]` beside its `path` in the architecture. Entries are exact paths from the project root, files or folders, with no patterns; existing folders grant their subtree, and a trailing slash declares an absent folder. Missing files remain exact grants. Build code refuses grants outside the host or overlapping protected tests before running tests or models: `tests/`, Registry suite directories, the in-host `testing.config` directory, and in-host Go command package directories (`./dir` or `./dir/...`, relative to the command's working directory). Broad `./...` names no separate test directory. Package directories within a service's own path, including `_test.go` files, remain writable service code; reserved tests, Registry and configuration directories take precedence. Its writer changes only the failing set's service paths, their writable entries and host tmp. Unknown-service failures use all declared services. Only codex fully enforces this boundary; the Claude writer's test command remains unconfined for model trials.

The plan first seeks a permitted solution. If no solution exists, it returns `impossible: [{path, reason}]`; the writer's confirmation carries the same list for a late obstruction. The first nonempty report keeps written files, reruns the failing set once, then returns a distinct impossible-repair error naming the required files, reasons and remaining failures. Change those files by hand, or add them to the service's `writable`. No further attempt or COMPLETE banner follows. An empty list leaves ordinary `applied: false` retries unchanged.

## Fix approval

Run `true-bdd build code --fix` to review proposed fixes. Every fix is applied only after a person answers apply at its fix menu. `us refine`, `us apply`, `scen check` and `build steps` use the same menu. The engine offers no way to approve fixes in advance.

A clarifying question at end of stdin prints its question and suggested answers and stops with an error saying a person must answer. End of stdin at a fix menu stops with an error saying a person must choose whether to apply the fix. These stops exit non-zero without a COMPLETE banner. Answers are read from stdin, including pipes used by tests and the drill, with no terminal check. A final non-empty answer without a newline is accepted; a blank line keeps its interactive meaning.
