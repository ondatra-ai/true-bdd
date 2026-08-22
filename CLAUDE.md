# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository

`github.com/ondatra-ai/true-bdd` — owner `ondatra-ai`, GitHub user `@killev`.

**TrueBDD** (binary: `true-bdd`) — a Spec-Anchored CLI that drives Claude-mediated checklists over user stories. See `README.md` for the vision, the three-levels-of-SDD taxonomy, and configuration reference.

**This repository tests in Go, and only in Go** — `go-test` for every suite, with `playwright-go` driving the browser inside the web suite. The engine still *supports* `jest`/`playwright` as `framework:` values a host project may declare (the `build-code-playwright-nextjs` fixture keeps that path covered); this repo declares neither. The parked TypeScript suite at `tests/legacy/bdd-web-playwright/` is the one exception, and it exists to be deleted.

This repo is the **engine**: Go source (`services/bdd-cli/`), prompt templates (`templates/`), the engine config seed (`true-bdd/`), and the BDD harness (`tests/`). A *host project* supplies its own `true-bdd/` config dir plus five documents under `docs/`: `architecture/architecture.yaml`, `product/product.yaml` (roles and BDD vocabulary), `product/epics/*.yaml`, `product/stories/*.yaml`, `scenarios.yaml` (the scenario registry). Those are conventional defaults — every location is driven by `true-bdd.yaml` (`documents:` for paths, `paths:` for directories). The repo's own root `true-bdd/` is a harness seed pre-copied into fixture tmpdirs, not a complete host configuration.

## Scoped context

`.claude/rules/` holds path-scoped rules that load when a matching file is read. Working in one of those areas before any file is open? Read the rule file directly:

- `bdd-harness.md` (`tests/**`, `docs/scenarios.yaml`) — fixtures, cassettes, record/replay, grading, step conventions, go.mod sentinels, run dirs.
- `engine-internals.md` (`services/bdd-cli/**`, `true-bdd/**`, `templates/**`) — build tests/build code deep behaviour, startup refusals, model tiers, crush gotchas, package notes.
- `go-conventions.md` (`**/*.go`) — slog vs console, file naming.

## CLI Subcommands

Two supergroups. `us` (story workflow): `us create <id>` extracts a story from its epic and runs the `us-create` checklist; `us refine <id>` runs `us-refine` on a story; `us apply <id>` walks every AC and merges scenarios into the registry (`documents.scenarios_yaml`). `build` (Spec-as-Source regeneration): `build tests` renders every scenario's Go test then makes sure every step binds (a converged repo costs zero AI turns); `build code` walks every declared suite's failing tests against the `build-code` checklist. Neither ever modifies the registry; `build code` never modifies test files.

Every command accepts `--fix` (interactive loop: Claude proposes edits, the user applies/refines/exits). Checklists resolve by convention via `paths.checklists_dir`: hyphenate the command path (`us apply` → `us-apply.yaml`). Precondition failures are reported as `Cannot start: …` on stdout plus a `Refusing to start` slog record — the BDD judge reads the log, not the terminal.

## Development Commands

```bash
# Validate every document against its schema (requires yamale: pip install yamale).
# Runs in CI and gates.sh. Pairing convention and its traps: engine-internals rule.
./scripts/validate-schemas.sh

# Build (requires Go 1.25 and the `claude` CLI on $PATH)
mkdir -p ./bin && go build -o ./bin/true-bdd ./services/bdd-cli

# Unit tests
go test ./...

# End-to-end BDD scenarios — real Claude calls, ~3-5 min per scenario
go test -tags bdd -timeout=180m ./tests/bdd-cli/...

# Web suite: builds services/bdd-web, boots it on a free port, drives chromium
# via playwright-go. Out of every gate on purpose; FAILS (not skips) when
# node/npm are missing — -allow-missing-toolchain asks for the skip explicitly.
go test -tags bdd -timeout=25m ./tests/bdd-web/

# Hermetic record/replay (the -mode flag comes AFTER the package path).
# replay serves per-fixture cassettes/ and spawns NO model — not even the
# judge; a scenario without a recording FAILS. Whole suite replays in under
# a minute for zero cost. record publishes cassettes only if the fixture passes.
go test -tags bdd ./tests/bdd-cli/ -mode=replay
# One scenario, by its generated test's name (every re-recordable failure
# prints this exact line with the fixture named in a trailing comment):
go test -tags bdd -run '^TestE2E016$' ./tests/bdd-cli/ -mode=record

# The three guards, in under a second, no toolchain needed
go test -tags bdd -run '^Test(ScenarioCoverage|FixtureTreesArePaired|StepCoverage)$' ./tests/bdd-cli/

# Run report — every session, live (rescans tmp/test_run every 15s;
# -addr 0.0.0.0:7331 to reach it from another machine)
go run ./tests/libraries/cmd/report-server   # http://127.0.0.1:7331

# Lint
golangci-lint run
```

