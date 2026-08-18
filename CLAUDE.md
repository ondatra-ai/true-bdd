# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Information

- **Owner**: ondatra-ai
- **Repository**: true-bdd
- **Host**: GitHub (github.com)
- **Clone URL**: https://github.com/ondatra-ai/true-bdd
- **GitHub User**: @killev

## Repository Overview

**TrueBDD** (binary: `true-bdd`) — a Spec-Anchored CLI (aspiring to Spec-as-Source) that drives Claude-mediated checklists over user stories. Extracted from the `awesome-claude-mcp` monorepo into this standalone repo. See `README.md` for the vision, the three-levels-of-SDD taxonomy, and configuration reference.

**This repository tests in Go, and only in Go.** `go-test` for every suite, with `playwright-go` driving the browser inside the web suite — no jest, no `@playwright/test`, no third test framework arriving through the side door. The engine still *supports* `jest` and `playwright` as `framework:` values a host project may declare, and the `build-code-playwright-nextjs` fixture keeps that path covered; this repo declares neither. The parked TypeScript suite at `tests/legacy/bdd-web-playwright/` is the one exception, and it exists to be deleted.

This repo is the **engine**: Go source (`services/bdd-cli/`), prompt templates (`templates/`), the engine config seed (`true-bdd/`), and the BDD test harness (`tests/`). A *host project* consuming the engine supplies its own `true-bdd/` configuration directory (`true-bdd.yaml`, `checklists/`, schemas) plus the five project documents under `docs/`: `docs/architecture/architecture.yaml` (architectural spec; may also carry the BDD vocabulary — the fixtures do), `docs/product/product.yaml` (product document incl. the `roles:` list and the BDD `vocabulary:` — its preferred home), `docs/product/epics/*.yaml`, `docs/product/stories/*.yaml`, and `docs/scenarios.yaml` (the scenario registry). Those are the conventional defaults the seed config declares — every location is driven by `true-bdd.yaml` (`documents:` for the file paths, `paths:` for the directories), so a host may relocate any of them. The engine repo's own root `true-bdd/` is a harness seed — canonical config and checklists that the BDD runner pre-copies into fixture tmpdirs, not a complete host configuration (fixtures supply their `docs/` tree under `input/docs/`, and may override any seed file via `input/true-bdd/`).

## CLI Subcommands

Commands are organized into two supergroups: `us` (story workflow) and `build` (build pipeline).

`us` supergroup:

- `us create <id>` — extract a story from its epic and run the `us-create` checklist.
- `us refine <id>` — load a story from `docs/product/stories/` and run the `us-refine` checklist.
- `us apply <id>` — walk every AC in a refined story, validate against `us-apply`, and merge scenarios into the central scenario registry (path from `documents.scenarios_yaml`, conventionally `docs/scenarios.yaml`).

`build` supergroup — Spec-as-Source regeneration steps:

- `build tests` — **renders every scenario's Go test, then makes sure every step it runs binds to something.** Reads the requirements registry (path from `documents.scenarios_yaml`) and the architectural spec (`documents.architecture_yaml`, override with `--architecture`; `--requirements` overrides the registry). Exits non-zero if a generated file is stale or a scenario is not executable.
  - **Two stages, and only the second can cost money.** First the engine renders one `func Test<Id>` per scenario into the file its `path:` names — deterministic codegen, written by `--fix`, verified by regenerate-and-compare without it. Then it asks each suite's `commands.coverage` which steps bind to nothing and walks **only** the scenarios with gaps, so a converged repository dispatches zero AI turns in about two seconds. With `--fix`, each remaining cell drives a Claude-mediated loop that authors the missing **step definition**; the registry is never modified by the run, and the generated files are re-verified afterwards.
  - "Executable" means every one of the scenario's Given/When/Then steps binds to exactly one step definition in the suite that owns it — except a step prefixed `llm:` or `judge:`, which is run by a model, always resolves, and must never be given a definition: a pattern that captured one would take back the author's explicit choice. Which suite owns it is one join: the `architecture.testing.suites[]` entry whose `service:` equals the scenario's `service:`; its `path:` is where the definitions live, under `<path>/steps/`. The engine now makes that join **in Go** rather than asking a model to make it in prose.
  - This replaced "some test file mentions the scenario id", which died with the feature file. A scenario is no longer copied into a spec that names it; it IS the spec — and the test file beside it is a rendering of that spec, checked against it on every run. Grepping for the id would pass on the registry itself.
