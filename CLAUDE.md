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

This repo is the **engine**: Go source (`src/`), prompt templates (`templates/`), the engine config seed (`true-bdd/`), and the BDD fixture test harness (`tests/bdd-cli/`). A *host project* consuming the engine supplies its own `true-bdd/` configuration directory (`true-bdd.yaml`, `checklists/`, schemas) plus the five project documents under `docs/`: `docs/architecture/architecture.yaml` (architectural spec + BDD vocabulary), `docs/prd/prd.yaml` (PRD incl. personas), `docs/prd/epics/*.yaml`, `docs/prd/stories/*.yaml`, and `docs/scenarios.yaml` (the scenario registry). The engine repo's own root `true-bdd/` is a harness seed — canonical config and checklists that the BDD runner pre-copies into fixture tmpdirs, not a complete host configuration (fixtures supply their `docs/` tree under `input/docs/`, and may override any seed file via `input/true-bdd/`).

## CLI Subcommands

Commands are organized into two supergroups: `us` (story workflow) and `build` (build pipeline).

`us` supergroup:

- `us create <id>` — extract a story from its epic and run the `us-create` checklist.
- `us refine <id>` — load a story from `docs/prd/stories/` and run the `us-refine` checklist.
- `us apply <id>` — walk every AC in a refined story, validate against `us-apply`, and merge scenarios into the central `docs/scenarios.yaml` registry.

`build` supergroup — Spec-as-Source regeneration steps:

- `build tests` — walks every scenario in the requirements registry against the `build-tests` checklist and exits non-zero if any scenario lacks an executable test. With `--fix`, failed cells drive a Claude-mediated test-authoring loop that writes the missing test referencing the scenario id; the registry is never modified by the run. Takes `--requirements <path>` to override the registry location.
- `build code` — walks every `(service, layer)` pair declared in the architectural spec (`architecture.yaml`, override with `--architecture`), discovers currently-failing tests via each framework's runner (go test / jest / playwright), and walks each failure against the `build-code` checklist, exiting non-zero if any test still fails. With `--fix`, each failed cell drives a Claude-mediated turn that edits production source until the failing test passes; test files and the registry are never modified by the run.

Every command accepts `--fix` for an interactive loop in which Claude proposes edits for each failed check and the user applies, refines, or exits.

Checklist resolution is by convention via `paths.checklists_dir` in the host's `true-bdd.yaml`: the loader hyphenates the full command path (`us apply` → `us-apply.yaml`, `build tests` → `build-tests.yaml`).

## Development Commands

```bash
# Build (requires Go 1.25 and the `claude` CLI on $PATH)
mkdir -p ./bin && go build -o ./bin/true-bdd ./src

# Unit tests
go test ./...

# End-to-end BDD fixtures — real Claude calls, ~3-5 min per fixture
go test -tags bdd ./tests/bdd-cli/...

# Lint
golangci-lint run
```

### Running the CLI

The tool spawns `claude` as a subprocess. When invoking it from inside a Claude Code session, unset `CLAUDECODE` first so the child has a clean environment:

```bash
env -u CLAUDECODE ./bin/true-bdd us create 4.1
```

`us refine` drives many sequential Claude calls and typically takes **about 5 minutes** end-to-end. Do not abort early; poll or wait rather than killing the process.

## BDD Fixture Harness

Fixtures live under `tests/bdd-cli/fixtures/<scenario>/`. Each fixture is a folder containing exactly two things: `fixture.yaml` (the manifest) and the input directory it references (conventionally `input/`, designed test content).

`fixture.yaml` declares what to run and what to assert:

```yaml
# Required. Single-line invocation passed verbatim as arguments to true-bdd.
cmd: us apply 99.3 --fix

# Required. Path (relative to the fixture's own dir) of the directory
# tree overlaid onto the runner's tmpdir. Conventionally "input".
input: input

# Optional. Bytes piped to the subprocess's stdin (one line per prompt
# for the `--fix` interactive loop). Absent means no stdin is piped.
answers: |
  1
  1

# Required. Assertion strategies applied after the CLI exits.
expected:
  exit_code: 0          # Optional. Defaults to 0.
  stdout_regex:         # Optional. Go regexp patterns asserted against stdout.
    - "ALL CHECKS PASSED!"
  judge: |              # Required. Markdown rubric handed to the Claude judge.
    # Expectations
    ...
```

