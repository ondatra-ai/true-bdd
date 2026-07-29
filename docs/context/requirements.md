# Requirements — conversational ledger

Standing requirements and corrections extracted from task transcripts by the
context archivist (`.claude/hooks/context.py`). Append-only; newest at the
bottom. A superseded entry is struck through, never deleted.

Not the BDD requirements registry: product scenarios live in a host project's
`docs/requirements.yaml`; this file holds engine-development knowledge from
conversations.

## 2026-07-21 — The task investigated BDD failures and Opus model selection, then reverted the unanalysed changes while retaining the restored `bdd-cli/` directory.

_transcript: 20260721-190544-9858af28-let-s-start-from-fixing-bdd-tests-let-s.md_

- [correction] Analyze the existing design before introducing model-selection code or stage abstractions; Peter required a full revert after `stage_model.go` was added without that analysis (2026-07-21T19:33:37Z).

## 2026-07-24 — The task added CI-backed strict branch protection, merged all open PRs against current main, fixed lint-config verification, and cleaned up branches.

_transcript: 20260724-172515-a34493d8-on-linkedin-ai-repo-there-s-new-feature.md_

- Pull requests must be updated to the latest commit on `main` before they can be merged (requested 2026-07-24T20:57:27Z).

## 2026-07-25 — This task began the requested, research-led rename of `bdd-cli` to `true-bdd`, with Codex critiques required for both the plan and implementation.

_transcript: 20260725-111029-40e36181-research-me-where-we-use-true-bdd-and-wh.md_

- For the `bdd-cli` to `true-bdd` rename, first research the impact thoroughly, then create a plan, share the goal and plan with Codex for critique, implement the improved plan, ask Codex to identify implementation weaknesses, and apply the relevant review suggestions (requested 2026-07-25T11:18:13Z).

## 2026-07-25 — The task began designing broader BDD coverage for every checklist branch by comparing independent Claude and Codex proposals before implementation.

_transcript: 20260725-120421-f9fa25d3-task-is-to-write-more-bdd-tests-the-idea.md_

- ~~Expand the BDD test suite to cover every checklist branch, explicitly exercising both true and false outcomes for each Q and F (requested 2026-07-25T12:04:21Z).~~ _(superseded 2026-07-25)_
- ~~Before implementing the checklist-branch coverage, obtain independent implementation proposals from a Claude agent and Codex, allow Codex to run commands that probe current behavior, and compare the two solutions (requested 2026-07-25T12:04:21Z).~~ _(superseded 2026-07-25)_
- Checklist-branch coverage must be measurable from current BDD runs, exposing which branches are covered and uncovered in a code-coverage-like report so the team can target all branches and add scenarios incrementally (requested 2026-07-25T12:45:33Z).
- Use a two-phase process for checklist-branch coverage: first have Claude and Codex independently inspect the codebase and reconcile the actual reachable branches, then design the coverage-measurement approach from that verified inventory (requested 2026-07-25T12:53:45Z).
- [correction] Do not assume every Q has exactly pass/fail branches or every F has applied/converged branches; derive the coverage targets from verified code behavior because Peter challenged those categories as unverified (corrected 2026-07-25T12:53:45Z).
- Build the checklist-branch coverage bootstrap collector, report, and baseline through a staged critique loop: create a goal-aligned plan, have Codex identify gaps, fill only goal-relevant gaps, implement, inspect the reporter’s covered and uncovered branches, then have Codex verify the implementation and results and fix only the remaining relevant gaps (requested 2026-07-25T14:46:26Z).

## 2026-07-25 — The task began reworking checklist-branch BDD fixtures to use one-item checklist overrides while preserving coverage against the full shipped checklist.

_transcript: 20260725-185231-8244ea0b-now-i-want-you-to-implement-bdd-tests-to.md_

- ~~For checklist-branch BDD tests, derive the coverage universe from the entire shipped checklist, but have each coverage fixture override that checklist with only the single step it targets; implement the change with the same Codex consultation approach used for the original solution (requested 2026-07-25T21:43:32Z).~~ _(superseded 2026-07-25)_
- For checklist-branch BDD tests, continue deriving the coverage universe from the full shipped checklist and giving each fixture only its targeted step, but eliminate copied checklist prompt and rationale text: during fixture preparation, generate the one-step checklist from the live shipped checklist plus fixture-provided information; plan the change, have Codex critique the plan, incorporate relevant suggestions, implement it, then have Codex review the implementation and apply relevant findings (requested 2026-07-26T06:47:47Z).
- Remove `bdd_guidelines` completely from TrueBDD, including its configuration, checklist and document references, fixture stubs, and documentation mentions (requested 2026-07-28T06:41:47Z).

## 2026-07-28 — The task corrected the intended host-project document/source map, validated its HTML visualization, and produced a Mermaid Markdown counterpart.

_transcript: 20260728-102315-7b460a15-ok-let-s-try-to-visualyse-relatiotiship.md_