- `build code` — walks every suite declared under `architecture.testing.suites[]` (path from `documents.architecture_yaml`, override with `--architecture`), discovers currently-failing tests by running **the command that suite declares**, and walks each failure against the `build-code` checklist, exiting non-zero if any test still fails. With `--fix`, each failed cell drives a Claude-mediated turn that edits production source until the failing test passes; test files and the registry are never modified by the run.
  - **The spec has one `testing:` section for the whole repository**, beside `services:` — how a project runs its tests is a property of the project, not of any one service. A suite names the single `service:` it exercises, and that name is what grants the fix applier its write root. A suite covering two services is two suites: a failure that names no single source root is a failure no fix prompt can be pointed at.
  - Every declared suite carries a mandatory `commands: {record, replay, live}` block; `build code` runs `replay`, and `record`/`live` are reserved. There is **no built-in invocation left** — an incomplete block is a startup refusal (`Cannot start:` on stdout), never a substituted default. Each command is complete and framework-native including its machine-readable flag (`-json` / `--json` / `--reporter=json`), runs from the directory holding the suite's `config:` (or, with no `config:`, whatever directory the CLI was invoked from), and gets only a name filter appended when one test is re-run. Splitting honours quotes and nothing else — see `internal/infrastructure/testrunner/command.go`.
  - Everything checkable is checked in one pre-pass before the first subprocess: every suite names a declared service, its framework routes to a runner, its command splits, its command is machine-readable. What a static check cannot reach — the binary not existing — surfaces as `Cannot run <service>/<suite>:` on stdout when the spawn fails, and *only* that: it is marked `runner.Reported` so the generic startup refusal stays silent, because by then the run has started and a second line headlined `Refusing to start` would contradict the progress line above it. Both refusals also go to slog, because the BDD judge reads the log, not the terminal.
  - Negative coverage lives in `tests/bdd-cli/fixtures/build-code-*`: `missing-suite-command`, `unknown-framework`, `command-not-machine-readable`, `unterminated-quote`, `command-not-executable`, `empty-command` (a command that is present and non-blank but whose only token is an empty argument — the shape the loader's trim check cannot see), `no-services`, `no-suites`, `unknown-suite-service` (a suite whose `service:` is one letter off a declared one, so the fix applier would be granted no root at all), `malformed-spec` (all refusals, zero AI calls) and `nonconvergence` (a full walk that ends `not_fixed` and exits 1, one AI turn). Each plants a *failing* test in the fixture tree, so "reported no failures" can never be confused with "had nothing to find".
  - `build-code-fix-named-test` covers the post-fix rerun being narrowed to the test that was fixed. Its assertion is a canary test that appends a line every time the whole suite runs: discovery runs it once, so a correctly narrowed rerun leaves ONE line and an unfiltered one leaves two — a golden-tree difference caught in replay with no model involved. Exit code alone cannot see it, since an unfiltered rerun also reports success.

**Known, deliberate inconsistency:** the `us-refine-*` fixtures still ship an `architecture.yaml` in the OLD `quality_gate.tests.{integration,e2e}` shape, and `true-bdd/checklists/us-refine.yaml` still asks the model to cite "the framework per service and per layer". Nothing parses those documents — they reach the prompt as text — so the fixtures' cassettes stay valid and the suite stays green. Aligning both means re-recording eleven fixtures at ~5 minutes of real model time each, which belongs in its own commit; until then, do not read those fixtures as the current spec shape.

**Every command reports a startup refusal as `Cannot start: …` on stdout, plus a `Refusing to start` slog record.** The precondition failures are a malformed story number, an unloadable checklist, an unsatisfiable `docs:` declaration, anything the command's `LoadItems` cannot load (the epic for `us create`, the story for `us refine`, the acceptance criteria for `us apply`, the registry for `build tests`, the architectural spec for `build code`), and anything its `Prepare` hook refuses — for `build tests` that means an unplannable registry (a scenario with no `path:`, a `path:` outside its suite, an id that renders no runnable test name), a generated file that is not what the registry renders, and a suite that could not answer which of its steps bind. All but one go through `runner.refuseStartup`; the `docs:` check reports inline instead, because it carries an extra `checklist=` attribute on its slog record and its own `msg`. Left to cobra they would surface only as a stderr line under a usage dump, which reads as "you typed the flags wrong" and is invisible to the fixture harness: `stdout_regex` matches stdout alone, and in replay the judge does not run at all. An error already diagnosed by the code that produced it is wrapped in `runner.Reported` so it is not announced twice.

Every command accepts `--fix` for an interactive loop in which Claude proposes edits for each failed check and the user applies, refines, or exits.

Checklist resolution is by convention via `paths.checklists_dir` in the host's `true-bdd.yaml`: the loader hyphenates the full command path (`us apply` → `us-apply.yaml`, `build tests` → `build-tests.yaml`).

## Model Tiers and Providers

The engine can drive three agent CLIs — `claude`, `crush`, `codex` — and picks between them per checklist role. `engine.models` in `true-bdd.yaml` binds each of three tiers (`xhigh`, `high`, `coder`) to a `"<cli>:<model>"` pair, and `engine.default_prompt_model` / `default_fix_model` / `default_apply_model` name the fallback tier for each of the three AI roles. The value splits on the FIRST colon only.

Each checklist cell runs up to three AI turns, each with its own tier knob in the checklist's top-level `engine:` block — `prompt_model` (validation), `fix_model` (failure → fix prompt), `apply_model` (writes the file) — and any single prompt overrides them with `model:` / `fix_model:` / `apply_model:`. Resolution is **prompt → checklist → `engine.default_<role>_model`** — the bottom of the chain is per-role, so a checklist that names nothing still gets `coder` for the apply turn. Anything unresolvable is a startup error, never a silent substitution.

Key implementation facts:

- `services/bdd-cli/internal/domain/models/provider/` owns the vocabulary (`ModelRef`, `Tier`, `Registry`). The registry is built and validated once in `bootstrap.newAIRouter`.
- `services/bdd-cli/adapters/ai/router.go` is the only `ports.AIPort` implementation; it dispatches on `ModelRef.CLI` to `claude_provider.go` / `crush_provider.go` / `codex_provider.go`.
- `ExecutionMode` is the single permission source for all three CLIs. `WriteGlobs()` / `AllowsBash()` project it onto crush's guard policy and codex's `-s` sandbox, so permissions are declared once.
- The fix generator and applier never see the prompt, so the evaluator resolves their tiers and carries them on `ValidationResult.FixModelTier` / `ApplyModelTier` — the same channel `Docs` already travels on.
- **crush gotchas, both verified against the live CLI:** it silently ignores an unknown model pinned in config (so the model is always passed via `-m`), and it fails OPEN when a hook cannot run (so `verifyCrushGuardEnforces` probes the guard before every turn and refuses to proceed if it does not deny).
- `true-bdd crush-guard` is a hidden subcommand acting as crush's `PreToolUse` write gate, configured through a generated config dir passed via `CRUSH_GLOBAL_CONFIG` — the host's own `.crush.json` is never touched, and hooks are additive. The generated config also pins `options.data_directory` under the run's tmp dir so crush's SQLite state never grows inside the host repo.
- **crush config discovery walks UP from its cwd and MERGES every `.crush.json` it finds** (nearest last; `.git`, a bare `.crush/`, and a non-dot `crush.json` do not anchor anything). `PreToolUse` hooks merge additively and a nested config cannot remove an inherited one — so a `.crush.json` at a repo root hijacks every crush process started anywhere beneath it. That is why this repo has no root crush config, and must not gain one: do not add a tracked `.crush.json`/`crush.json` at the root — it silently blocks every fixture's apply turn.

## Development Commands

```bash
# Validate every document against its schema (requires yamale: pip install yamale).
# Pairing is by convention: true-bdd/<key>-schema.yaml validates documents.<key>
# from true-bdd.yaml, so a new schema needs no script edit. The key is the whole
# name, underscores included — `architecture_yaml-schema.yaml`, not
# `architecture-schema.yaml`; a schema whose key names no document HARD-FAILS the
# gate rather than sitting unused. Runs in CI's `gates` job and in
# .claude/skills/pr-commit/gates.sh.
./scripts/validate-schemas.sh

# Build (requires Go 1.25 and the `claude` CLI on $PATH)
mkdir -p ./bin && go build -o ./bin/true-bdd ./services/bdd-cli

# Unit tests
go test ./...

# End-to-end BDD scenarios — real Claude calls, ~3-5 min per scenario.
go test -tags bdd -timeout=180m ./tests/bdd-cli/...

# The web suite: builds services/bdd-web, boots it on a free loopback
# port, drives chromium through playwright-go. Out of every gate on
# purpose — it needs node, a build, and a browser. It FAILS rather than
# skips when node/npm are missing (`BDD-WEB: TOOLCHAIN MISSING`), since
# a suite in no gate cannot afford a silent skip; -allow-missing-toolchain
# asks for the skip explicitly, on the command line where it is visible.
go test -tags bdd -timeout=25m ./tests/bdd-web/

# Hermetic record/replay of the scenarios' AI calls (the -mode flag must
# come after the package path). replay serves per-fixture cassettes/ via
# the tests/libraries/aiproxy PATH shim and spawns NO model at all — not
# even the judge; a scenario without cassettes or without a recorded
# outcome FAILS.
# The whole suite replays in well under a minute for zero cost.
# record runs the real CLIs, grades with the judge, and publishes the
# recording ONLY if the fixture passes: a failed run's cassettes stay in
# the session's .cassettes-staging/ and the previous good ones are left
# alone.
go test -tags bdd ./tests/bdd-cli/ -mode=replay
# One scenario, by its generated test's name. Every failure a re-recording
# would fix prints this line with the fixture named in a trailing comment,
# because nobody remembers ids.
go test -tags bdd -run '^TestE2E016$' ./tests/bdd-cli/ -mode=record

# The three guards, in under a second, on a machine with no toolchain at
# all: does every scenario have a generated test, does every fixture tree
# have a scenario, does every step bind. The third is the one
# `build tests` runs on every walk.
go test -tags bdd -run '^Test(ScenarioCoverage|FixtureTreesArePaired|StepCoverage)$' ./tests/bdd-cli/

# Browse the run report — every session, live. Rescans tmp/test_run every
# 15s, so a suite still running streams into an open page.
go run ./tests/libraries/cmd/report-server   # http://127.0.0.1:7331
# -addr 0.0.0.0:7331 to reach it from another machine (e.g. over Tailscale)

# Lint
golangci-lint run
```

### Running the CLI

The tool spawns `claude` as a subprocess. When invoking it from inside a Claude Code session, unset `CLAUDECODE` first so the child has a clean environment:

```bash
env -u CLAUDECODE ./bin/true-bdd us create 4.1
```

`us refine` drives many sequential Claude calls and typically takes **about 5 minutes** end-to-end. Do not abort early; poll or wait rather than killing the process.

## BDD Harness

**The registry drives the suite, and the tests are committed artefacts of it.** `docs/scenarios.yaml` says what the engine must do; `build tests` renders one Go test per scenario into the file that scenario's `path:` names; `tests/libraries/bddgo` runs it. There are no feature files. A scenario with a step no definition matches FAILS — it is not skipped and not silently absent, and that failure is exactly the gap `build tests --fix` closes.

**Every test file under `tests/*/` beginning `// Code generated` is written by `true-bdd build tests --fix` and by nothing else.** It is deterministic codegen, not an AI turn: the same registry always renders the same bytes, which is what lets `build tests` (without `--fix`) verify the tree by regenerating and comparing. A stale file is a startup refusal naming the regeneration command, because a walk over a tree whose tests no longer transcribe the registry measures a repository that does not exist. The AI turn survives exactly where judgement is needed — authoring the step definition a new step binds to.

```go
// tests/bdd-cli/build_code_test.go, generated
func TestE2E001(t *testing.T) {
	s := scenarios.New(t, "E2E-001")
	s.Given(`the "build-code-command-not-executable" project tree`)
	s.When(`the System Architect runs "build code"`)
	s.Then(`the command exits with code 1`)
	s.But(`no file outside "tmp" changed`)
	s.Done()
}
```

The step text is stated literally, so what a reader sees in the test is what the machine runs — and `Done()` compares those steps against the registry entry before running any of them. A disagreement in either direction stops the scenario dead with both sides printed; nothing runs partially. The registry still supplies what is not behaviour: the description, the `service:`, and the `timeout:` (how long a machine needs is not something a scenario can assert, so it must not appear in generated code).

**`path:` is required on every scenario and never defaulted.** Several scenarios may name one file and are emitted into it in id order — that is how a command's behaviours group into a file a reader can hold instead of one file each. Which behaviours belong together is editorial, and a generator that guessed would scatter them. The generator refuses a `path:` outside the owning suite's directory, one not ending `_test.go`, one inside that suite's `steps/` tree (the fix turn's), two scenarios sharing a file across services, and two ids that render one Go name.