The runner builds each run's tmpdir in two layers: first it pre-populates the repo-layer engine ingredients (the tracked `true-bdd/` config seed and `templates/`) so fixtures exercise the live prompt templates; then it overlays the fixture's input tree on top, which holds the designed host-project content — `docs/` at minimum (synthetic prd, architecture, epic, story, seeded requirements registry), plus, when the scenario needs them, project sources and tests, a per-fixture `CLAUDE.md`/`.claude/`, or engine-config overrides under `true-bdd/`. Files inside the input tree win over the pre-populated layer, so a fixture may deliberately ship a per-fixture variant of a checklist or config.

The runner snapshots the tmpdir after prep but before the run, so the diff fed to the judge only contains files the run itself created or modified. After the CLI exits, the runner asks Claude (via the `src/claudecode/` wrapper) to compare the diff against the `judge:` rubric and return PASS / FAIL.

Tests are gated by a `//go:build bdd` tag so they're invisible to default `go test ./...`. The whole suite skips if the `claude` CLI is not on `$PATH`.

If the fixture's `cmd` invokes `--fix` (the interactive fix loop), set `answers:` to a literal block scalar. Its contents are piped verbatim to the subprocess's stdin. Each line answers one prompt: `1`/`2`/`3` (or `apply`/`refine`/`exit`) for the choice prompt; a single line for clarifying-question answers; multi-line free text terminated by a blank line for refinement feedback. Surplus lines are harmless (EOF on stdin causes the CLI to exit cleanly).

If the fixture's `cmd` needs the tmpdir to have project dependencies installed (e.g. `build code` shells out to `npx playwright test`), declare a `prep:` list. Each entry is a shell command executed via `bash -c` in the tmpdir, run after the input overlay and before the pre-run snapshot — so the side effects of `npm install`, `playwright install`, etc., do not pollute the diff handed to the judge. A non-zero exit aborts the fixture.

If the fixture's `cmd` spawns long-lived external resources that outlive the CLI process (e.g. Playwright's `webServer` brings up a `docker compose` stack), declare a `teardown:` list. Each entry runs via `bash -c` in the tmpdir AFTER the post-run snapshot and AFTER the CLI exits — regardless of success, failure, or timeout — against a fresh 2-minute context. Failures are logged but never mask the primary verdict; teardown is best-effort hygiene.

## Project Structure

- `src/` — the Go module (`github.com/ondatra-ai/true-bdd`). Entry point `src/main.go`; builds to `./bin/true-bdd` (gitignored).
  - `src/cmd/` — cobra command tree (`root.go`, `us.go`, `build.go`).
  - `src/claudecode/` — Claude Code subprocess SDK wrapper (client, transport, message parsing).
  - `src/adapters/ai/` — Claude client adapter and execution modes.
  - `src/internal/app/` — bootstrap container, command implementations, the checklist engine.
  - `src/internal/domain/` — story/checklist/registry/architecture models and ports.
  - `src/internal/infrastructure/` — loaders (config, epic, story, checklist, registry, architecture), template rendering, test runners (go test / jest / playwright), fs, console input.
  - `src/internal/pkg/` — `console` (terminal UI output), `errors`.
- `templates/` — prompt templates (Go `text/template` with sprig), named `<command>.<role>.prompt.tpl`.
- `true-bdd/` — the engine's canonical config seed (`true-bdd.yaml`, `checklists/`); pre-copied together with `templates/` into every BDD fixture tmpdir as the repo layer.
- `tests/` — all end-to-end / BDD tests live here (unit tests stay with their code, e.g. `harness/tests/unit/`):
  - `tests/bdd-cli/` — the Go BDD-CLI fixture harness: `bdd_test.go`, `runner/`, `coverage/`, `fixtures/<scenario>/`.
  - `tests/harness/` — the web-harness Playwright E2E suite, a **self-contained npm package** (own `package.json` + `node_modules`, sentinel `go.mod` to keep Go tooling out of its deps): specs (`p*` protocol, `a*` AI), `helpers/`, `fixtures/`, `reporters/`, `playwright.config.ts`, global setup/teardown. Each test launches its own harness container via `docker compose` (see `helpers/server-controller.ts`). Run: `cd tests/harness && npx playwright test --project=protocol`.
  - `tests/materializer/` — the Go fixture materializer (shared with `tests/bdd-cli/runner`), built by the E2E suite to overlay fixtures.
