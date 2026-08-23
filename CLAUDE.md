<!-- KARPATHY:BEGIN — verbatim upstream mirror, do not edit; enforced by scripts/check-karpathy-block.sh -->
# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
<!-- KARPATHY:END -->

## Repository Overview

**TrueBDD** (binary: `true-bdd`) — a Spec-Anchored CLI (aspiring to Spec-as-Source) that drives Claude-mediated checklists over user stories. `README.md` carries the vision, the three-levels-of-SDD taxonomy, and the full configuration reference.

**This repo is the engine, not a host project.** A host supplies its own `true-bdd/` config and the five `docs/` documents (contract: README → Configuration); the root `true-bdd/` here is the fixtures' seed, not this repo's host config (mechanics: Project Structure / BDD Harness).

**This repository tests in Go, and only in Go** — `go test` for every suite, `playwright-go` for the browser; no jest, no `@playwright/test`. The engine still *supports* `jest`/`playwright` as host `framework:` values (kept covered by the `build-code-playwright-nextjs` fixture); the parked `tests/legacy/bdd-web-playwright/` suite is the one exception and exists to be deleted.

## CLI Subcommands

Five commands — `us create/refine/apply <id>`, `build tests`, `build code` — documented in `README.md` (subcommand table, flags, checklist naming) and `--help`.

**Startup refusals have one shape.** Anything a command needs before its first AI turn is checked up front and reported through `runner.refuseStartup`: `Cannot start: …` on stdout PLUS a `Refusing to start` slog record — the fixture harness and the BDD judge read those, never cobra's stderr usage dump. An error that already printed its own diagnosis is wrapped in `runner.Reported` so it is not announced twice.

**`--fix` refuses to start when any walked prompt lacks an `F:` template** (`runner.validateFixTemplates`). Consequence: `us create --fix` and `us refine --fix` refuse against the shipped checklists today (E2E-029 pins the `us create` refusal); the `us-refine-fix-*` fixtures pass only because `checklist-prompts.yaml` narrows the walk to F-carrying prompts.

## Model Tiers and Providers

Tiers, roles, resolution order and both crush/codex gotchas are all in README.md § "Model tiers";
`true-bdd/true-bdd.yaml`'s own comments restate the config side. Repo rule with no other home:
**never add a tracked `.crush.json`/`crush.json` at the repo root** — crush merges every config it
finds walking UP from its cwd (nearest last; see `warnOnHostCrushConfig` in
`services/bdd-cli/adapters/ai/crush_provider.go`), so a root one silently blocks every fixture's apply turn.

## Development Commands

```bash
./scripts/validate-schemas.sh                       # schema gate (yamale); pairing rule in its header
./scripts/check-karpathy-block.sh                   # the fenced block at the top of this file still matches upstream
mkdir -p ./bin && go build -o ./bin/true-bdd ./services/bdd-cli
go test ./...                                       # unit only — the BDD tree is behind -tags bdd
go test -tags bdd -timeout=180m ./tests/bdd-cli/... # live e2e: real Claude, ~3-5 min per scenario
go test -tags bdd -timeout=25m ./tests/bdd-web/     # needs node+browser; FAILS when missing (-allow-missing-toolchain to skip)
go test -tags bdd ./tests/bdd-cli/ -mode=replay     # hermetic, <1 min, no model; -mode MUST come after the package path
go test -tags bdd -run '^TestE2E016$' ./tests/bdd-cli/ -mode=record   # re-record one scenario by its test name
go test -tags bdd -run '^Test(ScenarioCoverage|FixtureTreesArePaired|StepCoverage)$' ./tests/bdd-cli/  # the three guards, <1 s
go run ./tests/libraries/cmd/report-server          # run-report UI, http://127.0.0.1:7331
golangci-lint run
```

From inside a Claude Code session run the CLI as `env -u CLAUDECODE ./bin/true-bdd us create 4.1` — the child needs a clean env.
`us refine` drives many sequential Claude calls, ~5 min end-to-end: wait or poll, never kill it early.

## BDD Harness

`docs/scenarios.yaml` is the spec — no feature files. `build tests` renders one committed Go test per scenario, `tests/libraries/bddgo` runs it, and `Done()` stops the scenario if the file's steps drift from the registry. `true-bdd build tests --fix` is the ONLY writer of the `// Code generated` test files (deterministic codegen) and of the `tests/*/steps/` definitions — never hand-edit either. A step no definition binds FAILS; it is never skipped.

Steps no regexp can settle carry a prefix: `llm:` (a model acts — legal in given/when only) or `judge:` (a model rules — then only, one call after every other step passed). A prefix in the wrong block is a refusal, not a reinterpretation; semantics and rationale live in `tests/libraries/bddgo/model_steps.go`.

A fixture (`tests/bdd-cli/fixtures/<name>/`) is a directory, not a document: behaviour — invocation, exit code, stdout, stdin, judged clauses — comes from the scenario; the tree carries data (`input/`, `cassettes/`, optional root-level `prep.sh`/`teardown.sh`/`checklist-prompts.yaml`, documented in `tests/libraries/runner/runner.go`). A tree no scenario names fails the whole run, and a scenario without cassettes fails replay rather than skipping.

A run is graded at three levels, each owning what the others cannot say:
1. the scenario's Then steps — every mechanical assertion, in every mode;
2. replay only — the byte-for-byte golden, a request hash per call (mismatch = stale cassette, exit 86), and every cassette consumed (`runner.CheckCassettesConsumed`);
3. the judge — the `judge:` clauses, live/record only; replay spawns no model at all, and the golden comparison discharges the clauses more strictly than a reading.
Residual gap: a change touching only `tmp/` scratch (per-run timestamped paths) and neither stdout nor an output file is not caught.