**Hand-written guards keep the generated set honest** (`tests/*/coverage_test.go`). Hand-written on purpose: a generated guard is one the generator could silence by regenerating it. `TestScenarioCoverage` — every owned scenario has exactly one generated test, in the file the registry names, and every generated test names a scenario that exists. `TestStepCoverage` — every step binds to exactly one definition. `TestFixtureTreesArePaired` — every fixture tree is named by a scenario; bdd-cli only, since bdd-web drives no fixture trees, so that suite has two guards and bdd-cli has three. None brings a harness up, so all of them answer in under a second on a machine with no toolchain.

**`build tests` answers its own question deterministically, and only then spends a model.** `architecture.testing.suites[].commands.coverage` names a command the engine runs before walking anything; the suite writes JSON to `$TRUEBDD_COVERAGE_REPORT` and the walk is narrowed to the scenarios with gaps. A converged repository costs **zero AI turns** and about two seconds. The command's exit status is ignored — a coverage test legitimately reds when it finds gaps — but a missing report is a refusal: "could not answer" is not "nothing to report".

The report says which scenarios it **examined**, not only which have gaps, and the narrowing keys on that. An empty gap list is meaningless on its own: a suite that died after writing its file, or one reading a different registry than `--requirements` named, reports no gaps for scenarios it never looked at — and reading that as "every step binds" drops them from the walk unchecked. A scenario absent from `examined` is walked. The engine also refuses an unknown `schema`, a `failure` the suite reports about itself (distinct from an ambiguity, which is a finding about a real step), and a report naming a gap in a scenario it never claims to have examined. `coverage:` is the one optional command; a suite declaring none falls back to a model reading the source in prose, so adding it is an upgrade rather than a break. After a `--fix` walk the generated files are re-verified, because the applier's write roots cover the whole test tree and a scenario "fixed" by editing the test that asserts it is one nothing checks any more.

`tests/libraries/bddgo` is generic over the state a suite keeps, so it serves both surfaces: `tests/bdd-cli/steps` keeps a subprocess and a working tree, `tests/bdd-web/steps` keeps a browser page. A generated file calls a non-generic `scenarios.New(t, "<id>")`, which works because each suite ships one hand-written shim — `tests/<suite>/scenarios/scenarios.go` — that fixes the state type and owns everything a run shares. The cheap part (the flag, the suite, the step table) happens in `TestMain`; the expensive part (the binary, the aiproxy shim, the judge for bdd-cli; the npm build, the server, chromium for bdd-web) sits behind a `sync.Once` fired by the first scenario, so a guard asking about two files on disk never pays for a browser. A missing CLI or toolchain becomes a skip *per scenario* with its reason, rather than one skipped parent that hides the count.

bddgo's own refusals are worth knowing — a suite the spec does not declare, a step matched by two definitions (which one runs would depend on registration order), a generated test naming a scenario the registry no longer holds or a suite that does not own it, a `Done()` that was never called (a test declaring a scenario and running nothing would otherwise pass). `Unbound()` answers "is every written-down behaviour executable?" without running anything, which is what makes that question affordable on every `build tests` walk — and it is the same resolver the run uses, so the two can never disagree.

**Not every step can be settled by a regexp, and the ones that cannot say so in their own text.** Two prefixes hand a step to a model instead of to a step definition, and each is legal in exactly one block:

| prefix | legal in | means | strategy |
|---|---|---|---|
| `llm:` | `given:` / `when:` | **act** — a model performs the step | one agent turn, at the step's position |
| `judge:` | `then:` | **rule** — a model rules on the clause | collected as the scenario runs, then **one** call after every other step passed |

`judge:` in a given/when block and `llm:` in a then block are both refusals, not reinterpretations: the two prefixes name two different engines, and a step that silently swapped engines depending on where it was pasted would be a step nobody could predict from its text. `llm:` needs the suite's state to implement `bddgo.Actor` and `judge:` needs `bddgo.Judgeable` — the bdd-cli state implements only the latter, so an `llm:` step in a CLI scenario stops the run instead of quietly doing nothing. A clause against a state that cannot grade it is likewise refused: an ungraded clause still READS as coverage. Use `judge:` only for what no comparison can settle — that two wordings mean the same thing, that a rewrite kept its substance. Everything else belongs in a step definition, and 34 of the 46 bdd-cli scenarios reach no judge at all. Neither suite implements `Actor` yet, so `llm:` is a declared capability with no implementer — deliberate forward work for the web scenarios, where "close whatever dialog is covering the list" is a real step and a testid would be a spoiler.

The bdd-cli suite's scenarios each drive a **fixture tree**: a folder under `tests/bdd-cli/fixtures/<name>/` holding the designed host-project content under `input/`, `cassettes/` — the recording that lets the run go hermetically under `-mode=replay`, one directory per AI call plus `golden.json` — and optionally the scaffolding described below. **There is no manifest.** A fixture is a directory, not a document: what the run DOES (the invocation, the exit code, the stdout, the interactive input, the clauses it is judged against) all comes from the scenario. The scenario's Given step names the tree. Every one is recorded; a scenario without a recording fails in replay rather than skipping, and **a tree no scenario names fails the whole run** — a directory nobody executes looks exactly like a directory that passes.

Three optional files sit at the fixture ROOT, beside `input/` rather than inside it:

