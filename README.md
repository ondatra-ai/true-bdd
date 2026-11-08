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
implement the pipeline `us refine`, `us apply`, `build steps`, `build tests`, `build code`. `build steps` writes and checks the Steps package, `build tests` renders the Scenario tests and their main file, and `build code` checks Red without the target when there is no code, then writes code until the tests pass.

## Status

| Subcommand | State |
|---|---|
| `us refine <id>` | **Working** — the pipeline's first step: walks a hand-written Story against `us-refine`, saving it in place when all Checklist items pass. Every shipped Checklist item has an `F:`; `--fix` proposes added or rewritten Acceptance Criteria, or a wider or new concept, applied only after the Product Owner answers apply at the menu. Stories still need 3 to 7 criteria. Each real failure mode needs its own criterion; a Story with none passes with a context line explaining why. On a host declaring `concepts:` every step must link to exactly one concept of its block (see concepts below). A custom checklist missing an `F:` still refuses `--fix`. |
| `us apply <id>` | **Working** — walks every AC in a refined story, validates against `us-apply`, and merges scenarios into the central `docs/scenarios.yaml` registry. Its merge Checklist item reads the architecture document, refused at startup when missing, and its fix is told to give a new Scenario the one service the architecture declares or, of several, the service of the Scenarios whose test file it reuses, asking which when neither settles it. The Registry is written only when every Scenario claiming one of the Story's criteria is keyed by its lineage id and names a service the architecture declares; otherwise the run exits 1, naming each Scenario and its `service`, and leaves the Registry unchanged. |
| `build steps` | **Working** — groups unbound steps into concept Items and rejudges their service bindings. For concept Items the checklist asks `pattern-bindings`, `step-coverage`, then `step-verification`. When `testing.testkit` is declared, a suite Item per distinct Steps package follows them, even with no concept Items; only `scenario-run` is asked about it, with a code Q and a code F through the fix menu. Unasked Cells have no result row and do not count as passes. The pattern item's F proposes patterns and rewrites skeletons; body Fs write only the selected definitions and shared state. Every apply rechecks its own Q before restarting the checklist, and a fix walk is followed by a walk of all Items. Bare runs report pattern and suite Scenario run findings without changing definitions. `--fix` writes only the Steps package, before any generated test or main file exists, and checks whole-suite Step coverage at convergence. It never lists or runs tests. Success prints `BUILD STEPS COMPLETE`. |
| `build tests` | **Working** — requires the suite's Steps package first, naming `build steps --fix` when it is missing. Deterministically renders one test per Scenario and `main_test.go`, holding `TestMain`, the suite and `New(t, id)`. Marked files refresh; unmarked main files are preserved. Without `--fix`, stale files refuse startup; `--fix` writes them. It checks that each Scenario has exactly one generated test in its Registry path, no test names a missing Scenario, and every fixture tree is named by a Scenario. Registry, path, renderer and step-shape refusals precede rendering. No AI turn, fix menu or test runner. |
| `build code` | **Working** — for Go suites with a Steps package, first lists every Scenario test and runs the suite when a service has no source under its path or writable paths. Each Scenario of a service without code must fail at an executed definition; a green Scenario stops distinctly, naming it, with no fix path. Compilation, bring-up, skipped or missing results fail that Red check. Existing source skips it; Jest and Playwright retain their replay behavior. It then runs the architecture's declared replay command and walks the whole failing set as one Item. An engine-owned code Q verdict needs no validation AI turn. With `--fix`, one plan and one writer repair the set; both receive the failing Scenarios and what the Specification says their steps mean (the concepts the steps' definitions record, with their words and bindings), and the writer also service paths and exact `writable` paths, and codex or Claude whole-suite and selected replay forms. After every completed apply the engine reruns the remaining set in one start; passing members leave and missing results stay failing. A successful writer response missing its confirmation warns once with the result path and follows the same rerun as `applied: false`, keeping files and consuming the same allowance. At most `max_apply_attempts` applies are allowed per Checklist item. Every apply reruns its own Q; a failure after the last allowed apply stops with `FIX_LOOP_NOT_CONVERGING` before another F, while a passing Q restarts the checklist. Started codex writers that exit non-zero or answer empty, and Claude writers that exit non-zero, return an error result or answer empty, follow the same recovery without a stall watch. Start failures and engine refusals before spawning remain fatal, as do other roles' failures. The writer ends its final message with the confirmation block; the engine saves the block at the heading's result path, which the writer never writes. Fresh discovery on each outer walk catches regressions. Tests and the Registry remain unchanged. |
| `scen check [id...]` | **Working** — walks registry scenarios against the `scen-check` checklist, one Cell per (Scenario, Checklist item). Ordinary questions read that Scenario; shared binding and word judgements answer every Scenario reaching the same concept. Exits non-zero when any scenario fails, so it can gate a change. With no argument every scenario is walked; ids narrow the walk to those scenarios, in ascending id order. |

