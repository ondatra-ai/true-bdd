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
implement the regeneration loop — `build tests` derives executable
tests from the registry, `build code` regenerates production code
until those tests pass.

## Status

| Subcommand | State |
|---|---|
| `us refine <id>` | **Working** — the pipeline's first step: iterates a hand-written story against the `us-refine` checklist; updates in place. When the product document declares a catalogue, every step of every acceptance criterion must match exactly one of its templates (`D: step-phrases`), checked before any model prompt; under `--fix` a step outside it gets a new catalogue entry in the product document, and the Story keeps its words. |
| `us apply <id>` | **Working** — walks every AC in a refined story, validates against `us-apply`, and merges scenarios into the central `docs/scenarios.yaml` registry. |
| `build tests` | **Working** — walks every scenario in the registry against the `build-tests` checklist and exits non-zero if any scenario is not executable: every one of its steps must bind to a step definition in the suite that owns it. When the product document declares a catalogue, every registry step, whether a Story claims its scenario or not, must first match exactly one of its templates, as `D: step-phrases` holds them, or the run is refused before it writes a file or spends an AI turn, naming each step outside it; a host with no catalogue walks as before. With `--fix`, failed cells drive a Claude-mediated authoring loop that writes the missing step definition; when the product document declares a catalogue, that loop is handed each unbound step's phrase, the glossary meaning of the words it relies on and their word mapping, so the definition observes what the mapping says. After each definition fix, it runs the bound Scenarios in every changed step package, including Scenarios that already bound every step, through `architecture.testing.commands.replay` with a Go test name filter. A selected Scenario must pass in its owning Go package; a same-named test elsewhere cannot satisfy it. A Scenario that becomes unbound remains a failure. Runtime failures feed the step-authoring loop; a definition still failing at `config.max_apply_attempts` ends non-zero, naming the Scenario and step. `BUILD TESTS COMPLETE` follows successful execution and generated-file verification. The registry itself is never modified. |
| `build code` | **Working** — runs the architecture's declared replay command and walks the whole failing set as one Item. An engine-owned `D:` verdict needs no validation AI turn. With `--fix`, one plan and one writer repair the set; the writer receives Specification context, service edit roots and crush, codex or Claude whole-suite and selected replay forms. After every completed apply the engine reruns the remaining set in one start; passing members leave and missing results stay failing. A successful writer response missing its confirmation warns once with the result path and follows the same rerun as `applied: false`, keeping files and consuming the same allowance. The terminal apply is rerun but still stops at the existing bound even if it passes. A started crush writer that stalls, exits non-zero or answers empty also warns once, keeps its files and reruns the set; the next plan sees that rerun's failures and evidence. Started codex writers that exit non-zero or answer empty, and Claude writers that exit non-zero, return an error result or answer empty, follow the same recovery without a stall watch. Start failures and engine refusals before spawning remain fatal, as do other roles' failures. The writer ends its final message with the confirmation block; the engine saves the block at the heading's result path, which the writer never writes. Fresh discovery on each outer walk catches regressions. Tests and the Registry remain unchanged. |
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
`build tests` and `scen check` also take
`--requirements <path>` and `build code` takes `--architecture <path>`
to override the configured spec locations (`documents.scenarios_yaml`
and `documents.architecture_yaml` in `true-bdd.yaml`).

## Install

Requires Go 1.25 and the `claude` CLI on `$PATH`.

```bash
mkdir -p ./bin && go build -o ./bin/true-bdd ./services/bdd-cli
```

## Usage

The tool spawns `claude` as a subprocess. If you invoke it from inside
a Claude Code session, unset `CLAUDECODE` first so the child has a
clean environment:

```bash
env -u CLAUDECODE ./bin/true-bdd us refine scen-check --fix
env -u CLAUDECODE ./bin/true-bdd us apply  scen-check --fix
env -u CLAUDECODE ./bin/true-bdd build tests --fix
env -u CLAUDECODE ./bin/true-bdd build code  --fix
env -u CLAUDECODE ./bin/true-bdd scen check scen-check-003-unknown-id-refused
```

`us refine` issues many sequential Claude calls and typically takes
~5 minutes end-to-end. Don't abort early.

## Configuration

