# implement-task paths

Single source of truth for every folder/file path used by the `implement-task`
skill (`.claude/skills/implement-task/SKILL.md`) and its four agents
(`.claude/agents/implement-task-*.md`). **Skills and agents read paths from this
file; do not hardcode paths in skill or agent instructions.** Fulfils the Harness
requirement in `docs/context/requirements.md`.

The only path literals permitted in the skill/agents are (1) this file —
`docs/context/paths.md`, the config entry point — and (2) the PreToolUse hook path
in the coder agent's frontmatter, which is static agent wiring and cannot be read
from here. Every other folder/file location is read from this file.

All paths are repo-relative (repo root = where this file's `docs/` lives).
`<slug>` is the task brief's filename stem, produced by `identify-task`.

## Inputs

- **Task brief**: `docs/tasks/<slug>.md` (from `identify-task`).
- **Requirements context** (read for goal/constraints): `docs/context/requirements.md`,
  `docs/context/requirements-guide.md`, `docs/context/terms.md`.
- **Project guidance**: `CLAUDE.md` (conventions, architecture principles).

## Plan

- **Plan file** — created by the planner; the orchestrator records implementation
  challenges here: `docs/tasks/plans/<slug>.md` (tracked alongside task briefs;
  create the folder if absent).
- **Codex rounds ledger** — `docs/tasks/plans/<slug>.codex.md`, written by each
  agent's Codex loop. Lives beside (not inside) the plan so downstream agents
  reading the plan don't pay for the round tables; the plan's "Codex rounds"
  section is a one-line pointer here.

## Phase state (enforcement)

- **Hook script**: `.claude/hooks/phase_state.py` (wired in `.claude/settings.json`).
  Records agent spawns/completions, denies out-of-order phases, blocks turn-end and
  commits while the reviewer is pending, and appends task metrics on close.
- **Active state**: `tmp/implement-task/active.json`; archives + audit log under
  `tmp/implement-task/`.
- **Metrics**: `docs/context/skill-metrics.jsonl` (one line per closed task).
- **Retro output**: `docs/context/retro/<slug>.md` (written by the `task-retro`
  skill; proposals only, never auto-applied).

## End-to-end tests

The e2e/BDD tests that **drive** the coding (red → green). Written and hardened by the
test-author; the coder may NOT touch them (see Off-limits). Only these gate the task.

- **E2E tests (Playwright)**: `tests/harness/` (self-contained suite: its own
  `package.json` + `node_modules`) — binding UI/API contract:
  `tests/harness/helpers/README-testids.md`.
- **BDD-CLI fixtures + coverage/materializer harness**: everything else under `tests/`
  (`tests/bdd-cli/`, `tests/materializer/`).

## Unit tests

The coder's responsibility (NOT the test-author's), written alongside the production
code they exercise. They are the coder's own quality net and never gate the task —
only the e2e/BDD suite does. Not off-limits to the coder:

- **TS unit (Vitest)**: `harness/tests/unit/` + its runner config `harness/vitest.config.ts`.
- **Go unit**: `*_test.go` beside the production package under `src/`.

## Architectural startup scaffolding

The test-author may create these so services start, but leaves them **empty** for
the coder to implement (no production logic):

- Repo-root `docker-compose*.yml` (incl. the e2e override `docker-compose.test.yml`),
  the harness image `harness/Dockerfile` (+ `harness/.dockerignore`), and new
  service directories.

## Production code

The coder and reviewer may edit these:

- `src/` (Go engine), `templates/` (prompt templates), `true-bdd/` (config seed),
  `harness/app/` (Next.js web harness: pages, components, lib, api routes).

## Change-surface exclusions

Directories omitted from the baseline **change-surface content copy** and the
Phase 3 reviewer's `diff -r` (build + VCS noise, never part of the reviewed
change) — the single source for this list so the skill does not hardcode it:

- `node_modules/`, `.git/`, `.next/`, and the Codex artifacts dir (`./tmp/`, see Codex).

## Package manifest

- `harness/package.json` — the coder may add runtime **dependencies** here, but NEVER
  its `scripts` (the test scripts). The orchestrator snapshots the `scripts` object
  before/after the coder and rejects any change to it.

## Off-limits to the coder

Hard-enforced by `.claude/hooks/block_test_edits.py`, which reads the list in the
fenced block below — the **only** place these patterns live (edit here, not in the
hook). Anything matched is denied even in `bypassPermissions`; the coder must
escalate to the test-author via the orchestrator instead of editing tests.

Scope is deliberately the **e2e/BDD** tree only (everything under `tests/`). The
unit-test trees (`harness/tests/unit/`, `harness/vitest.config.ts`, Go `*_test.go`
beside `src/`) are the coder's own territory (see Unit tests) and are NOT listed.

`harness/package.json` is deliberately **not** in this list (the coder may add
runtime deps): its `test:*` scripts stay off-limits by prompt + parent diff-review,
since a path hook cannot scope inside a single file.

```text
tests/
```

## Codex

- **Wrapper**: `.claude/skills/implement-task/scripts/codex.sh` (`<ro|auto> <prompt-file> [label]`).
- **Mechanics + prompt guide**: `.claude/skills/implement-task/references/codex.md`.
- **Loop procedure** (shared by all four agents): `.claude/skills/implement-task/references/codex-loop.md`.
- **Complexity matrix** (lane → agents + Codex round caps): `.claude/skills/implement-task/references/complexity-matrix.md`.
- **Artifacts**: answers at `./tmp/codex-<label>.md` (+ `.trace.log`); prompts at `./tmp/codex-<label>-rN.md`.

## Run commands

- **E2E (Playwright)**: `cd tests/harness && npx playwright test --project=protocol` (or `--project=ai` for the real-Claude suite).
- **E2E, specific specs** — preferred for tight iteration; pass the spec file(s) and Playwright routes to the project by filename (`p*`→protocol, `a*`→ai): `cd tests/harness && npx playwright test <file>.spec.ts`.
- **Unit (Vitest)**: `cd harness && npm run test:unit`.
- **Harness typecheck/lint**: `cd harness && npm run typecheck && npm run lint`.
- **Go unit**: `go test ./...`.
- **Go lint**: `golangci-lint run`.