Every command accepts `--fix` for an interactive loop in which
Claude proposes edits for each failed check and the user applies,
refines, or exits. `--fix` is refused at startup when a prompt the walk
would visit carries no `F:` fix template; every prompt of the shipped
`scen-check` checklist carries one. A `scen check` fix for a failed
closure check defines the missing term in the product document, or maps
it in the architecture document, never in the Scenario; a fix for a
failed model question rewrites the Scenario's own registry entry; a
concept or binding judgement's fix offers the responsible person a
rewritten step, a wider or new concept, or a binding (see concepts below).
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
env -u CLAUDECODE ./bin/true-bdd build steps --fix
./bin/true-bdd build tests --fix
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

`true-bdd/` holds the initialized files, including customizable schemas:

- `true-bdd.yaml` — the model tiers (below), filesystem paths under
  `paths:` (`stories_dir`, `checklists_dir`, `tmp_dir`,
  `tmp_glob`), per-command prompt-template paths, and a `documents:`
  map naming the files a check may cite (`product`, `architecture_yaml`,
  `scenarios_yaml` — the scenario registry). Each check in a checklist
  lists the document keys it needs under `docs:`, and the engine points
  the prompt at those files.
- `checklists/` — one checklist per checklist command, named by hyphenating the
  command path (`us refine` → `us-refine.yaml`, `build steps` →
  `build-steps.yaml`, …). Each Checklist item has `Q: {prompt: …}` for a
  model or `Q: {code: …}` for engine code, and an optional `F:` (below).
- `*-schema.yaml` — Yamale schemas pinning the shape of the spec
  documents (`product.yaml`, the scenario registry, `architecture.yaml`);
  the host project validates them in its lint step, outside the engine
  itself. The binary ships these schemas through `init`, preserving any existing copies.

The host default configuration omits `scripts:`. This repository keeps that section
in its own `true-bdd/true-bdd.yaml` for its tooling. The `Init defaults drift` gate
holds the init copies to the repository checklists and prompt templates byte for
byte and to the configuration values after removing `scripts`.

`engine.max_prompt_bytes` is the positive integer limit for every Claude and Codex turn (default `150000`). Before dispatch, the engine counts UTF-8 system and user bytes, including Codex assembly headings, logs `prompt_bytes` and `prompt_limit_bytes`, and refuses an oversized prompt with its turn, size and limit. It never truncates evidence. Body proposals and applications embed the semantic core and read Scenarios and fixture inputs by path; binding judgements read the complete product document by path. The step-verification Checklist item remains tool-less, with complete evidence selected by reference and no fixture files.

`engine.scen_check_concurrency` bounds the number of Scenarios validating at once in
`scen check`. `init` writes `1`, the value an older configuration omitting it gets.
Set a positive integer such as `10` to overlap validation turns. Each Item walks
its checklist in written order. Under `--fix`, validation pauses at its first failed
Q for the F and fix menu; menus run one at a time in ascending Scenario id order.
The binding judgement waits until every Scenario has answered the preceding
checklist items, including its link. Each apply invalidates shared verdicts,
rechecks that same Q, and retries its F within `max_apply_attempts` if necessary.
Once the Q passes the Item's checklist starts again; a walk applying any fix is
followed by a walk of all Items.

### Checklist items

Each `validation_prompts` entry has a `Q` mapping with exactly one option:
`prompt`, the question a model answers, or `code`, the name of an engine Built-in
check answered without an AI turn (ADR-0042, amending ADR-0015). Parameters stay
under `with:`; `rationale`, `docs`, optional `F:`, `fix_model` and `apply_model`
keep their meanings. The `validation_prompts` key keeps its name.

```yaml
validation_prompts:
  - Q: {code: given-when-then}
    rationale: "A Scenario needs all three step kinds."
    F: "Supply the missing step."
  - Q:
      prompt: "Does this Scenario state one observable outcome?"
    F: "Rewrite the outcome."
```

Checklist items run in their written order. Without `--fix` the walk continues to
the end and reports every failure, including prompt Qs after a failed code Q.
A code Q returns pass or fail and context lines, with its F using the same menu.
Legacy `D:`, scalar `Q:`, both options, neither option and extra Q keys refuse at
startup, naming `Q: {prompt: …}` and `Q: {code: …}` as replacements. The engine
also refuses an unknown check, an unread `with:` parameter, `model:` on a code Q,
and `docs:` omitting a document its check reads. `scen check` holds six checks:
`given-when-then`, `declared-terms`, `term-mappings`, `concept-bindings`,
`public-interface` and `step-definitions`; its shipped checklist walks the last five.