The host project supplies a `true-bdd/` directory at its root:

- `true-bdd.yaml` — the model tiers (below), filesystem paths under
  `paths:` (`stories_dir`, `checklists_dir`, `tmp_dir`,
  `tmp_glob`, and the `test_write_globs` roots `build tests --fix` may
  author into, `./tests/**` and `./services/**` when it names none), per-command prompt-template paths, and a `documents:`
  map naming the files a check may cite (`product`, `architecture_yaml`,
  `scenarios_yaml` — the scenario registry). Each check in a checklist
  lists the document keys it needs under `docs:`, and the engine points
  the prompt at those files.
- `checklists/` — one checklist per command, named by hyphenating the
  command path (`us refine` → `us-refine.yaml`, `build tests` →
  `build-tests.yaml`, …). Each prompt is a `Q:` a model answers or a
  `D:` the engine answers itself (below).
- `*-schema.yaml` — Yamale schemas pinning the shape of the spec
  documents (`product.yaml`, the scenario registry, `architecture.yaml`);
  the host project validates them in its lint step, outside the engine
  itself.

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
`docs:` does not list a document its check reads. `scen check` holds five
checks, `given-when-then`, `step-phrases`, `declared-terms`,
`term-mappings` and `step-definitions` (below); the shipped `scen-check`
checklist walks the last four.

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

`build tests --fix` hands whoever writes a step definition the phrase each
unbound step matches, with the glossary meaning and the word mapping of
every term that phrase declares, and nothing else of the catalogue: not
this grammar. A host whose definitions must follow it states it as a word
its phrases declare. TrueBDD's own product document does: its glossary
defines `template`, its architecture document maps it, and every phrase
whose template holds a slot or brackets declares it.

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
        One request through the engine's AI router to claude, crush or
        codex, logged once as msg="Dispatching AI turn".
      observed_by: >-
        Count the "Dispatching AI turn" records in the run's JSON log file.