- `tmp/` — runtime working dir for prompt/response artifacts (gitignored).
- `docs/history/` — conversation history captured by the `.claude/hooks/history.py` hook (`<UTC-ts>-<session8>-<slug>.md`), gitignored. `docs/history/hook-state` holds a single line — the current file's name — shared across sessions so a new session continues the same file. `/new-task` (`.claude/commands/new-task.sh`) deletes it so the next prompt opens a fresh file, and also resets the repo to a clean state: local changes discarded, untracked files removed (ignored files kept), main checked out and fast-forwarded to origin — except `docs/context/`, whose uncommitted archivist writes always survive the reset. `docs/history/context-processed/` holds the context archivist's done-markers and offsets (see Context below).
- `docs/context/` — the requirements tree (git-tracked): a single `requirements.md` with three flat sections — `# Harness` (web-harness), `# System` (system design), `# Product` (user experience) — each a list of `## <requirement>` headings. Maintained by the context archivist (see Context below) via add/update/delete operations — the durable memory for what is said in conversation but never lands in a commit. **Not** the BDD requirements registry: product scenarios live in a host project's `docs/scenarios.yaml` (or a fixture's input tree). `terms.md` lists the only allowed subject terms (Harness / Systems / Roles) that a requirement may be phrased around.
- `docs/tasks/` — one Markdown task brief per task (`<slug>.md`, slug derived from the goal), written by the `identify-task` skill (Goal + Requirements) and consumed by `implement-task`.
- `tmp/test_run/<YYYY-MM-DD_HH-MM-SS>/<fixture-name>/` — per-fixture working dir created by the BDD test harness. Predictable, never auto-cleaned; wipe manually when you want to reclaim disk.

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

**Location**: `src/internal/pkg/console/console.go`

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

## Context

Durable conversational requirements — observable `should`/`must` statements that never land in commits — live in `docs/context/requirements.md`, a living tree (`# System` over `# Product → ## Feature → ### Requirement`) extracted from task transcripts by the context archivist. Check it before working in an area it covers.

**Context archivist:** `.claude/hooks/context.py sweep` maintains `docs/context/requirements.md` — after EVERY response, not just at task boundaries. The Stop hook chains it behind `history.py` (backgrounded, so the turn never waits): each sweep finalizes any finished history files, then incrementally processes the ACTIVE file's newest chunk — a byte offset in `docs/history/context-processed/<file>.offset` tracks what's been distilled, and growth under ~300 bytes is skipped; `/new-task` triggers the finalizing sweep after rolling the state file over. Per chunk, ONE `codex exec -s read-only --ephemeral` call (schema-forced JSON via `--output-schema` + `-o`; prompt in `.claude/hooks/context-prompt.md` — the no-code tuning knob) returns a list of operations against the requirements tree: each is an `add` / `update` / `delete` of a requirement, classified as `harness`, `system`, or `product` (landing under the matching `# Harness`, `# System`, or `# Product` section), that passes three admission tests (a future session would act differently; not recoverable from code, git history, CLAUDE.md, or requirements.md; human-confirmed or empirically verified) — empty operations is the expected result for routine turns. The script applies the operations in place: a `match` substring uniquely identifies an existing requirement to `update`/`delete` (ambiguous/missing matches are skipped and logged); `add` appends under the right section heading. It writes the raw reply as the done-marker to `docs/history/context-processed/<file>.json` — markers/offsets advance only on success, so failed codex runs retry next sweep; an flock on `docs/history/context-sweep.lock` prevents double-writes; sweeps no-op under `GITHUB_ACTIONS` / `CLAUDE_HISTORY_ROLE` so CI runs and headless `claude -p` workers never burn codex calls; log at `docs/history/context-sweep.log`. The archivist never edits CLAUDE.md.

## Notes

- **Temporary files go to `./tmp/`** (the repo's gitignored runtime dir) — plan files, scratch scripts, intermediate outputs, anything session-temporary. Do not use system temp dirs or session scratchpads for repo work.
- Environment variables should be stored in .env files (excluded from git)
- Invoke the Vercel CLI via `npx vercel` (no global install)
- Never update `.golangci.yaml` without my permission
- **CRITICAL**: NEVER merge pull requests without explicit user command to merge
- **CRITICAL**: NEVER use `git commit --amend` or `git push --force`/`--force-with-lease`. Always create new commits.
