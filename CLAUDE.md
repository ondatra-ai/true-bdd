<!-- KARPATHY:BEGIN — verbatim upstream mirror, do not edit -->
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
<!-- KARPATHY:END — enforced by scripts/cmd/lint claude-md -->

## Repository Overview

**TrueBDD** (binary: `true-bdd`) — a Spec-Anchored CLI (aspiring to
Spec-as-Source) driving Claude-mediated checklists over user stories.
`README.md` carries the vision, the SDD taxonomy, and the config reference.

**This repo is the engine, not a host project.** A host supplies its own
`true-bdd/` config and the five `docs/` documents (contract: README →
Configuration); the root `true-bdd/` here is the fixtures' seed.

**Go, and one shell script.** Engine and tooling are Go (`scripts/`, via
`go run`); `go test` everywhere, `playwright-go`, no jest. `yamale`,
`alint` and `markdownlint-cli2` are PATH tools. The engine still *supports*
both as host `framework:` values (`build-code-playwright-nextjs` covers it).
`.alint.yml` fences JS to `services/bdd-web/`, the report UI's `web/`,
fixtures, `tests/legacy/` — and `no-shell` allows only `start.sh`.

## Scoped context

`.claude/rules/` holds path-scoped rules that load when Claude reads a
matching file. Working in one of these areas before opening a file? Read
the rule directly:

- `bdd-harness.md` — `tests/**`, `docs/scenarios.yaml`
- `engine-internals.md` — `services/bdd-cli/**`, `true-bdd/**`,
  `templates/**`: command depth, startup refusals, model tiers, crush
- `go-conventions.md` — `**/*.go`
- `markdown.md` — `**/*.md`

## CLI Subcommands

`us create/refine/apply <id>`, `build tests`, `build code`,
`scen check [id...]` — see `README.md` and `--help`.

**`--fix` refuses to start when any walked prompt lacks an `F:` template**
(`runner.validateFixTemplates`). Consequence: `us create --fix` and
`us refine --fix` refuse against the shipped checklists today (E2E-029
pins the refusal); the `us-refine-fix-*` fixtures pass only because
`checklist-prompts.yaml` narrows the walk to F-carrying prompts, and
`scen check --fix` refuses always (E2E-293) — `scen-check` ships no
`F:` at all.

## Development Commands

```bash
go run ./scripts/cmd/lint [FILE...]   # every gate; a hook runs it per edit
mkdir -p ./bin && go build -o ./bin/true-bdd ./services/bdd-cli
go test ./... && golangci-lint run   # unit only; BDD tree is -tags bdd
go run ./tests/libraries/cmd/report-server    # report UI on :7331
# BDD suites. `-mode` MUST come after the package path.
go test -tags bdd ./tests/bdd-cli/ -mode=replay       # hermetic, <1 min
go test -tags bdd -timeout=180m ./tests/bdd-cli/...   # live, ~3-5 min ea
go test -tags bdd -timeout=25m ./tests/bdd-web/       # needs node+browser
go test -tags bdd -run '^TestE2E016$' ./tests/bdd-cli/ -mode=record
go test -tags bdd ./tests/bdd-cli/ \
  -run '^Test(ScenarioCoverage|FixtureTreesArePaired|StepCoverage)$'
```

Run the CLI from inside a session as
`env -u CLAUDECODE ./bin/true-bdd us create 4.1` — the child needs a clean
env. `us refine` drives many sequential Claude calls, ~5 min end-to-end:
wait or poll, never kill it early.

## BDD Harness

`docs/scenarios.yaml` is the spec — no feature files. Every
`// Code generated` test under `tests/*/` is written by
`true-bdd build tests --fix` and nothing else, as are the
`tests/*/steps/` definitions; never hand-edit either. A step no
definition binds FAILS. Replay runs in both gates: exit 86 means a stale
cassette (re-record), a golden mismatch is the regression signal. Never
hand-edit a cassette. Full contract: `.claude/rules/bdd-harness.md`.

## Project Structure