- [correction] Model host-project production code as service-specific directories under `src/` (for example, `src/service1`), not as `services/<name>/` (corrected 2026-07-28T15:20:28Z).
- [correction] Structure `architecture.yaml` with `services:` and an `environment:` section split into `dev:` and `prod:`; it must declare the path to required `docker-compose.yaml` and, when present, optional `docker-compose.dev.yaml` (corrected 2026-07-28T15:20:28Z).
- [correction] Treat `vocabulary` as the definition of allowed BDD language: actions map phrases to concrete meanings, forbidden qualifiers enumerate vague wording, and forbidden actions include replacements; prefer the PRD for forbidden-action/replacement material while allowing it in architecture (corrected 2026-07-28T15:20:28Z).
- Populate the TrueBDD repository’s root `.mcp.json` with MCP server configurations copied from neighboring repositories (requested 2026-07-28T15:48:13Z).

## 2026-07-28 — The task scoped and began a test-first, Codex-reviewed Next.js web harness for inspecting TrueBDD projects and invoking existing CLI workflows.

_transcript: 20260728-193056-431e4f8a-next-task-would-be-to-create-web-interfa.md_

- ~~Create the web harness as a Next.js project under `./harness/`; it must let users select a project folder, parse that folder's `true-bdd.yaml` and required documents, and show what is present and missing (requested 2026-07-28T19:30:56Z).~~ _(superseded 2026-07-29)_
- [correction] Folder selection from the browser was rejected as the wrong direction: the harness server and browser never touch a host project's filesystem. Instead the CLI gains a `remote` mode started INSIDE the host folder that connects OUT to the harness server over an RPC (pull/poll) protocol; the server learns the folder path and document inventory from the connected CLI, and "selecting a folder" in the UI means picking a connected CLI session. Multiple concurrent sessions must be supported (corrected 2026-07-28T19:52:00Z).
- The harness must expose user-story Create, Apply, and Refine operations plus Build Tests and Build Code, with every operation dispatched over the RPC channel to the connected `true-bdd remote`, which executes the existing CLI as a child process inside its own folder; the interactive `--fix` prompt/answer loop is relayed to the browser (requested 2026-07-28T19:30:56Z, transport corrected 2026-07-28T19:52:00Z).
- Develop the harness test-first: write the correct Playwright tests before implementation, center verification on Playwright, and finish implementation with all Playwright tests passing; change tests only when implementation genuinely cannot satisfy them and the change is relevant (requested 2026-07-28T19:30:56Z).
- Before planning, analyze the task, repository, and risks, ask Peter questions one at a time, and reconsider the task's purpose whenever Peter says the work is moving in the wrong direction (requested 2026-07-28T19:30:56Z).
- Use staged Codex review for the harness: iterate goal/risk/question analysis up to three rounds before planning; iterate plan critique up to three rounds and separately review the Playwright plan; then iterate post-implementation critique and relevant fixes up to three rounds. Give Codex access to the repository, `CLAUDE.md`, documents, plans, and runnable commands, and where possible let it run tests and inspect the site with Playwright (requested 2026-07-28T19:30:56Z).
- Use separate agents for writing Playwright tests and implementing the harness code, and actively manage their work (requested 2026-07-28T19:30:56Z).
- Use Opus rather than Fable for all future code- and test-writing agents (requested 2026-07-28T22:33:37Z).
- For the harness UI rework, keep the established Codex-involved process, test the UI itself, and do not exercise time-consuming operation buttons in this round (requested 2026-07-29T05:54:37Z).
- [correction] The harness review UI must present a useful domain hierarchy: a list of epics expandable into their user stories, with each story openable in a popup for review; the existing flat story presentation was rejected as not useful (corrected 2026-07-29T05:54:37Z).

## 2026-07-29 — Peter redirected the harness architecture to CLI-owned state with a thin-client web UI.

- [correction] All harness state moves to the CLI: each `true-bdd remote` owns a per-project SQLite inside its host folder; the web/Next.js server keeps NO database at all — it is a stateless relay with only an in-memory registry of connected CLIs (the earlier server-owned-SQLite design is superseded) (corrected 2026-07-29T12:40:00Z).
- The WebUI must reflect exactly the CLI's current state — no cache, no server-side store-and-forward: every view reads the CLI's state through the relay, and when the CLI requires user action the web shows a dialog and sends the response to the CLI via RPC (requested 2026-07-29T12:40:00Z).
- Sessions disappear from the UI when their CLI disconnects (no last-known ghost entries) (requested 2026-07-29T12:40:00Z).
- The harness UI rework must use the repository’s established S&F Design System (requested 2026-07-29T06:40:52Z).
- For the harness UI rework, automated Playwright tests must be supplemented with a hands-on Playwright MCP smoke pass through representative scenarios and a visual assessment of whether the interface looks and feels smooth (requested 2026-07-29T08:07:14Z).

## 2026-07-28 — The task requested and implemented a repository launcher for selecting Anthropic, Z.AI, or Kimi as the Claude Code backend using configuration from a gitignored `.env` file.

_transcript: 20260728-193056-431e4f8a-next-task-would-be-to-create-web-interfa.md_

- Provide a repository-root `start.sh` launcher that can start Claude Code in standard Anthropic, Z.AI, or Kimi mode and reads backend configuration and secrets from a gitignored `.env` file; Kimi must be supported even though it was not previously configured (requested 2026-07-29T16:10:25Z).