Any F may answer that it cannot fix, at proposal, after clarification or at apply:
`cannot_fix: {reason: …, correction: …}`. Each F fixes only what its Q found,
receiving the question, answer, context lines and item's F text. A problem its Q
did not find is neither its work nor a reason for cannot-fix. A person who has
not decided is a legitimate cannot-fix. The run stops distinctly, naming the
Checklist item's section and index, reason and what a person must correct.
The existing `interface_cannot_serve` and `impossible` answer shapes remain
instances with their established log records, edits and build-code rerun.
No F is asked to name an Earlier step: that term means a previous command of
the pipeline `us refine`, `us apply`, `build steps`, `build tests`, `build code`.

### Parking a prompt

`skip: true` parks a Checklist item: it costs no AI turn, takes no report row
and no position in artifact names. Its Q must still have the valid new form;
command-specific check and tier resolution are left out with the parked item. `skip: false`, or no `skip:`, walks the prompt as usual:

```yaml
validation_prompts:
  - Q: {prompt: "…"}
    skip: true    # parked: not walked, no AI turn
  - Q: {code: given-when-then}
    skip: false   # walked, as with no skip: at all
```

`skip:` takes only a YAML boolean. Any other value is refused at
startup, naming the prompt, since the engine would have to guess what it
meant: the quoted `"false"`, which once parked the prompt it was meant
to keep, and `yes`, which YAML reads as a string, among them. A
checklist whose every prompt is parked is refused like one with no
prompts, rather than passing every item unchecked.

### Concepts and their bindings

The product document declares `concepts:`, what each Given, When and Then step may state, and the architecture binds each concept to its service's Public interface. They replaced the step templates of the retired Catalogue and their Phrase mappings (ADR-0034, ADR-0035): every command refuses at startup a product document still declaring the `catalogue` key, or an architecture document still declaring `architecture.phrases`, even empty or null, naming `concepts:` or `architecture.concepts` as its replacement.

```yaml
concepts:
  given:
    - concept: stored number
      means: The number file holds the starting number before the command runs.
      terms: [number]
  when:
    - concept: halve a number
      means: The halve command is called once for the stored number.
      terms: [number]
  then:
    - concept: stored number
      means: The number file is read and its number compared with the expected number.
      terms: [number]
```

Write one concept per Public interface item and kind of use, per block (ADR-0036). The item is what the use is about; a binding can reach it through several declared references. What the step definition does, as the step's wording fixes it, decides its concept, never the target an argument names. A Given states a state. A concept's `means` is closed: name the item, kind of use and every admitted variation in prose, without slots or brackets, using only declared `terms` and plain English. A new variation widens the existing concept by rewriting its `means`; only a new item or kind of use calls for a new concept. Names are unique within their block and may repeat across blocks. An entry holds `concept`, `means` and `terms` and nothing else: every `us`, `scen` and `build` command refuses at startup a concept carrying another key (a `variations:` list, say), naming its block, concept and key, and a fix whose working copy adds one is not committed. That fix did not take: every working copy it changed is reset, no document is written, and the refusal joins the findings of that Prompt's next failed answer, so the next proposal can repair it within `max_apply_attempts`.

The architecture binds each concept by name and block:

```yaml
architecture:
  concepts:
    - concept: halve a number
      block: when
      service: calculator
      references: [{command: number halve}]
      does: Call number halve once and keep its result.
```

A binding's `does` says what the definition does: a Given puts state in place, a When calls, a Then observes and compares. Each reference has exactly one selector: `command` is the CLI command path (the exact `commands` key), `file`, `log_record` and `outgoing_call` name keys under the CLI document's `extensions.files`, `extensions.log_records` and `extensions.outgoing_calls`; `operation_id` names an HTTP `operationId`; `element` names a UI element by page or region, role and accessible `name`. An `intent` sentence replaces both `does` and `references`, accepted only for UI services until #246. The architecture stores no Go identifier or link to a step definition.

`Q: {code: declared-terms}` and `Q: {code: term-mappings}` check every concept's terms against the glossary and Word mapping (see the term documents below). `Q: {code: concept-bindings}` (`docs: [product, architecture_yaml]`) checks that each binding names a declared concept and service, each concept (name and block) and service pair has at most one binding, and every reference belongs to that service's Public interface. These are document checks over all declarations, including concepts no current step uses and Scenarios no Story claims. Every finding names the concept. Fixes go through the menu to the product or architecture document, with an added Public interface declaration when needed, never to a Scenario: a `concept-bindings` fix first asks the System Architect what a step of the concept does, proposes the binding at the fix menu, and after apply writes the architecture and any Public interface document the answers name. No AI turn answers these checks. A host declaring no concepts passes them, and `Q: {code: step-definitions}`, as "not checked: the product document declares no concepts".