- **`prep.sh`** — installs what the CLI cannot (an npm install, a browser download). Run in the tmpdir after the input overlay and before the pre-run snapshot, so `node_modules/` never reaches the graded diff. A non-zero exit aborts the fixture.
- **`teardown.sh`** — tears down resources that outlive the CLI (Playwright's `webServer` brings up a `docker compose` stack). Runs after the post-run snapshot whatever the verdict, against a fresh 2-minute context; failures are logged and never mask the primary verdict.
- **`checklist-prompts.yaml`** — which prompts of a shipped checklist the run walks, keyed by stem. The tree ships a story carrying exactly one designed defect, and narrowing the checklist is what makes the walk evaluate the check that defect trips. A present-but-empty declaration is refused rather than read as "no filter".

Each script becomes ONE command, and both are read **by content** rather than executed by path: a script overlaid into the tmpdir would become a file of the host project the run is grading. The scenario still names all three preconditions as Given steps — `the project's browser test dependencies are installed`, `the "us-refine" checklist is narrowed to its "…" prompt` — and the step definitions behind them verify the tree really establishes them. Same division of labour as `the "<name>" project tree`: the registry names the precondition, the tree provides it.

The record/replay proxy is `tests/libraries/aiproxy/`: one binary installed as `claude`/`crush`/`codex` in a shim dir the harness prepends to the CLI subprocess's PATH (the engine is unmodified). A cassette per call stores argv, stdin, the output streams, exit code, and the working-tree fs-diff — with run-volatile paths normalized (`{{CWD}}`, `{{RUN_DIR}}`, `{{HOME}}`) in paths, contents, AND stdout, because the engine parses result files out of the response via `FILE_START: tmp/<run-dir>/…` markers. Replay matches cassettes by sequence per binary, verifies a normalized request hash (mismatch = loud "stale cassette" failure, exit 86 — never a silent fall-through to real CLIs), applies the fs-diff before emitting output, and lingers until the engine closes stdin (exiting earlier truncates the response mid-pipe).

**Recordings are sanitized before they are written.** Cassettes are committed to a public repository, so the shim drops the agent CLI's `system`/`init` inventory — the tool schemas, connected MCP servers, skills, plugins, slash commands and memory paths — and rewrites the recording machine's home directory to `{{HOME}}`. The engine reads none of it (`adapters/ai/claude_provider.go` answers a system message with two `slog.Debug` calls), and it was 47% of the bytes. `.claude/skills/pr-commit/scan-recordings.sh` re-checks the committed recordings for home paths, session inventory, credentials and e-mail addresses; a hit means fix the shim and re-record, never hand-edit a cassette.

**A run is graded at three levels, and each owns something the others cannot say.**

1. **The scenario's own Then steps** — every mechanical assertion: the exit code, the stdout patterns, which files were created/modified/left alone, how many AI turns the engine dispatched and which test runners it spawned, the shape of a story or registry it wrote. Asserted by the suite's step definitions on every run, in every mode. These are behaviour, so they live in the registry.
2. **The recording**, in replay only: `cassettes/golden.json`, the run's diff outside `tmp/`, written by a passing record run and compared byte-for-byte, plus **every cassette consumed** (`runner.CheckCassettesConsumed` — the one divergence a request hash cannot see, since a call that never arrives matches nothing) and the request hash per call (prompt drift → exit 86). In replay every AI-written file was materialised from a cassette, so grading its content re-grades a fixed artefact; what can still move is the engine — which cells it walks, how it parses each response, and the files it writes itself, the registry merge above all.
3. **The judge** — the scenario's `judge:` clauses, in live and record only, because the model output is new and only a reader can say whether it preserved a meaning. In replay the clauses are collected and the model is never spawned: the golden comparison discharges them, and it is stricter than a reading.

Residual gap, stated plainly: a change that alters only `tmp/` scratch and neither stdout nor an output file is not caught. `tmp/` is excluded because its paths carry a per-run timestamp, so pinning them would fail every future run rather than any regression.

**The judge prompt is generated, not authored.** `runner.buildJudgeUserPrompt` renders the clauses into the numbered rubric the judge always received, under a scope paragraph telling it what is NOT its job — the exit code, the output and the file set were asserted mechanically before the call was made, and a model left to re-derive them from the diff fails runs for files the suite already approved. The `## Tolerances` noise policy is one harness constant; it used to be near-identical prose in all 46 rubrics, which was 46 places to update and 46 chances to disagree. Nothing else feeds it: a run with no clauses never reaches a judge.

**Noise is excluded from the diff where it can be, and tolerated in the prompt only where it cannot.** `runSnapshotSkipDirs()` drops `.git`, `node_modules/`, `.next/`, `test-results/` and `playwright-report/` from both snapshots, so they reach neither the judge nor the golden — the dependency trees because prep.sh installed them before the run, the last two because they are what a test runner writes ABOUT a run rather than as part of the project. A tolerance a judge has to read is a tolerance it can misread, so excluding beats tolerating; `tmp/` is the one that must stay a tolerance, since its per-run timestamped paths are the engine's own scratch and the log inside it is evidence several steps read.

A scenario, in `docs/scenarios.yaml`, showing both halves:

```yaml
E2E-037:
  description: A vague qualifier in an acceptance criterion's description must drive the fix loop until the description is measurable
  service: bdd-cli
  path: tests/bdd-cli/us_refine_test.go   # where its generated test goes; shared with the other us refine scenarios
  user_stories: []
  timeout: 10m            # harness budget, not behaviour; absent means the suite default
  merged_steps:
    given:
      - the "us-refine-fix-desc-qualifier" project tree
      - and: the "us-refine" checklist is narrowed to its "whether its description field contains a vague word" prompt
      - the Product Owner answers "1" to every prompt
    when:
      - the Product Owner runs "us refine 96.6 --fix"
    then:
      - the command exits with code 0
      - stdout matches ALL CHECKS PASSED!
      - and: the only file changed outside "tmp" is "docs/product/stories/96.6-document-summary-desc-qualifier.yaml"
      - and: the story "docs/product/stories/96.6-document-summary-desc-qualifier.yaml" has 4 acceptance criteria
      - and: the description of acceptance criterion "AC-3" of the story "docs/…-desc-qualifier.yaml" does not match (?i)\b(properly|quickly|…)\b
      - but: stdout does not match Hit max apply attempts
      - and: 'judge: the four acceptance criteria still cover the substance of the seed — a 150-word summary, the unshared-document message, the timeout message, and the first-10000-words notice — with every exact message and numeric threshold intact'
```

`and:`/`but:` are display-only sugar, interchangeable and unlimited, exactly as Cucumber defines them — `flattenSteps` excludes the keyword from the match text so one definition serves a step wherever it sits. Convention: `but:` for a group's negative clause.

**Two families of path argument, and the difference matters when one fails.** The `the story "…"` and `… of the story "…"` steps take a **glob that must resolve to exactly one file**: zero matches and two matches are each their own failure, never a silently-picked file, which is what lets a scenario name a story whose slug the fix loop chose (`docs/product/stories/99.1-*.yaml`). The diff and content steps — `the file "…" is created/modified/unchanged/matches`, `has exactly N lines` — take a **literal path**, compared against diff entries or joined onto the run dir, so a mistyped one reports "the run did not create it" rather than a glob miss. Where a created file's name is not knowable, use `exactly N file(s) matching "<glob>" is created`, which does glob.

The stdout pattern runs to the end of the line, undelimited: several of them carry double quotes of their own (`msg="Refusing to start"`), and any quoting scheme would push escaping into a document meant to be read. The same holds for the regex tail of `the file "…" matches`, `… does not match`, and the acceptance-criterion assertions.

The runner builds each run's tmpdir in two layers: first it pre-populates the repo-layer engine ingredients (the tracked `true-bdd/` config seed and `templates/`) so fixtures exercise the live prompt templates; then it overlays the fixture's input tree on top, which holds the designed host-project content — `docs/` at minimum (synthetic product doc, architecture, epic, story, seeded requirements registry), plus, when the scenario needs them, project sources and tests, a per-fixture `CLAUDE.md`/`.claude/`, or engine-config overrides under `true-bdd/`. Files inside the input tree win over the pre-populated layer, so a fixture may deliberately ship a per-fixture variant of a checklist or config.

**A fixture input tree carrying `.go` files must also carry a sentinel `go.mod`.** Run dirs live at `tmp/test_run/…` INSIDE the repo module, so without a module boundary the root `go test ./...` and golangci-lint compile whatever a past run left there — including the deliberately-failing tests the `build code` fixtures plant, which would turn the repo's own gates red for reasons no one wrote. The per-fixture `input/go.mod` is still required on its own account: the runner materialises that tree into a run dir where it has to be a real module.

**`tests/bdd-cli/fixtures/go.mod` closes the same trap from the other side.** A cassette's `fsdiff/after/` is a recording of the files an AI turn *wrote*, and it carries no `go.mod` — the fsdiff captures only what changed, and the input tree's `go.mod` did not. So a recorded `.go` file lands directly in the repo's module, where the root `go test ./...` compiles it. One sentinel at the fixtures root covers every fixture's inputs and recordings at once. It is never materialised into a run dir, because the runner overlays `<fixture>/input/`, not the fixture directory.

The runner snapshots the tmpdir after prep but before the run, so the diff fed to the judge only contains files the run itself created or modified. After the CLI exits — and only in live or record — the suite asks Claude (via the `services/bdd-cli/claudecode/` wrapper) to compare that diff against the scenario's clauses and return PASS / FAIL.

Tests are gated by a `//go:build bdd` tag so they're invisible to default `go test ./...`. **`TestBDDFixtures` is gone**: one scenario is now one top-level Go test named after its id (`TestE2E016`), so `-run` filters name a scenario rather than a subtest path. Nothing in `gates.sh` or CI used such a filter, but muscle memory did — every failure a re-recording would fix prints the exact `-run` line with the fixture named in a trailing comment, and the run directories under `tmp/test_run` are still named after the fixture tree, so the report server, the coverage tool and a person looking at disk all read what they always did. In live and record the suite skips when `claude` — or any CLI a model tier names — is not on `$PATH`. Replay skips for nothing: it spawns no model at all, so a missing CLI is not a reason to stay silent about whether the engine still behaves.

**Replay runs in both gates** — `.claude/skills/pr-commit/gates.sh` and CI's `gates` job — because it is the only check that watches the engine run end to end, and it costs about a minute and no money. It also runs the bdd-cli suite's three guards for free, since they live in the same package. A gate failure says which kind: `stale cassette` (exit 86) means a prompt, template or engine change altered a request and the affected fixtures need re-recording; a golden mismatch means the run produced different files than the recording, which is the regression signal and worth reading before re-recording. The live suite stays manual.

**The bdd-web suite's scenario guard runs in both gates too, and only that guard** — `-run '^TestScenarioCoverage$' ./tests/bdd-web/`. The suite itself needs node, a Next.js build and a browser and stays manual, but "does every bdd-web scenario have a generated test?" needs none of those and answers in milliseconds. Without it the web half of the registry could go stale for weeks and the only thing that would notice is a deliberate run nobody makes.

`TestStepCoverage` is deliberately NOT in the gates for bdd-web yet, and the reason is a number. The suite owns 244 scenarios — the 243 ported from the legacy Playwright suite, plus the one landing scenario whose steps do bind — and `tests/bdd-web/steps` registers four definitions, so the guard reports ~1680 unbound steps across those 243. Gating on it today would not fail the gate, it would make the gate unsatisfiable — which is why the web scenarios are described above as declared forward work. Gating on it would also block every unrelated PR in the repo, not just the port's own. It joins both gate lines in the change that lands the web step definitions — tracked at <https://app.clickup.com/t/86cb6fjwy>, which is the ticket that closes when both lines are restored and green. The same arithmetic is why `build tests` on THIS repository does not yet cost zero AI turns: the bdd-web coverage command truthfully reports 243 scenarios with gaps, and the walk takes them all.

If the scenario runs `--fix`, its Given step supplies the keystrokes: `the <role> answers "1" to every prompt` sets the subprocess's stdin to a generous run of that answer, one line per prompt. Surplus lines are harmless — EOF makes the CLI exit cleanly — so a fixed supply answers every prompt any single fix loop raises, and no fixture has to keep a transcript nobody can check.

## Project Structure

The repo follows the same `services/<name>/` + `tests/<name>/` layout TrueBDD asks
of a host project: one directory per service, and a test suite named after the
service it exercises — plus `tests/libraries/` for everything both suites share,
and `tests/legacy/` for what is on its way out.

- `services/bdd-cli/` — the Go module (`github.com/ondatra-ai/true-bdd`), i.e. the engine itself. Entry point `services/bdd-cli/main.go`; builds to `./bin/true-bdd` (gitignored). Exercised by `tests/bdd-cli/`.
  - `services/bdd-cli/cmd/` — cobra command tree (`root.go`, `us.go`, `build.go`).
  - `services/bdd-cli/claudecode/` — Claude Code subprocess SDK wrapper (client, transport, message parsing).
  - `services/bdd-cli/adapters/ai/` — Claude client adapter and execution modes.
  - `services/bdd-cli/internal/app/` — bootstrap container, command implementations, the checklist engine.
    - `internal/app/generators/scenariogen/` — renders a registry scenario into a Go test. Deterministic: `BuildPlan` refuses the whole registry before writing a byte, `Render` always runs `format.Source` (a compare that fights gofmt reports drift on a correct file), and `Write` refuses a target that carries no generated marker. The template is `//go:embed`ed rather than shipped under `templates/`, because those are prompt templates a host tunes and this one must compile against bddgo's API.
    - `internal/infrastructure/stepcoverage/` — runs a suite's `commands.coverage` and reads the JSON report back. Ignores the exit status on purpose and refuses a missing report: "could not answer" is not "nothing to report".
  - `services/bdd-cli/internal/domain/` — story/checklist/registry/architecture models and ports.
  - `services/bdd-cli/internal/infrastructure/` — loaders (config, epic, story, checklist, registry, architecture), template rendering, test runners (go test / jest / playwright), fs, console input.
    - `architecture/loader.go` decodes exactly two things: `architecture.testing.suites[]` and the `name`/`path`/`language` of each `architecture.services[]` entry. Everything else a host writes under `architecture:` — dependencies, patterns, design systems, non-test quality gates — is spec for its own readers and stays undecoded, which is why `true-bdd/architecture_yaml-schema.yaml` is the only thing that would notice a typo in it.
  - `services/bdd-cli/internal/pkg/` — `console` (terminal UI output), `errors`.
- `services/bdd-web/` — the Next.js relay + UI (a **sentinel nested Go module** so root-level `go`/`golangci-lint` never descend into its `node_modules`). Its `src/` is GENERATED code and is gitignored — the scenarios and the suite that runs them are the spec. `design/` holds the design system, `SPEC.md`, and the `proto-workspace/` design-truth prototype. Exercised by `tests/bdd-web/`.
- `templates/` — prompt templates (Go `text/template` with sprig), named `<command>.<role>.prompt.tpl`.
- `true-bdd/` — the engine's canonical config seed (`true-bdd.yaml`, `checklists/`, `<key>-schema.yaml`); pre-copied together with `templates/` into every BDD fixture tmpdir as the repo layer. The schemas are host lint contracts, not engine inputs: the engine never parses the documents they pin, so only `scripts/validate-schemas.sh` enforces them.
- `scripts/` — repo tooling invoked by CI and the commit gates; `validate-schemas.sh` is the schema gate described under Development Commands.
- `tests/` — all end-to-end / BDD tests live here (unit tests stay with their code, e.g. `services/bdd-web/src/tests/unit/`). One directory per SUITE, one for the libraries they share:
  - `tests/bdd-cli/` — the suite for `services/bdd-cli`: `*_test.go` (**generated** — one per command family, holding one `func Test<Id>` per scenario), `main_test.go` + `coverage_test.go` + `scenarios/` (hand-written: the TestMain shim, the state binding, the three guards), `steps/` (the step definitions, every one authored by `build tests --fix`), `fixtures/<name>/` (the project trees its scenarios drive, each with its input tree and cassettes).
  - `tests/bdd-web/` — the suite for `services/bdd-web`: same shape — generated `*_test.go`, hand-written `main_test.go`/`coverage_test.go`/`scenarios/`, and `steps/` (browser verbs on `playwright-go`, plus the harness that builds the app, boots it on a free loopback port, and assembles the Playwright driver from npm because the library's own CDN is dead).
  - `tests/libraries/` — everything neither suite owns alone:
    - `bddgo/` — the registry-driven scenario runner (see BDD Harness). Generic over the state a suite keeps, so it knows nothing about subprocesses or browsers.
    - `runner/` — fixture-directory loading (the input tree, `prep.sh`/`teardown.sh`/`checklist-prompts.yaml` — there is no manifest), tmpdir assembly, the CLI invocation, the per-run diff, the judge, goldens, the harness recorder, session metadata.
    - `aiproxy/` — the record/replay PATH shim for the AI CLIs (see BDD Harness); built by the harness when `-mode` is not `live`.
    - `fstree/` — shared tree snapshot/diff, used by both the runner's per-run diff and the shim's per-call fs-diff.
    - `reporter/` — parse-only loaders for a session's on-disk artefacts. `reportserver/` — the store, JSON API, diff layer and embedded UI.
    - `materializer/` — the Go fixture materializer, built by the parked suite to overlay fixtures.
    - `cmd/report-server/`, `cmd/coverage/` — the two binaries (`go run ./tests/libraries/cmd/...`).
  - `tests/legacy/bdd-web-playwright/` — the PARKED TypeScript Playwright suite: 60 specs and its own `node_modules`, behind a sentinel `go.mod`. Every one of those specs is now a registry scenario (`E2E-048`…`E2E-290`, rendered into `tests/bdd-web/{protocol,workspace,design,ai}_test.go`) — but the scenarios are not executable yet, so this suite is still the only thing that actually tests the web surface and must stay until the step definitions land. Delete it per family, in the same commit that binds that family's steps; a phase that does not delete what it retires is not done. Do not add to it.
- `tmp/` — runtime working dir for prompt/response artifacts (gitignored).
- `docs/history/` — conversation history captured by the `.claude/hooks/history.py` hook (`<UTC-ts>-<session8>-<slug>.md`), gitignored. `docs/history/hook-state` holds a single line — the current file's name — shared across sessions so a new session continues the same file. `/new-task` (`.claude/commands/new-task.sh`) deletes it so the next prompt opens a fresh file, and also resets the repo to a clean state: local changes discarded, untracked files removed (ignored files kept), and the current branch fast-forwarded from origin (the branch is never switched).
- `tmp/test_run/<YYYY-MM-DD_HH-MM-SS>/<fixture-name>/` — per-fixture working dir created by the BDD test harness. Predictable, never auto-cleaned; wipe manually when you want to reclaim disk. Everything the report reads lives here. The `bdd-cli-logs/` files below are recorder **sidecars**, written from the recorder's `t.Cleanup` so they survive a `t.Fatalf` and land strictly after both snapshots, never entering the judge's diff. `tmp/true-bdd.log.json` is not one of them — the engine writes it from its own process, during the run:
  - `bdd-cli-logs/harness.json` — wall clock, verdict, exit code, structural diff, judge window and cost. **Its presence means the fixture is final** (it is the last byte written into the directory), which is exactly the cache key the report server keys on. Written write-then-rename so a reader cannot catch it half-written.
  - `bdd-cli-logs/judge-{system,user,response}.txt` — the judge call verbatim, so two runs' graders can be diffed. Absent for sessions recorded before schema 2.
  - `bdd-cli-logs/manifest.json` — what this run was held to **as it resolved it**: the command, the expected exit code, the stdout patterns and the judged clauses, all filled in by the scenario's own steps. Without the snapshot a report shows today's registry against an old run's actuals, and comparing "expected" across runs is meaningless.
  - `tmp/true-bdd.log.json` — the engine's own slog: every AI turn's role, model, duration, cost and tokens.
  - `harness.log.json` at the session root is the *test process's* slog, not run data — no fixture names, no verdicts. Only the judge's `AI turn usage` records matter, and the recorder already folds those into `harness.json`.
  - `session.json` at the session root — what is true of the whole invocation rather than any one fixture: the `-mode` it ran under and the fixtures it *planned* to run (discovery minus the `-run`/`-skip` filters, computed by `runner.PlannedTests` over the generated tests' function names and then mapped back to fixture names). Written before the first fixture starts, which is what lets a live report say "6 / 0 / 19" instead of a denominator that grows as the run proceeds. Both facts are absent for older sessions and must render as unknown, never as `live`.
  - `.cassettes-staging/<fixture>/` at the session root — record mode's in-flight cassettes plus the `golden.json` written from the run's own diff, promoted into `fixtures/<name>/cassettes/` only when the fixture passes. At the session root, not inside the run dir, because anything written there lands in the graded diff.
- **Run report** — served, not generated. `go run ./tests/libraries/cmd/report-server` reads every session under `tmp/test_run`, rescans on a 15s interval, and serves a single-page UI at `127.0.0.1:7331`: run list, per-run fixtures, per-test expected-vs-actual with the phase timeline, and comparison of any two runs — test by test, then turn by turn. Every surface states the AI mode (run list column, run tiles, test tile, matrix column) — a green replay run and a green live run prove different things — and scores a run as **passed / failed / planned**. Comparison uses a real Myers diff (`znkr.io/diff`), aligning turns on `(checklist cell, role)` rather than turn number, so a run that needed an extra retry shows one insertion instead of shifting every later row. Loaders live in `tests/libraries/reporter/` (parse only); the store, JSON API, diff layer and embedded UI in `tests/libraries/reportserver/`.

## Architecture Principles

### Quality Over Cost Principle
**QUALITY IS PARAMOUNT - TIME, PRICE, AND TOKEN USAGE ARE LOWEST PRIORITY** 🎯

When making decisions about true-bdd implementation:
- ✅ **Prioritize output quality**: Always choose the approach that produces the best results
- ✅ **Multi-stage generation is acceptable**: If it takes 3x tokens to get perfect output, do it
- ✅ **Take time for quality**: Generation time is not a concern if results are better
- ✅ **Token usage is not a constraint**: Use as many tokens as needed for comprehensive prompts
- ❌ **Never compromise quality for speed**: Fast but mediocre output is unacceptable
- ❌ **Never optimize for token cost**: Cutting corners on prompts to save tokens is wrong

**Examples:**
- Two-stage generation with critique? ✅ Do it
- Embed full articles in prompts? ✅ Do it
- Multiple validation passes? ✅ Do it
- Self-critique and revision loops? ✅ Do it

### Core Data Flow Principle
**NO CACHING, NO LOADERS, NO UNNECESSARY INTERFACES - JUST DIRECT DATA FLOW!** 🎉

- ✅ **Direct data flow**: `Epic File → StoryFactory → StoryDocument → Generators`
- ✅ **Single source of truth**: StoryDocument contains all needed data
- ✅ **No abstraction layers**: Components work directly with concrete data
- ✅ **No caching complexity**: Load once, use directly
- ✅ **Simple interfaces**: Functions take concrete types, not abstractions

Implementation guidelines:
- **Extend domain models** (like StoryDocument) with required data instead of creating loaders
- **Pass complete data structures** to functions instead of IDs that require loading
- **Load data once** at the factory level and populate the complete structure
- **Avoid interfaces** unless there's a genuine need for multiple implementations
- **Question every layer** — if it doesn't add real value, remove it

```go
// ✅ GOOD: Direct data flow
type StoryDocument struct {
    Story            Story
    Tasks            []Task
    DevNotes         DevNotes
    Testing          Testing
    QAResults        *QAResults
    ArchitectureDocs *docs.ArchitectureDocs  // All data included
}

func (g *Generator) Generate(ctx context.Context, storyDoc *StoryDocument) (Result, error) {
    // Direct access to all needed data
    return processData(storyDoc.Story, storyDoc.ArchitectureDocs)
}

// ❌ BAD: Unnecessary abstractions
type StoryLoader interface { LoadStory(id string) (*Story, error) }
type DataCache struct { /* caching complexity */ }
```

## Go Output Conventions

### Logging vs Terminal UI Output

**Rule**: Use `slog` for application logging, use `console` package for terminal UI output.

| Use Case | Package | Example |
|----------|---------|---------|
| Debug/info logs | `slog` | `slog.Info("Processing story", "id", storyID)` |
| Error logging | `slog` | `slog.Error("Failed to load", "error", err)` |
| User prompts | `console` | `console.Print("Your choice: ")` |
| Status messages | `console` | `console.Println("ALL CHECKS PASSED!")` |
| Visual separators | `console` | `console.Separator("=", 80)` |
| Section headers | `console` | `console.Header("RESULTS", 80)` |

**Why separate?**
- `slog` produces structured logs with timestamps/levels (for debugging, monitoring)
- `console` produces clean terminal output (for CLI user interface)

**Location**: `services/bdd-cli/internal/pkg/console/console.go`

**Linter**: The `console` package is excluded from the `forbidigo` linter in `.golangci.yaml`

## Go File Naming Convention

### Single Entity Single File Principle

**Rule**: Each Go file should contain one primary entity (struct/interface/type), and the filename must match the entity name in snake_case.

**Examples:**
- `GitHubService` struct → `github_service.go`
- `ClaudeClient` struct → `claude_client.go`
- `BranchManager` struct → `branch_manager.go`
- `ThreadProcessor` interface → `thread_processor.go`

**Important**: Treat "GitHub" as a single word (not "Git" + "Hub"):
- ✅ `GitHubService` → `github_service.go` (correct)
- ❌ `GitHubService` → `git_hub_service.go` (wrong)

**Exceptions (Acceptable):**
- Files with only functions/constants (e.g., `logging.go`, `errors.go`)
- Re-export files that aggregate types from internal packages
- Multiple closely related types (e.g., `ExecutionMode` + `ModeFactory` in `execution_mode.go`)
- Data structures bundled with their primary entity (e.g., `ArchitectureLoader` + `ArchitectureDoc`)

**Enforcement:** through code review; no automated linter rule in `.golangci.yaml` yet.

## Shell Usage

**Do not use `cd` to change the working directory.** Always run commands from the repository root using absolute paths or `-C <path>` flags. This keeps paths predictable across turns and prevents the working directory from drifting into nested subdirectories.

## Task Management

**ClickUp is the task manager for this repo**: <https://app.clickup.com/90151491867/v/l/li/901523097822> (list id `901523097822`). Reach it with the ClickUp MCP tools — `createTask`, `listTasks`, `updateTask`, `getTask`.

**"Defer this" / "deferred task" means: write it to that ClickUp list.** It does not mean the session todo list, which evaporates when the session ends. A deferred task that only exists in session state has been lost, not deferred.

Deferring is not filing a title. Write the task so it can be picked up cold, by someone without this conversation, and use `markdownContent`:

- **Why** — the problem and what prompted it, not just the change.
- **What to change** — the concrete edits, with `file:line` for each reference. Grep for the references before writing, so the list is real rather than remembered.
- **Verification** — the commands that prove it worked, and the one behaviour the change could silently break.
- **Context** — why it was deferred rather than done, so a reader knows whether the reason still holds.

Deferring is also a scope decision: it keeps unrelated work out of the change in hand, so each lands as its own reviewable commit.

## Response Style

Lead with the answer or result; drop restatement, framing sentences, and narration of what a tool did. Tables and short bullets over paragraphs. Findings and caveats stay — the prose around them goes.

**How to apply:** One-line result, then only the details that change a decision. No recaps of already-known context, no "what landed / what I verified" sections unless asked. Still report failures and skipped work plainly — brevity is not omission.

## Commit / Merge Skills

`.claude/skills/` holds the commit-and-merge workflow, plus `lib/` for what
more than one skill needs. **`./start.sh` is how a session begins** — it
sources `.env` (which is gitignored) so `CODERABBIT_API_KEY` and, when set,
`CLICKUP_API_TOKEN` reach the scripts; a key sourced mid-session does not.

- **`pr-merge` is ONE script** — `python3 ./.claude/skills/pr-merge/merge.py`,
  **no arguments and no flags**. The repository and the PR both come from the
  current checkout; an argument would be a second answer to a question the
  branch already answers, and the two can disagree. It refuses up front on
  `main`, on a detached HEAD, and on a branch whose PR is closed or absent.
  The SKILL.md invokes it and reports what it printed; it does not drive the
  steps.
  - **It replaced 2,248 lines across seven files in two languages**
    (`orchestrate.py`, `triage.py`, `render_review.py`, `threads.sh`,
    `merge.sh`, `postmortem.sh`, `run.sh`) with one Python file about half
    that size.
    The old design was resumable, four-phased, and wrapped every round in
    `except (Exception, SystemExit)` so the loop always reached a merge. It
    reached one on PR #76 after 7 hours and 13 commits, with eight recorded
    anomalies, a red preflight, and a `merge.sh` that failed afterwards. The
    complexity was the defect.
  - **Up to three rounds, then the merge.** 1-2 review and fix; **3 reviews
    and files tickets only**. Round 3 changing no code is load-bearing: the
    commit it reviewed has to be the commit it approves. That is now
    structural rather than guarded — `split_comments` empties the fix band
    on the last round, so the loop leaves before `commit()` is ever called,
    and `commit()` carries no round check of its own.
    `@coderabbitai approve` analyses *nothing* — it resolves every thread and
    approves, and is exempt from the rate limit precisely because it does no
    work. On PR #76 it approved `14e327a`, a commit no review was ever
    pinned to. So `merge()` **checks** rather than assumes: `reviewed_sha()`
    reads back the last CodeRabbit review carrying a non-empty body (the
    rubber stamp is an `APPROVED` with `body_len=0`; a real review is a 9 KB
    `COMMENTED`) and refuses unless its `commit_id` is the current head.
    A dirty tree is checked in the same place and for the same reason — as a
    merge precondition rather than a property of any round: the merge ends
    in `git checkout main`, which refuses on a dirty tree *after* the squash
    and the branch deletion — PR #76's `merge-err.txt` verbatim.
  - **Banded by consequence**: ≥9 fixed inline (rounds 1-2 only), 6-8 →
    ClickUp tagged `fix-now`, ≤5 recorded to `tmp/merge/round-N/ignored.json`
    and dropped. Round 3 files everything ≥6. A **body-only finding is never
    fixed inline** whatever it scores — it has no thread and no diff
    position, so the fix could not be reported back. One model call per round
    scores every finding at once; a finding the model returns no verdict for
    is a stop, never a default.
  - **It stops instead of improvising, and stopping means leaving state
    alone.** A fix that fails or leaves the gates red ends the run with
    **nothing reverted and nothing committed**; the message lists what the
    worktree now holds. A fix that could not converge is the one case where
    a person has to look, and what they need is what the agent actually did
    — half a fix that is nearly right beats a clean tree and no evidence,
    and reverting would also discard every fix that already succeeded that
    round. Same principle as `check_pushed` refusing to push rather than
    pushing for you: this script never mutates its way out of a problem.
    Also stops: a dirty worktree when the merge is reached (the merge ends
    in `git checkout main`, which refuses on one *after* the squash and the
    branch deletion), red gates at commit time although every fix
    reported green, an unparseable scoring answer, a review that is accepted
    and never posted, and a ticket-filing failure (a thread cannot be
    answered with a destination that does not exist).
  - **`Review rate limited` is the one thing that waits, not fails.** The bot
    answers a review request with either `✅ Action performed` /
    `Full review triggered.` or `⚠️ Action not completed` /
    `Review rate limited.` Nothing in the predecessor read that answer, so
    PR #76 requested four reviews in 25 minutes, was refused each time, and
    then waited 900s for a review that was never coming. The script now reads
    it, sleeps, and asks again for as long as it takes — for the number of
    minutes the bot itself names (`Your next included review will be
    available in 18 minutes.`), falling back to 15 when it names none.
    **The acknowledgement is edited IN PLACE, so it must be re-read while
    waiting.** Measured on PR #77: comment `5330633865` was posted at
    15:48:15 saying the review was triggered and rewritten at 15:48:47 to
    say the quota was spent. Reading it once caught the optimistic version
    and then blocked the full 900s for a review that was never coming, which
    is why `await_review` re-reads that comment on every poll and treats a
    late edit as the rate limit it is.
  - **A round that fixed nothing ends the loop**, which is capped at three.
    `if not to_fix: break` — one reason, stated in `main()`. Measured need:
    on PR #77 all three rounds bought a full review of the same commit
    `089c2af` and found nothing to fix in any of them, a quarter of the
    hourly quota each.
  - **`fix()` is bracketed by two assertions**, which is what lets the loop
    condition be a single test. Going in, the worktree must be clean —
    otherwise a path dirty afterwards cannot be told from a fix. Coming out,
    every path the agents reported in `files_changed` must be one git
    reports as changed, and a fix that named none is a fix that changed
    nothing. Both are stops. Asking a fixer to fix something and getting no
    edit is not an outcome to absorb: `resolve_conversations` answers the
    thread `**Fixed on this branch.**` moments later, which would be false,
    and the merge would proceed on it. Triage scored the finding 9-10 —
    *wrong behaviour reachable by a real invocation* — and the fixer says
    otherwise; a scorer and a fixer that disagree is something a person
    settles. The fix prompt says so too, so the agent knows a no-op ends the
    run and does not make a token edit to dodge it. Neither `commit()` nor
    `main()` carries a "nothing changed" path any more — that path is where
    the reason a run stopped early went to hide, decided inside a function
    whose name says it commits and reported only as a return value.
    Deliberately NOT a per-fix before/after comparison: `git status` names
    files rather than contents, so two fixes touching one file print the
    same listing and the second reads as a no-op.
    No resume and no `--only` phase. `main()` is the loop and nothing else —
    request, read, triage, split, fix/file/ignore in parallel, resolve,
    commit — and everything a round also has to *arrange* (the push, the
    banner, the score file, the band table) lives inside the function it
    belongs to. `REPO` and `PR` are module-level because `start()` sets them
    once; the round number is passed, because a value that changes every
    iteration is one a reader has to trace.
  - **Every thread is answered and resolved at the end of every round**,
    including a final sweep of anything still open — a human's thread, or one
    whose finding deduped away. `main` requires conversation resolution, so a
    single open thread blocks the merge, and leaving it to the end is how
    PR #76 got there.
  - **A reconciliation gap warns; it never blocks.** CodeRabbit over-counts:
    on PR #76 it claimed 43 actionable comments and posted 42. Treating the
    gap as fatal made the guard unsatisfiable and stalled that merge
    permanently. A gap can mean a missed finding OR a bot over-count; both
    are printed, neither stops.
  - **No round requests a review of a commit origin does not have, and it
    does not make that true either.** `check_pushed` runs first and only
    ever answers, in two questions: is the tree dirty, and is local HEAD the
    PR's head SHA. It will not commit or push on the caller's behalf —
    publishing work nobody decided to publish is not something a merge
    command gets to do.
    **Neither question goes through `@{u}`, and that is load-bearing.** A
    tracking ref describes local config, not what origin holds:
    `pr-commit/commit.sh` pushes with `git push origin HEAD` and no `-u`, so
    every branch it creates is fully pushed with no upstream at all — and an
    `@{u}` check refused exactly those, telling the caller to "push it
    yourself" about a commit origin already had. `git log @{u}..HEAD` is no
    better, comparing against a local ref that only moves on fetch or push.
    Both were inherited from the version that PUSHED, where they chose
    between `git push -u` and `git push`; once the pushing went, they
    answered nothing. `head_sha()` asks GitHub what the PR is built on,
    which is the thing that has to be true.
  - **The post-mortem reads the session history rather than the PR.** It
    takes the current file named by `docs/history/hook-state` (tens of MB, so
    never fed whole to a model), keeps the turns this script made — they are
    addressable by heading, because `CLAUDE_HISTORY_ROLE` labels them
    `## claude to @merge-fix`, `@merge-triage`, `@clickup` — plus any turn
    inside the run window, caps the extract at 300 KB, and asks one
    **read-only** turn for improvements to `merge.py`, its `SKILL.md` and
    `lib/clickup.py`. The proposals are filed under `merge-improvements`, and
    it then asserts the worktree is clean: the post-mortem reads, it never
    edits.
  - `fix-queue` is the other half — it works the `fix-now` tickets back into
    merged changes, one ticket per PR, each merge on its own command.
  - **`lib/clickup.py` is deliberately not folded in.** `fix-queue/SKILL.md`
    invokes it by path, and its four-heading ticket shape (`### Why` /
    `### What to change` / `### Verification` / `### Context`) is that
    skill's contract. It is the only other file the skill reaches, and it is
    Python — the skill is one language, and `pr-commit/gates.sh` is the sole
    non-Python thing it runs.
- **`.coderabbit.yaml` — `auto_review` is OFF, deliberately.** With it on,
  every push spends a review, so an eight-commit branch exhausts the hourly
  quota before anyone asks for a verdict; that is exactly how PR #76 ran
  out. With it off, pushes are free and each of `merge.py`'s three rounds
  buys one `@coderabbitai full review` when the branch is ready. It also makes that
  command *correct* — the docs scope it to paused auto-review. Consequence
  to know: the required `CodeRabbit` status check does not exist until a
  review is requested, so the first round is what brings it into existence.
  `request_changes_workflow: true` is what makes `@coderabbitai approve`
  work and the bot flip its own verdict to Approve once threads resolve.
  `path_filters` keep cassettes, generated tests and `doc-universe.html` out
  of review entirely.

- **`main` is protected by the "Main Protection" ruleset** (id `20972312`),
  modelled on the one in `speedandfunction/website`. Classic branch
  protection is **deleted** — the two would otherwise stack, and
  `/branches/main/protection` now 404s, which does not mean unprotected.
  Read the live rules with `gh api repos/ondatra-ai/true-bdd/rules/branches/main`.
  It requires two green checks (`gates`, `CodeRabbit`), one approving
  review, every review thread resolved, squash-only merges, linear history,
  and forbids deletion and force-push. Its code-scanning and Copilot rules
  were dropped in the adaptation: this repo runs neither, and requiring a
  tool that never reports blocks every PR forever.
  - **The admin role bypasses all of it on a PR merge** (`bypass_actors`,
    `pull_request` scope), so a merge succeeding proves nothing about the
    preconditions. What proves them is that `merge.py` resolves every thread
    at the end of every round and only escalates to `--admin` after the
    plain merge is refused, printing the refusal it escalated past.
  - `dismiss_stale_reviews_on_push` and `require_last_push_approval` are
    **on**, so every push voids the approval: get the re-review *after* the
    last commit. Only `killev` has write access and GitHub forbids
    self-approval, so the approval comes from CodeRabbit's `APPROVED`
    review — the bypass is what keeps that from being a deadlock when the
    bot does not deliver.
  - `require_code_owner_review` is on but inert: there is no `CODEOWNERS`
    file, so no path has an owner. Adding one makes the rule bite.
- **`.claude/skills/lib/diff-context.sh`** — sourced by `pr-commit/commit.sh`
  and `pr-update/pr-update.sh`. `emit_diff_context <git-diff-argv>` always
  sends the **complete** `--stat`, then the diff body in one of three
  shapes: whole when it fits `DIFF_BUDGET_BYTES` (200 KB); whole-but-filtered
  when dropping recorded cassettes and `docs/doc-universe.html` brings it
  under (the usual shape after a re-record); truncated to the cap otherwise.
  Which one it is, is stated in the piped text — a model told it is reading a
  prefix leans on the stat, and one that is not told describes the prefix as
  the whole change. `run_claude_or_explain` names a `claude -p` failure on
  stderr — a 3.4 MB diff used to exit 1 with nothing on the terminal.
- **`merge.py`'s `read_comments` is the part that is not simple, and it
  earns it.** CodeRabbit posts findings in two places and only one is a
  thread: "Actionable comments" become review threads, while "Nitpick /
  Outside diff range / Duplicate comments" live **only inside the review
  body** with no id, no reply target and nothing to resolve. Querying
  `reviewThreads` alone saw 16 of 28 findings on PR #70. It extracts both
  classes with a nesting-aware `<details>` scanner, strips only the named
  noise blocks rather than truncating at the first `<details>` (CodeRabbit
  routinely opens a comment with its "Analysis chain" transcript and puts
  the finding *after* it), and refuses outright when the thread page or a
  thread's comment page is truncated — a truncated page turns every count
  into a floor while still reading as a total. Verified against the live
  API: 16 + 12 on PR #70 and 42 + 19 on PR #76, matching both PRs' own
  stated counts.
- **A bot acknowledgement is not a bot verdict.** `request_review` waits for
  the *comment* that says whether the request was accepted, then separately
  for the review *object*. On PR #70 CodeRabbit answered "I will perform a
  fresh review" and never posted one, and nothing bounded the wait — the
  session sat for 4h37m. Both waits are bounded now, and the second timing
  out is a stop, never a dismissal.
- **The merge escalates to `--admin` only after the ordinary `gh pr merge`
  is refused**, so the log shows plainly when the ruleset bypass was used.
  Before merging it re-checks that the PR's `headRefName` is still the
  checked-out branch: not a re-check of an argument (there is none) but of
  the checkout having MOVED mid-run, since a fix agent holds `Bash(git *)`
  and squashing the wrong branch then deleting it is not reversible.

## Notes

- **Temporary files go to `./tmp/`** (the repo's gitignored runtime dir) — plan files, scratch scripts, intermediate outputs, anything session-temporary. Do not use system temp dirs or session scratchpads for repo work.
- Environment variables should be stored in .env files (excluded from git)
- Invoke the Vercel CLI via `npx vercel` (no global install)
- Never update `.golangci.yaml` without my permission
- **CRITICAL**: NEVER merge pull requests without explicit user command to merge
- **CRITICAL**: NEVER use `git commit --amend` or `git push --force`/`--force-with-lease`. Always create new commits.