The CLI spawns `claude` as a subprocess — unset `CLAUDECODE` when invoking it from inside a Claude Code session: `env -u CLAUDECODE ./bin/true-bdd us create 4.1`. `us refine` drives many sequential Claude calls and takes **about 5 minutes**; poll or wait rather than killing the process.

**BDD in one breath**: `docs/scenarios.yaml` is the spec; every `// Code generated` test file under `tests/*/` is written by `true-bdd build tests --fix` and nothing else; replay runs in both gates (exit 86 = stale cassette → re-record; golden mismatch = the regression signal); never hand-edit a cassette. Full harness contract: `.claude/rules/bdd-harness.md`.

## Project Structure

Same `services/<name>/` + `tests/<name>/` layout TrueBDD asks of a host: one suite per service, `tests/libraries/` for shared code, `tests/legacy/` for what is on its way out.

- `services/bdd-cli/` — the Go module (`github.com/ondatra-ai/true-bdd`); builds to `./bin/true-bdd` (gitignored). `cmd/` (cobra tree), `claudecode/` (Claude Code subprocess SDK wrapper), `adapters/ai/`, `internal/app/` (bootstrap container, commands, checklist engine, scenario generator), `internal/domain/` (models and ports), `internal/infrastructure/` (loaders, template rendering, test runners, step coverage, fs, console input), `internal/pkg/` (`console`, `errors`).
- `services/bdd-web/` — Next.js relay + UI (a **sentinel nested Go module** so root Go tooling never descends into `node_modules`). Its `src/` is GENERATED and gitignored — the scenarios and suite are the spec. `design/` holds the design system, `SPEC.md`, `proto-workspace/`.
- `templates/` — prompt templates, `<command>.<role>.prompt.tpl` (Go text/template + sprig).
- `true-bdd/` — the engine's canonical config seed, pre-copied with `templates/` into every fixture tmpdir.
- `tests/bdd-cli/`, `tests/bdd-web/` — per-suite: generated `*_test.go`, hand-written `main_test.go` + `coverage_test.go` + `scenarios/` (the TestMain shim and state binding), `steps/`, and (bdd-cli only) `fixtures/`.
- `tests/libraries/` — `bddgo/` (registry-driven scenario runner, generic over suite state), `runner/` (fixture loading, tmpdir assembly, CLI invocation, per-run diff, judge, goldens, recorder), `aiproxy/`, `fstree/`, `reporter/` + `reportserver/`, `materializer/`, `cmd/{report-server,coverage}/`.
- `tests/legacy/bdd-web-playwright/` — the PARKED TypeScript suite; every spec is now a registry scenario (`E2E-048`…`E2E-290`) but the scenarios are not executable yet, so this suite is still the only thing testing the web surface. Delete per family, in the same commit that binds that family's steps. Do not add to it.
- `docs/for_further/` — agreed designs for unstarted work, one file per idea with its own decision log; deliberately outside the doc universe.
- **`docs/*.html` are published pages; merging a change to `main` IS its deploy** (`.github/workflows/deploy-pages.yml` is a `cp` per page; each page is self-contained). Never publish one as a Claude artifact instead. Adding a page needs BOTH a `cp` line and a `paths:` trigger — missing `cp` 404s, missing `paths:` silently never redeploys. `docs/roadmap.md` is `roadmap.html`'s Markdown twin; the ClickUp "Idea" whiteboard stays the source. Repo-local like `for_further/` — `sync-doc-universe` does not audit them.
- `docs/history/` — conversation history from `.claude/hooks/history.py` (gitignored); `hook-state` holds the current file's name, shared across sessions. `/new-task` deletes it and resets the repo to clean.
- `tmp/` — runtime working dir (gitignored). `tmp/test_run/` holds per-fixture run dirs (layout: bdd-harness rule); browse them with the report server.

## Architecture Principles

**Quality over cost — time, price, and token usage are lowest priority.** Always choose the approach with the best output: multi-stage generation, full articles embedded in prompts, multiple validation passes, self-critique loops are all acceptable. Never compromise quality for speed or token savings.

**Direct data flow — no caching, no loaders, no unnecessary interfaces.** `Epic File → StoryFactory → StoryDocument → Generators`: load once at the factory level into a complete domain structure, pass concrete types, extend domain models instead of creating loaders, avoid interfaces without a genuine second implementation, question every layer.

## Shell Usage

**Do not use `cd`.** Run commands from the repository root using absolute paths or `-C <path>` flags, so paths stay predictable and the working directory never drifts.