For concept hosts, `us refine` and `scen check` each run one link `Q:` Prompt per Item. Every step is scored from 0 to 10 against every concept of its own block. Exactly one must reach the threshold; two reaching it fail as ambiguous and name both, and a step of a block that declares no concept has none to reach it and fails. The answer schema requires every score, the reason for the highest and the other-block concept or none. Missing, malformed or incomplete answers fail the Cell. Every score, reason and verdict is recorded in context. `means` is closed: an unstated variation scores at most 8; a wrong-block step scores at most 4. `judge:` clauses pass unlinked; `llm:` steps link by intent.

After the link, a separate `Q:` Prompt (`with: {concepts: words}`) judges the `means` of every linked concept: once per Story in `us refine`, and in `scen check` once per walk for each concept a Scenario's link reached, claimed or not, in a turn reading the product document and that concept alone, every Scenario linking it answering from that verdict with its own context lines. Each concept needs a verdict naming every domain word its meaning relies on that is neither in that concept's `terms` nor readable as plain English in context. Ordinary English passes. Every verdict is saved in context; a missing, malformed or incomplete answer fails the Cell. Each Scenario's word cell follows its own link, and a failure exits `scen check` with code 1.

The word fix belongs to the Product Owner and changes only the product document after menu apply: add a glossary entry and the word to the concept's `terms`, or reword its `means` in product words. `us refine` never waits for a Word mapping. On the next `scen check` walk, `Q: {code: term-mappings}` fails a newly declared unmapped term; its separate System Architect menu fix adds `architecture.terms`. Every walk judges the words afresh after shared fixes.

The shipped checklists write code Qs first where needed, then the link before the other model questions. The engine preserves written order and uses each selected file position for the Cell's artifacts; a link fix can add a concept before refinement sketches it.

`build tests` renders free-text steps, matched against nothing. `build steps`, with or without `--fix`, groups only distinct texts no definition binds, separately per suite and block. It excludes `judge:` and `llm:` steps. Each answer supplies the best concept and score and the runner-up and score (with one concept, the runner-up is `null` with score zero); the engine applies the link threshold and ambiguity rule. A step with no concept, or two at the threshold, stops before any skeleton and names the step and matching concepts. Missing, repeated or unknown indexes, invalid rankings and an output-capped answer crash the run.

`config.grouping_batch_texts` in `build-steps.yaml` defaults to 800 distinct texts. Above it, grouping uses the fewest batches whose sizes differ by at most one (five texts at three gives batches of three and two). This default was measured for `claude-opus-5` and the best-and-runner-up answer; remeasure when either changes. The grouping `Q:` uses `with: {concepts: group, threshold: 9}`, with the same configurable threshold as linking. Each resulting Item is a concept and suite. Before any skeleton, all grouped concepts need a service binding, judged as `scen check` judges it: `scen check`'s binding Prompt, one turn per binding over the steps of every Item that reaches it. Without `--fix`, `build steps` then lists every concept Item still to write and exits 1, or reports `BUILD STEPS COMPLETE` when none is left, Step coverage passes, and every harness suite's `scenario-run` Q passes.

The checklist first asks `Q: {code: pattern-bindings}` about each concept Item's registered patterns against every deterministic suite step, in every block. A pattern must compile, name one capture per group, bind only its Item's steps and bind some step; every owned step binds exactly one pattern. A fresh Item with no pattern fails. A resumed registration has no saved capture names, so its capture count comes from its regex; a new proposal still has its named captures checked. Invalid registered regexes remain startup refusals before any AI turn.

The pattern item's `F:` holds the former `concept-patterns` proposal prompt verbatim. There is no separate proposal Q. With `--fix`, its failed code Q sends its findings and the current patterns through the existing proposal protocol: a Go regular expression, named captures in order and the clause of the binding's `does` it serves. The protocol reads the concept, its terms' glossary entries, binding, whole Public interface, existing definitions and owned steps. It makes no pass/fail decision. After the person's apply choice, the engine rewrites the skeletons and removes dropped patterns with their registrations. A skeleton records its concept and starts with a body failing `step not written yet`. The same code Q judges it next; a pass restarts the checklist. `config.max_apply_attempts` bounds applies, and a pattern still failing stops with `FIX_LOOP_NOT_CONVERGING`, naming the Checklist item and concept. Exiting the menu writes no skeleton. Refinement carries the proposed candidate and the person's feedback in the existing refusal inputs.