```

Every term must be mapped exactly once: an unmapped term fails the
Scenario naming the term, and a term mapped twice fails it naming the
count.

`D: step-definitions` (`docs: [product]`) holds the catalogue to the
suite's step definitions, the `steps/` package beside the Scenario's test
file, read without running it as `build tests` reads it. Over the steps
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
checklist follows the four closure checks with two `Q:` prompts that
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
    coder: "crush:zhipu-coding/glm-5.2"
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
`claude`, `crush`, and `codex`; each must be on `$PATH`.

Every command that runs AI turns (`us refine`, `us apply`, `scen check`,
`build tests`, `build code`) accepts repeated `--model <tier>=<cli>:<model>`
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
`build code --fix` or `build tests --fix`, records them in the run's
arguments and forwards their values unchanged through typed options.
It still refuses other extra arguments; the engine validates the values.

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

| | `claude` | `crush` | `codex` |
|---|---|---|---|
| System prompt | native flag | prepended to the prompt | prepended to the prompt |
| Write gate | `--allowedTools`; generated hook for the build-code writer | generated `PreToolUse` hook | named writer profile for build code; otherwise `-s read-only` / `workspace-write` |

`crush run` has no permission gate of its own, so the engine generates a
per-run config (pointed at via `CRUSH_GLOBAL_CONFIG`, leaving any host
`.crush.json` untouched) whose `PreToolUse` hook is this binary's hidden
`true-bdd crush-guard` subcommand — a host project needs no guard script
of its own. Hooks are additive, so a host config's own hooks run
alongside it and a denial from either blocks the tool.

Only `build code --fix`'s crush and Claude apply turns receive a test-command grant:
the declared replay without its machine-output flag, alone or followed by
the framework selector and a single-quoted nonempty selection. The guard
matches those strings exactly and requires the suite's resolved directory.
An argument containing a single quote, newline or NUL withholds the grant
with one warning. Its generated tools include `bash`, `job_output` and
`job_kill`, so a replay longer than a minute can finish in crush's background
and be read back. General `allow_bash` remains false. Tests execute the writer's code with the
machine's permissions; this restricts shell use, not that code's access.
The default crush writer has no router deadline and stops with the command.
Only a `build code --fix` crush apply watches its own
`options.data_directory/logs/crush.log`: `ModelProvider called` opens a
model response, the next record closes it, and `Hook completed` opens tool
execution. A response open for ten minutes stops the process group and
warns once with the turn label, minutes and log path. Tool execution is
never timed; without a model record nothing is timed. This deliberate
exception to unbounded AI turns is recorded in ADR-0020. No prompt,
checklist or template states the bound. Other Claude turns retain their 20-minute bound. Codex writers receive the same two command forms and poll their native process tool.

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

Two crush behaviours the engine works around, both verified against the
live CLI rather than assumed:

- crush **silently ignores an unknown model pinned in config**, falling
  back to global state. The model is therefore always passed as `-m`.
- crush **fails open** when a hook cannot be executed. The engine
  probes the guard before every crush turn and refuses to run if it does
  not deny, so enforcement can never disappear quietly.

Only `build code --fix`'s codex writer uses a generated `CODEX_HOME` and
named permission profile (ADR-0021), with high reasoning and a configured
1,050,000-token window. This configuration does not verify 1M capacity.
The home copies only sign-in credentials and is denied to writer commands;
user config, MCP, web search, history and AGENTS.md are suppressed. Reads
reach the host, OS runtime, Go toolchain and module cache excluding
`github.com/ondatra-ai` (including download archives) and the entire module-cache `cache/vcs` tree. All generated homes share a denied `tmp/codex-homes` subtree; authentication is removed after each turn, including failures. Writes reach service
edit roots and host `tmp/`; `HOME`, `TMPDIR`, `GOCACHE` and controlled Go
flags are set there. Subprocess network is off and `GOPROXY=off`.
The profile is calibrated on macOS with codex-cli 0.162.1 and has no weaker
fallback. Codex's `--json` events are saved beside its transcript and read
by drill audit as references, not proof that reads succeeded. The final
message still comes through `-o`. Started non-zero or empty codex writer
turns preserve edits and rerun the set with one warning; start failures
stay fatal. No codex stall watch is added. Other codex turns retain their
`workspace-write` or `read-only` sandbox and recorded requests.

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
  `build tests` walks, and the test suites RUN: each scenario is a test,
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
    prebuilt_engine_arg: "-engine-bin={engine_bin}" # optional; runner must consume it
    commands:
      record: "go test -tags bdd -json -count=1 ./tests/bdd-cli/ -mode=services:record,tests:record"
      replay: "go test -tags bdd -json -count=1 ./tests/bdd-cli/ -mode=services:replay,tests:replay"
      live: "go test -tags bdd -json -count=1 ./tests/bdd-cli/ -mode=services:live,tests:live"
  services:
    - name: bdd-cli
      path: services/bdd-cli
      language: go
```

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
- **One bounded exception: `prebuilt_engine_arg`.** This optional field is one argv element containing `{engine_bin}` exactly once. Only `build tests --fix` verification appends it, replacing that placeholder with the running binary's absolute, symlink-resolved path. Spaces and quotes in that path stay in the same argument. Every command that loads the architecture, including `build code`, refuses empty declarations or zero or repeated placeholders at startup, even when it never uses the argument. The replay entry point must consume the argument and skip building its service; this repository's harness does so with `-engine-bin`. Native Jest or Playwright runners need a wrapper, otherwise omit the field. `commands.*` stay literal, hosts without the field keep their existing verification, and `build code` runs its declared command unchanged.
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
go test ./...

# end-to-end BDD scenarios — real Claude calls, ~3–5 min per scenario
go test -tags bdd -timeout=180m ./tests/bdd-cli/...
```

The suite's contents come from `docs/scenarios.yaml`: every scenario the
architectural spec assigns to it gets one top-level Go test named from
its key in CamelCase (`scen-check-003-unknown-id-refused` gives `TestScenCheck003UnknownIdRefused`),
written into the file the scenario's `path:` names
by `true-bdd build tests --fix` and by nothing else. Its steps are bound
by regexp to the definitions in `tests/bdd-cli/steps/`. A scenario's
Given step names the fixture tree it drives.

**A fixture is a directory, not a document — there is no manifest.**
Trees under `tests/bdd-cli/fixtures/<name>/` hold the designed
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
Lint with `golangci-lint run` against `.golangci.yaml`.

## License

TBD. Not yet released for external use.