## Task Management

**ClickUp is the task manager**: <https://app.clickup.com/90151491867/v/l/li/901523097822> (list id `901523097822`), via the ClickUp MCP tools. **"Defer this" means: write it to that ClickUp list** — never the session todo list, which evaporates. Write each ticket so it can be picked up cold, using `markdownContent` with four parts: **Why** (the problem), **What to change** (concrete edits with `file:line`, grepped not remembered), **Verification** (commands that prove it, and what it could silently break), **Context** (why deferred). Deferring is a scope decision: it keeps unrelated work out of the change in hand.

## Response Style

Lead with the answer or result; drop restatement, framing sentences, and narration of what a tool did. Tables and short bullets over paragraphs. Findings and caveats stay — the prose around them goes. One-line result, then only the details that change a decision. Still report failures and skipped work plainly — brevity is not omission.

## Commit / Merge Workflow

`.claude/skills/` holds the commit-and-merge skills plus `lib/` for shared pieces. **`./start.sh` is how a session begins** — it sources `.env` (gitignored) so `CODERABBIT_API_KEY` and `CLICKUP_API_TOKEN` reach the scripts; a key sourced mid-session does not.

**24 skills are vendored from `mattpocock/skills` (MIT), project-scoped on purpose** — `.claude/skills/VENDORED-mattpocock.md` is the manifest: what was taken, what was stripped, how to re-sync, and the consequence that `code-review` shadows Claude Code's built-in skill of the same name.

- **`pr-merge`** = `python3 ./.claude/skills/pr-merge/merge.py`, no arguments — the repo and PR come from the current checkout. Up to three bounded CodeRabbit review rounds (rounds 1-2 fix ≥9 and file 6-8 as ClickUp tickets; round 3 files everything ≥6 and changes no code, so the reviewed commit is the approved commit), then merge. It stops rather than improvises, leaves state alone on failure, waits out CodeRabbit's rate limit, and never commits or pushes on the caller's behalf. **The design rationale lives in merge.py's own docstrings and comments — read those before changing it.**
- **`fix-queue`** works the `fix-now` tickets back into merged changes, one ticket per PR. `.claude/skills/lib/clickup.py` is its deliberately-separate ClickUp client; the four-heading ticket shape above is the contract between them.
- **`.coderabbit.yaml`**: `auto_review` is OFF deliberately — pushes are free, and each merge round buys one `@coderabbitai full review` when the branch is ready. Rationale in the file's own comments. The required `CodeRabbit` status check does not exist until the first review is requested. `path_filters` keep cassettes, generated tests and `doc-universe.html` out of review.
- **`main` is protected by the "Main Protection" ruleset** (id `20972312`); read the live rules with `gh api repos/ondatra-ai/true-bdd/rules/branches/main`. Classic branch protection is deleted — `/branches/main/protection` 404s, which does not mean unprotected. Gotchas the API won't volunteer: the admin role bypasses everything on PR merges, so a merge succeeding proves nothing about the preconditions; every push voids the approval (`dismiss_stale_reviews_on_push` + `require_last_push_approval`) — get the re-review *after* the last commit; only `killev` has write access and GitHub forbids self-approval, so the approval comes from CodeRabbit's review; `require_code_owner_review` is inert until a `CODEOWNERS` file exists — adding one makes it bite.

## Notes

- **Temporary files go to `./tmp/`** (the repo's gitignored runtime dir) — plan files, scratch scripts, intermediate outputs, anything session-temporary. Do not use system temp dirs or session scratchpads for repo work.
- **`.claude/plugins-local/`** — repo-local plugin marketplace (`true-bdd-local`) carrying `go-bdd-lsp`: gopls with `-tags=bdd` and `tmp/`/`tests/legacy/` filtered (config in `go-bdd-lsp/.lsp.json`). One-time per machine: `claude plugin marketplace add ./.claude/plugins-local`. After editing the plugin, bump its `version` and run `claude plugin marketplace update true-bdd-local` — installs are version-keyed cache copies. Keep `gopls-lsp@claude-plugins-official` disabled in settings: when two LSP servers claim `.go`, only the first registered starts.
- Never add a tracked `.crush.json`/`crush.json` at the repo root — crush config discovery merges every one it finds walking up from cwd, and a root config silently hijacks every fixture's apply turn.
- Environment variables should be stored in .env files (excluded from git)
- Invoke the Vercel CLI via `npx vercel` (no global install)
- Never update `.golangci.yaml` without my permission
- **CRITICAL**: NEVER merge pull requests without explicit user command to merge
- **CRITICAL**: NEVER use `git commit --amend` or `git push --force`/`--force-with-lease`. Always create new commits.