`step-coverage` then sends pending bodies through its own F. Writers change only selected bodies, imports, shared State and helpers inside protected test paths. The engine checks compilation, one binding per owned step, concept records and written bodies after apply; `step-verification` judges their meaning. It rechecks its own Q after an apply and restarts the checklist once that Q passes. A walk applying a fix walks every Item again, followed by whole-suite Step coverage at convergence. Each concept Item's skeletons are written by its pattern F before its body F, with no generated test or main file required. A writer finding that the declared interface cannot serve its concept stops with the missing setup or observation. Test listing and Red without the target remain in `build code`.

With `testing.testkit`, the command appends one suite Item per distinct Registry Steps package after the concept Items, even when no concept Item is pending. The final `scenario-run` Checklist item asks only these suite Items whether State embeds `*testkit.ScenarioRun`. Its Q reads without writing; its F names the suite and State file at the menu and embeds the field in code, preserving definition-owned declarations without a model turn. If a definition-owned field already has the name `ScenarioRun`, the Q fails naming the file and field, and the F answers cannot fix before the menu, with no file changed and no AI turn: rename or remove that field, then run `build steps --fix` again. A foreign `testkit` import without that field uses a free alias for the harness. Bare runs report a missing or conflicting field by suite and file. Unasked Cells produce neither a pass nor a result row.

That context holds the concept, its binding, its terms' glossary entries and Word mappings, the Public interface declaration and document, each definition's pattern, captures and bound steps. Body turns receive the Registry path, the keys of every using Scenario, and fixture directory names under a shared fixture_root. A Scenario's fixtures are the directories under its suite's `fixtures/` (beside `steps/`) that a quoted value of its steps names, such as `tests/calc/fixtures/offline/` for `the calculator runs on the "offline" toolchain`; their inputs are every file there except the recordings under `cassettes/`. Body turns read needed inputs by path. The tool-less `step-verification` Q receives the complete semantic core and architecture `step_arguments` section when declared, source declarations and their transitive local dependencies (State, helpers and registrations), Public interface entries named by the binding, and the runtime `go doc` declarations the source references, with required type and field meanings. Bound steps appear as distinct forms with up to three complete original examples and their capture tuples; literal alternatives such as a negation stay visible. It receives no fixture file. A host-specific requirement, such as an environment variable a stand-in needs, belongs in the host architecture binding.

The `step-verification` Checklist item asks one question: does the generated step definition match what its steps mean under its Concept and binding? Its answer remains `pass` with no findings or `fail` with findings naming a definition, category and discrepancy. The categories are `pattern_captures`, `pattern_scope`, `concept_binding`, `concept_meaning` and `computes_assertion`. Judge the whole regex language, including combinations absent from the examples. Its one F fixes only the reported discrepancies: a pattern finding prepares the concept's proposal again, with the findings and current patterns as its refusal; after the person's apply choice the engine rewrites changed skeletons and the writer supplies their bodies for the new captures. Other findings select only their named bodies. A missing planned skeleton before apply is expected. Cannot-fix at proposal, clarification or apply names the reason and what a person must correct. After apply the same Q runs again, then the checklist from its start, within the per-Checklist-item allowance.

The link Prompt's checklist setting is `with: {concepts: link, threshold: 9}`. Omit `threshold` for 9; integers 1 through 10 are accepted. `with: {concepts: bindings}` selects the dependent binding judgement in `scen check`; `with: {concepts: sketch}` makes refinement Prompt 12 sketch against concept meanings and the service's interface kind. These parameters select structured model answers for `Q:` Prompts; a host declaring no concepts skips these Prompts.

Refinement reads only product concepts for the link. Its fix offers the Product Owner a rewritten Story step, a wider existing concept for a new variation of the same item and use, or a new concept for a new item or use. Concept edits go to the product document; rewrites go to the Story. Prompt 12 uses the concept's `means` and the service's interface kind, so a Story can pass before the System Architect writes a binding.

`scen check` judges every Scenario afresh, claimed or unclaimed. Every concept its steps link to needs a binding for that Scenario's service; unused unbound concepts are not that Scenario's findings. After a successful link, a further `Q:` Prompt judges those bindings against their instructions, references and complete Public interface: they must be unambiguous enough for an LLM to write a step. Each binding (concept, block and service) is judged once per walk, after every Scenario's link, for every variation its concept's `means` admits: its turn reads the product document, the concept, the binding, the service's complete Public interface and every distinct step of the walk's passing links that reached the concept for that service, and every Scenario whose link reached it answers from that verdict with its own context lines. `build steps` judges each binding the same way before any skeleton, in one turn per binding over the steps of every Item that reaches it. An `llm:` step requires an intent binding. Any failure fails the Scenario and exits 1. Fixes offer the Product Owner a concept repair and the System Architect a binding repair, applied only after the menu's apply decision. An applied shared change causes all Scenarios to be judged again, within the configured validation concurrency; the binding judgements wait for every Scenario's link.