Four roots, gated by `.alint.yml`: `services/<name>/` + `tests/<name>/`
mirror what TrueBDD asks of a host (plus `tests/libraries/`); `scripts/` is
this repo's tooling; `pkg/` is the four IO channels. `ls` for the tree.

- **Never import `os/exec`** — spawn via `pkg/cli/<tool>` (ADR 0005).
- `services/bdd-web/src/` is GENERATED and gitignored, so a listing does
  not show it: the bdd-web scenarios and suite are the spec.
- Sentinel `go.mod`s fence root `go test`/lint out of `services/bdd-web/`,
  `tests/legacy/…` and `tests/bdd-cli/fixtures/`; each states its trap.
- `tests/legacy/bdd-web-playwright/` exists to be DELETED per spec family,
  in the same commit binding that family's steps. Never add to it.
- `docs/*.html`: merging to `main` IS the deploy — never publish one as a
  Claude artifact; a new page needs a `cp` line AND a `paths:` trigger in
  `deploy-pages.yml`.
- `CONTEXT.md` is the glossary: take domain words from it, write settled
  ones back. A decision that is hard to reverse gets an ADR in `docs/adr/`.

## Architecture Principles

**When an AI turn must happen, spend freely on it** — multi-stage
generation, critique passes, full embedded context; never trim a prompt
to save tokens. This governs the *engine's runtime* model spend, not the
volume of code you write: Simplicity First still applies there.

## Task Management

Tasks live in ClickUp: <https://app.clickup.com/90151491867/v/l/li/901523097822>
**"Defer this" means `clickup defer`** — not MCP `createTask`, not the session
todo list. It and `clickup file` are the only creation paths, both stamping
`backlog` (`to do` is what `task-loop` works), headings from `ticket.yaml`,
scores from the ONE rubric `scripts/triage.Score` — floor 6, vs HEAD. `clickup
triage <N>` re-scores the N stalest and stamps them.

## Response Style

Answer first; no "what landed / what I verified" recaps unless asked.
Brevity is not omission — report failures and skipped work plainly.
Cite code repo-relative (`scripts/lint/hook.go:50`), never a basename.

## Commit / Merge Skills

- **`./start.sh` starts a session** — it exports `.env` before launching
  `claude`; a key sourced mid-session never reaches the commands skills run.
- Commit → `pr-commit`; merge → `pr-merge`; one Ticket → `task-handle` (Go,
  imports both); the whole queue → `task-loop`; a status is *moved* only by
  `task-start`, `clickup close` or `clickup triage`. merge: PRs #70/#76/#77.
- 24 skills vendored from `mattpocock/skills` (manifest:
  `.claude/skills/VENDORED-mattpocock.md`); its `code-review` shadows
  Claude Code's built-in skill of that name.
- **CodeRabbit reviews `tests/` only, and never automatically**
  (`.coderabbit.yaml` carries every reason). Two gotchas the YAML cannot
  state: the required `CodeRabbit` check does not exist until the first
  review is requested (absent, not red), and an empty body is not a pass.
- **`main` is guarded by ruleset `20972312`** — live via
  `gh api repos/ondatra-ai/true-bdd/rules/branches/main`, provenance in
  `docs/for_further/github-main-ruleset.md`. Classic protection is gone, so
  `/branches/main/protection` 404s, not "unprotected". Admin holds a merge
  bypass, so a merge proves nothing; every push dismisses approval.

## Notes

- CLAUDE.md: ≤214 lines, ≤80 cols, and the `KARPATHY` block byte-verbatim.
- Never `cd` — it matches no `Bash(...)` allow rule, so it prompts; use
  absolute paths or `-C <path>` from the repo root.
- Session-temporary files go to `./tmp/` (gitignored), never system temp
  dirs or scratchpads. Never edit `.golangci.yaml` without permission.
- **CRITICAL**: NEVER create a branch — only `scripts/commit` cuts one.
- **CRITICAL**: NEVER merge a pull request without an explicit command.
- **CRITICAL**: NEVER `git commit --amend`, `git push --force`, or
  `git push --force-with-lease` — always create new commits.