Gotchas:
- A fixture input tree carrying `.go` files needs its own sentinel `go.mod`, and `tests/bdd-cli/fixtures/go.mod` closes the same trap for recordings — full rationale in that file's comment.
- A recording leak found by `scan-recordings.sh` means fix the shim's sanitization and re-record — never hand-edit a cassette.
- The guards in `tests/*/coverage_test.go` stay hand-written so the generator cannot silence them by regenerating.
- `build tests` on THIS repo is not yet zero-AI-turn: bdd-web's coverage command truthfully reports 243 scenarios with unbound steps (arithmetic in `gates.sh`).

## Project Structure

Layout mirrors what TrueBDD asks of a host: `services/<name>/` + `tests/<name>/`, plus
`tests/libraries/` (shared harness code). `ls` for the tree; package doc comments carry
the descriptions — the `tmp/test_run` artefact layout is in `tests/libraries/reporter/session.go`.

- `services/bdd-web/src/` is GENERATED and gitignored (invisible in a listing): the
  bdd-web scenarios and suite are the spec — see the `.gitignore` comment at the rule.
- Sentinel `go.mod`s in `services/bdd-web/`, `tests/legacy/bdd-web-playwright/` and
  `tests/bdd-cli/fixtures/` fence root `go test`/lint out; each states its trap.
- `tests/legacy/bdd-web-playwright/` exists to be DELETED per spec family, in the same
  commit that binds that family's step definitions. Never add to it.
- `docs/*.html`: merging to `main` IS the deploy — never publish a page as a Claude
  artifact; a new page needs a `cp` line AND a `paths:` trigger in `deploy-pages.yml`.

## Architecture Principles

- **When an AI turn must happen, spend freely on it** — multi-stage generation, critique passes, full embedded context; never trim a prompt to save tokens. This governs the *engine's runtime* model spend (where the file elsewhere eliminates turns deterministically, that wins); it is not a licence for more code — Simplicity First still applies to what you write.

## Go Conventions

`slog` for logs, `internal/pkg/console` for terminal UI (console's package doc states the same).
forbidigo forbids raw `fmt.Print*` and its message says "use slog for logging" — misleading when
the output is user-facing: the fix there is `console`, the one package excluded from the rule.

One primary entity per file, filename = entity in snake_case (`FixApplier` → `fix_applier.go`). "GitHub" is one word: `github_service.go`, never `git_hub_service.go`. Function-only and bundled-helper files (`ExecutionMode` + `ModeFactory`) are fine. Review-enforced; no linter rule.

## Shell Usage
Never `cd` — run from the repo root with absolute paths or `-C <path>`; a `cd`-prefixed command matches no `Bash(...)` allow rule in `.claude/settings.json`, so it triggers a permission prompt.

## Task Management

Tasks live in ClickUp, reached via the MCP tools: <https://app.clickup.com/90151491867/v/l/li/901523097822> (list id `901523097822`).
**"Defer this" means create a task there** — never the session todo list, which evaporates when the session ends.
Write it to be picked up cold, in `markdownContent`, with the four headings `.claude/skills/lib/clickup.py` renders: **Why / What to change (`file:line`) / Verification / Context**.

## Response Style

Answer first; no "what landed / what I verified" recaps unless asked. Brevity is not omission — failures and skipped work are still reported plainly.

## Commit / Merge Skills

- **`./start.sh` starts a session** — it exports `.env` before launching `claude`. A key sourced
  mid-session never reaches the skill scripts; restart instead.
- Commit → `pr-commit` skill; merge → `pr-merge` skill; deferred `fix-now` tickets → `fix-queue` skill.
  `merge.py`'s docstrings ARE the design record (measured failures on PRs #70/#76/#77) — read them before editing it.
- 24 skills vendored from `mattpocock/skills` — manifest: `.claude/skills/VENDORED-mattpocock.md`. Its
  `code-review` shadows Claude Code's built-in skill of the same name.
- **CodeRabbit reviews `tests/` only, and never automatically** (`.coderabbit.yaml`; every reason is in its
  comments). Gotcha the YAML cannot state: the required `CodeRabbit` status check does not exist until the
  first review is requested — on a fresh PR it is absent, not red.
- **`main` is guarded by ruleset `20972312`**, live-readable: `gh api repos/ondatra-ai/true-bdd/rules/branches/main`.
  Classic protection is deleted — `/branches/main/protection` 404s, which is not "unprotected". The admin role
  holds a `pull_request`-scoped merge bypass, so a merge succeeding proves nothing about the preconditions. Every
  push dismisses the approval (re-review after the last commit); provenance and the two dropped rules are in
  `docs/for_further/github-main-ruleset.md`.

## Notes

- The block at the top of this file between the `KARPATHY:BEGIN`/`END` markers is a verbatim upstream mirror. Never edit inside it; `scripts/check-karpathy-block.sh` fetches upstream and diffs.
- Session-temporary files (plans, scratch scripts, intermediate outputs) go to `./tmp/` (gitignored) — never system temp dirs or session scratchpads. Never edit `.golangci.yaml` without permission.
- **CRITICAL**: NEVER merge a pull request without an explicit user command.
- **CRITICAL**: NEVER `git commit --amend` or `git push --force`/`--force-with-lease` — always new commits.