`Q: {code: step-definitions}` (`docs: [product]`) holds every step of the Scenario, except `judge:` and `llm:` ones, to its suite's step definitions: no step binds two, and the one a step binds records, in its `//truebdd:concept <block> "<concept>"` comment, a concept the product document declares under that step's block. A step two definitions bind, a definition recording no concept, and one recording another block's concept or an undeclared one each fail the Scenario, naming the step and the definitions by pattern, file and line. A step no definition binds yet is a gap `build steps` writes, reported but not failed. A pattern that does not fold to a constant stops the walk with `msg="Check could not answer"`. TrueBDD's own product document declares concepts and its architecture binds them (#259): every step definition of its suite records the concept of its steps.

`build code` hands both repair turns, for every failing generated Go test it matches to a Registry Scenario, the concepts that Scenario's steps link to. It runs no link turn: a step's concept is the one the `//truebdd:concept` record of the one step definition it binds names, among the concepts the product declares under the step's block. A `judge:` or `llm:` step, a step binding no definition or two, and a definition recording no declared concept of its block link nothing, and an unreadable suite is warned about and links nothing. Under "What the spec says these steps mean" each linked concept appears once across the failing set, with its block, its closed `means`, its `terms`, and its binding for the Scenario's service: the references and `does`, or the `intent`. The words of those terms follow, each with its glossary entries and Word mappings. A host declaring no concepts gets no such section. `build steps --fix` reads the product's concepts and glossary, the Word mapping and the bindings before its grouping turns, so a malformed term list stops it before any turn.

### The term documents

`Q: {code: declared-terms}` (`docs: [product]`) holds the words every declared
concept relies on to the product document's `glossary:`, each entry a
`term` and what it `means`. Every term a concept declares must be
defined exactly once, spelled exactly as the concept spells it: a term
the glossary lacks fails the Scenario naming the concept and the term,
and a term defined twice fails it naming the count.

`Q: {code: term-mappings}` (`docs: [product, architecture_yaml]`) holds the same
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

Keep a glossary entry’s `means` in one paragraph of at most three sentences, each at most 25 words. Every product reader and command startup refuses a violation with the term, sentence count, longest sentence’s word count, paragraph count and rule. A sentence ends at a run of `.`, `!` or `?` followed by Unicode whitespace or end of text; a remaining nonempty fragment counts. Words are nonempty fields separated by Unicode whitespace, so paths, decimals and hyphenated words stay whole. Blank lines separate paragraphs; wrapping lines does not. Concepts’ `means` are exempt.

The Word mapping’s `observed_by` is optional: keep it only for setup or observation instructions the Concept binding does not already state.

Every term must be mapped exactly once: an unmapped term fails the
Scenario naming the term, and a term mapped twice fails it naming the
count.

A host adopts concepts by declaring them: with no `concepts:`, every
Scenario passes these checks, `Q: {code: concept-bindings}` and
`Q: {code: step-definitions}` as "not checked: the product document declares no
concepts". Under `--fix` a failed check's `F:` drives the usual
clarification loop, and the fix is written to a scratch copy of the
product or architecture document in the run directory, which the engine
puts in place of the document once it parses: the Scenario, the Story
and the step definitions are never edited. Each fix to `concept-bindings`
or `public-interface` works on run working copies of the architecture and
of every cli or http document it names; a changed document must still
parse in the form of its kind, and is committed before the architecture.

What no check can settle, a model answers. The shipped `scen-check`
checklist follows its five Closure checks with its eight quality
questions on the description and the steps, then the link, binding and
word Prompts above. Under `--fix` a quality question's fix rewrites the
Scenario's own entry in a scratch copy of the registry, which the engine
puts in place once it loads with the same Scenarios; the walk then
restarts on the Scenario as it now reads, and `build tests --fix`
re-renders its test.

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
flags are set there. `TMPPREFIX` is `{host}/tmp/codex-tmp/zsh`, keeping zsh here-documents in granted scratch. The engine creates private `codex-shell-home`, `codex-tmp` and `codex-go-build` directories before the turn. Subprocess network is off and `GOPROXY=off`.
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
  declares the services a Scenario `us apply` merges may name,
  and maps each product word onto the implementation under `terms:`.
- `docs/product/product.yaml` — the product document, including its
  `roles:`, the BDD `vocabulary:` (forbidden qualifiers and forbidden action verbs) the `us refine` and `scen check` Prompts cite, its `concepts:`, and the `glossary:` defining their declared terms. These are roles (responsibilities), not personas (invented
  individuals) — one person commonly holds two of them.
- `docs/product/stories/*.yaml` — the stories, written by hand out of a
  discussion of a Feature, that `us refine` / `us apply` read, one
  `<id>-<slug>.yaml` each. No command creates one. A story id is
  a slug (`scen-check`, `96.2`), and a story may carry a `feature:` label
  that `us refine` keeps through every rewrite. Each `acceptance_criteria[].steps`
  entry admits only `given`, `when` and `then` keys. `and` and `but` are modifiers
  inside those statement lists, never step keys. Readers refuse any other key,
  naming it and the Acceptance Criterion. A writer's Story body that does not
  parse is a fix not taken: the Story stays unchanged and the refusal reaches
  the next F as context.
- `docs/scenarios.yaml` — the scenario registry `us apply` merges into,
  `build tests` renders, and the test suites RUN: each scenario is a test,
  its steps bound to code by the suite that owns it. A scenario is keyed
  by its lineage, `<story-id>-<ac-id>`, and the author writes each
  acceptance criterion's id as `<NNN>-<slug>` (ADR 0014), so the key
  `scen-check-001-unknown-id-refused` survives reordering the story;
  `us apply` refuses a criterion id of any other shape.

Prompt templates live in [`templates/`](templates/) (Go `text/template`
with sprig).

### Step argument conventions

A Host project's architecture document may declare `step_arguments` at its top level, beside `architecture`. Each entry names an argument family and states how it is read, where it applies and its escaping:

```yaml
step_arguments:
  - name: regular expression
    read: Compile the complete undelimited tail, including quotes, as it stands.
    applies: stdout and stderr comparisons
    escaping: RE2 escaping only; no token expansion
architecture:
  # services, testing, terms and concepts as usual
```

This is the authoritative argument contract. Concept bindings and Word mappings refer to it. Every engine turn that reads steps receives it: a read-enabled turn already receiving the architecture names the section at that path; other turns embed its whole text, including the tool-less verification of definitions. Evidence is never summarized or truncated and stays inside `engine.max_prompt_bytes`; an oversized prompt refuses before an AI CLI starts. Ordinary regexes and token-expanding every-file patterns keep separate scopes in TrueBDD's own [argument families](docs/architecture/architecture.yaml).

Checklist turns carry the evidence through their Model's optional string field `StepArguments`, checked against the scheme before rendering. Construction follows the selected Docs after the sketch projection; other roles use `prompt_data`. Without the section, the field is omitted from the Model validation view, templates render no extra bytes, and existing prompt requests stay byte-identical. Build commands read the architecture selected by `--architecture`, when supplied. Block-style section text is preserved; flow-style documents and aliases render their complete decoded values. This field does not change argument execution.

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
    testkit:
      repo_layer: [true-bdd, templates]
      fixture_step: 'the "([^"]+)" project tree'
    commands:
      record:
        command: "go test -tags bdd -json -count=1 ./tests/true-bdd/ -mode=services:record,tests:record"
        exclude: ["tmp/*cache*/**", "tmp/go-*/**", "tmp/*-tmp/**"]
      replay: "go test -tags bdd -json -count=1 ./tests/true-bdd/ -mode=services:replay,tests:replay"
      live: "go test -tags bdd -json -count=1 ./tests/true-bdd/ -mode=services:live,tests:live"
  services:
    - name: true-bdd
      path: services/true-bdd
      language: go
```

`testing.commands.record` accepts a command string (no shared exclusions) or a mapping with `command` and optional `exclude`. Each exclusion is a path pattern relative to the run's Host project root, using the same glob syntax as `recording.unread` (`**` crosses directories). It has the same unread effect for every dependency, in addition to that dependency's own rules: neither run nor per-call recording snapshots traverse or read matching paths. `build tests` passes the list in the tests' main file to `pkg/testkit`, which adds it to every dependency's `unread`. Replay and live commands remain strings and keep their behavior. Other record forms, unknown mapping keys and non-string exclusions are refused distinctly.

For Go suites, the optional `testing.testkit` mapping enables the testkit harness. `repo_layer` lists repository-relative trees copied before a fixture input, and `fixture_step` is a regular expression with one capture naming its fixture. Every service dependency with `protocol: cli` and `scope: third_party` gets a stand-in; `ai_clis` is retired and refused. Each dependency may declare `recording.placeholders` (a token, matching regular expression and directory used to resolve the current value), `recording.unrecorded` (paths omitted only from cassettes) and `recording.unread` (paths neither run nor cassette snapshots may traverse or read), both relative path patterns with `**` crossing directories, and `recording.read_only_when` (environment keys and values marking a read-only request). The tests' main file passes these values to `pkg/proxy/cli`. `build steps` checks State's embedded `*testkit.ScenarioRun` through the `scenario-run` Checklist item on one suite Item per distinct Steps package. Its code F, applied at the `--fix` menu without a model turn, ensures the field while preserving every definition-owned field, method and declaration, even when no new skeleton is needed. A conflicting definition-owned `ScenarioRun` field makes the Q fail and the F answer cannot fix before the menu, without writes or an AI turn, naming its file and asking a person to rename or remove it and run `build steps --fix` again. A bare run reports a missing or conflicting field by suite and file without changing definitions. `build tests --fix` renders a bdd-tagged `main_test.go` holding `testkit.NewSuite`, `TestMain` and `New(t, id)` beside the Scenario tests. Without `testkit`, the main file uses plain bddgo. Marked main files refresh and unmarked ones are preserved. The host's Go module graph must provide `github.com/ondatra-ai/true-bdd/pkg`.

Each service may declare `public_interface`. It is optional in the schema so an older architecture loads, but `scen check`'s `Q: {code: public-interface}` (`docs: [architecture_yaml]`) fails every Scenario whose service has no valid declaration, naming the service without an AI turn. This check runs even without concepts or a Story. Its `F:` asks the System Architect what the service offers, then writes the architecture document, and an existing document it names where the answers say that document lacks what the service offers, never the Scenario.

```yaml
public_interface:
  kind: cli
  document: docs/greeter-cli.yaml
```

`document` is relative to the Host project root. CLI documents use this supported OpenCLI-like draft shape (YAML or JSON): `opencli` and `info: {title, version}` identify the document; `commands` maps command names to `parameters` (with `name` and optional `in: argument|flag|option`, `schema: {type: ...}`, `description` and `required`) and `responses` (exit-code keys mapped to objects with optional `description` and `content`). This follows the [OpenCLI reference](https://github.com/nrranjithnr/open-cli-specification/blob/main/opencli.spec.json); the referenced project currently advertises `opencli: "1.0.0"`. Every command declares responses for its exit codes. Optional `extensions` holds maps keyed by stable names a Concept binding can reference:

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

TrueBDD declares its own Public interface the same way: the `true-bdd` service names `docs/architecture/true-bdd-cli.yaml` (`kind: cli`), and `architecture.concepts` binds every concept of its product document to that document. The document lists every command with its arguments, options and exit codes, the hidden `claude-guard` hook, and, under `extensions`, the files the engine reads and writes in a Host project, the log records its Scenarios observe and its outgoing calls to `claude`, `codex`, the declared suite command and the Go toolchain. A statement the architecture already holds keeps that home, and the document points to it: the log vocabulary under `architecture.terms`, the IO contract under `patterns.owned_io`, the AI CLIs' invocation contracts under the service's `dependencies`, and the templates' inputs under `prompt_data`. The drill keeps the document in every cut because the declaration names it.

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
Lint all four modules with `go run ./scripts/cmd/alint`, using the root `.golangci.yaml`.

## Release rehearsal (repository tooling)

The repository has four Go modules: the root (tests and scripts), `pkg/` (shared libraries and the test library), `services/true-bdd/` (the engine), and `utilities/` (reusable stand-in binaries). The `pkg/proxy/cli` SDK builds `utilities/cliproxy` from the host library's commit, or from the sibling module through a temporary workspace for a local library copy; a host declares no utility dependency. Local development uses the root module's `replace` for `pkg/`; release engine builds use `GOWORK=off` and no replacements. Only the engine is released: a release is the engine module's tag `services/true-bdd/vX.Y.Z`, the only form under which Go resolves `go install …/services/true-bdd@vX.Y.Z`. No `pkg/` or root tag is pushed; a Go host takes the test library at the release's commit.

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

## Checklist Models

The service declares four `models` in the architecture: `us_refine_checklist`, `us_apply_checklist`, `scen_check_checklist` and `build_steps_checklist`. Each names its system/user templates and a fields-and-types-only JSON Schema written in YAML. Construction rules live in Scenarios naming the Model. Before rendering, the engine validates a YAML view of the actual call inputs and Subject in process, refusing a mismatch with the Model and field; rendering still uses the original typed data.

`init` ships `true-bdd/models/<model>.schema.yaml` and preserves existing files. `templates.schemes.<model>` in `true-bdd.yaml` overrides that default path, resolved from the host working directory like template paths; runtime never substitutes an embedded scheme. Before its walk, `scen check` checks declared scheme/template files and requires every declared Model to be named in a Scenario step of its service. The build-code checklist reads no data and declares no Model.

The repository report and checklist coverage tools run as `go run ./scripts/cmd/report-server` and `go run ./scripts/cmd/coverage`. The report reads retained sessions under `tmp/test_run/`; coverage reconstructs checklist branch evidence from those sessions.
