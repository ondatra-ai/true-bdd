# Facts — conversational ledger

Dated empirical discoveries about external systems, extracted from task
transcripts by the context archivist (`.claude/hooks/context.py`). Append-only;
newest at the bottom. A superseded entry is struck through, never deleted.

## 2026-07-24 — The task added CI-backed strict branch protection, merged all open PRs against current main, fixed lint-config verification, and cleaned up branches.

_transcript: 20260724-172515-a34493d8-on-linkedin-ai-repo-there-s-new-feature.md_

- GitHub branch protection on `main` requires the `gates` check with strict up-to-date enforcement; this was verified when each PR became blocked after `main` advanced and became mergeable only after updating and rerunning the check (verified 2026-07-25T06:06:34Z).

## 2026-07-25 — The task researched OpenSpec’s architecture and how its CLI interoperates with agent harnesses such as Claude Code.

_transcript: 20260725-082510-cc6a7058-let-s-researc-h-architecture-of-openspec.md_

- OpenSpec uses a pull-style harness integration: it does not invoke an LLM; `openspec init` installs on-demand workflow skills and harness-specific slash commands, after which the harness agent drives a stateless CLI through shell calls and file reads/writes (verified by source review 2026-07-25T08:28:04Z).
- OpenSpec’s core runtime call is `openspec instructions <artifact> --change <name> --json`, which compiles project context and rules, a schema-defined template and instruction, dependencies, workflow state, and the resolved output path into a per-step instruction packet; artifact dependencies form a YAML-defined DAG and completion is inferred from files or task checkboxes (verified by source review 2026-07-25T08:28:04Z).
- OpenSpec replaced managed instruction blocks in root `CLAUDE.md` or `AGENTS.md` files with on-demand skills; its Claude Code commands use `allowed-tools: Bash(openspec:*)` to pre-approve CLI calls rather than restrict all other tools (verified by source review 2026-07-25T08:28:04Z).
- OpenSpec’s `validate` command performs deterministic structural validation and does not provide semantic LLM-based judging comparable to true-bdd’s checklist judge (verified by source review 2026-07-25T08:28:04Z).

## 2026-07-25 — The task reconciled independent source-level checklist branch inventories and began designing measurable BDD coverage from the verified branch model.

_transcript: 20260725-120421-f9fa25d3-task-is-to-write-more-bdd-tests-the-idea.md_

- Claude Code's declared Write/Edit path restrictions did not enforce a write sandbox: retained build-tests and build-code run logs showed writes outside the configured `./tmp/**` scope, so those declarations cannot be relied on to prevent out-of-scope edits (observed 2026-07-25T13:32:16Z).

## 2026-07-25 — The task was comparing expanded BDD fixtures under Sonnet and Opus, with the Sonnet comparison still in progress.

_transcript: 20260725-185231-8244ea0b-now-i-want-you-to-implement-bdd-tests-to.md_

- ~~In the interim model comparison, Sonnet judged a previously Opus-convergent `us refine` fixture more strictly and less protocol-reliably, adding failures on binary-outcome and test-convertibility checks plus an empty verdict until the fix loop was exhausted; completed `us create` and `us apply` comparisons had matched Opus (observed 2026-07-25T20:53:01Z).~~ _(superseded 2026-07-25)_
- During the Sonnet timing run, the fix generator truncated the timestamp partition in an artifact path it was required to echo; the resulting marker mismatch prevented fix-prompt extraction and caused the fixture to end NotFixed without writing the story (observed and root cause verified 2026-07-26T11:20:30Z).
- The completed Sonnet timing comparison found no meaningful speed advantage over Opus and materially worse nondeterministic reliability, including non-converging collapse and fix-application loops; current evidence therefore favors retaining Opus as the engine model (observed 2026-07-26T11:49:43Z).
- The `us-refine-fix-step-qualifier` fixture can fail nondeterministically because the fix applier rewrites the whole story and may drop unrelated `and:` lines; a clean rerun showed that the failure was not caused by the document-layout migration (observed and rerun-verified 2026-07-27T21:58:31Z).
- The original host project `awesome-claude-mcp` retains a substantive BDD scenario-writing guide at `docs/architecture/bdd-guidelines.md`; it was left behind when only `scripts/bdd-cli` was imported into TrueBDD, explaining why this repository’s fixtures contain placeholders instead (verified by source and lineage inspection 2026-07-28T06:41:31Z).
- The `us-refine-happy-path` fixture can fail nondeterministically when its binary-outcome judge treats AC-4's requested summary length as subjective; the unchanged fixture passed cleanly on rerun (observed and rerun-verified 2026-07-28T07:03:00Z).

## 2026-07-28 — The task corrected the intended host-project document/source map, validated its HTML visualization, and produced a Mermaid Markdown counterpart.

_transcript: 20260728-102315-7b460a15-ok-let-s-try-to-visualyse-relatiotiship.md_

- GitHub Markdown sanitization prevents faithfully embedding an interactive HTML visualization because scripts, styles, and inline styling are stripped; fenced Mermaid diagrams are the supported Markdown-native alternative and were locally render-verified (researched and verified 2026-07-28T15:42:28Z).
- The available Playwright browser could not open local `file:` URLs directly; serving the directory over localhost enabled the HTML inspection (observed 2026-07-28T15:51:29Z).

## 2026-07-28 — The task published the document-universe visualization to GitHub Pages, configured automatic redeployment from main, and verified the live site with Playwright.

_transcript: 20260728-185658-083b5c96-next-task-is-to-publish-docs-doc-univers.md_

- GitHub Pages is enabled for `ondatra-ai/true-bdd` and serves the document-universe map at `https://ondatra-ai.github.io/true-bdd/` and `/doc-universe.html`; both URLs rendered successfully in Playwright (verified 2026-07-28T19:00:48Z).

## 2026-07-28 — This transcript chunk continued the CLI-owned-state v2 harness lint cleanup and exposed violations hidden by golangci-lint’s default reporting caps.

_transcript: 20260728-193056-431e4f8a-next-task-would-be-to-create-web-interfa.md_

- In this repository, golangci-lint’s default issue-reporting caps hid additional violations; running with `--max-same-issues 0 --max-issues-per-linter 0` exposed the complete lint set, so uncapped runs are needed before declaring lint clean (observed and verified 2026-07-29T17:13:38Z).

## 2026-07-29 — The task corrected the deployment source and explored Git-backed Vercel deployment for the local harness, but stopped pending Vercel authentication.

_transcript: 20260729-172938-d9329923-do-you-have-vercel-connection-if-yes-cou.md_

- ~~The available Vercel connector supported one-shot deployments with source files inlined but could not establish the GitHub-to-Vercel repository connection; the local Vercel CLI also lacked reusable connector authentication, a token, or an auth file, so Git-backed deployment required separate authentication or dashboard/GitHub App setup (observed 2026-07-29T18:01:18Z).~~ _(superseded 2026-07-29)_
- The Homebrew npm global locations `/opt/homebrew/lib/node_modules` and `/opt/homebrew/bin` were writable without sudo, so installing the Vercel CLI globally would not require elevated filesystem permission (observed 2026-07-29T19:17:46Z).
- ~~The Vercel connector still cannot establish the GitHub repository connection, but the local `npx vercel` CLI is now authenticated as `killev` with `kavoon` access and successfully deployed only `harness/` as project `true-bdd-app`; `https://true-bdd-app.vercel.app` returned HTTP 200, while repository-backed automatic deployment remains unconfigured (login reported 2026-07-29T19:38:03Z; deployment verified 2026-07-29T19:44:14Z).~~ _(superseded 2026-07-29)_
- Vercel currently builds the harness with Node 24.x and cannot satisfy its committed Node 26 requirement; production deployment succeeded only after temporarily changing the engine target to 22.x and restoring it afterward, so deployments from the unchanged tree will fail until the committed target is lowered to a Vercel-supported version (verified 2026-07-29T19:44:14Z).
- The Vercel connector still cannot establish the GitHub repository connection, but local `npx vercel` remains authenticated as `killev` with `kavoon` access and deployed only `harness/` as project `true-bdd-app`; its page shell returns HTTP 200 at `https://true-bdd-app.vercel.app`, while Playwright, direct HTTP probes, and the real `true-bdd remote` client verified that browser and agent API calls receive 403 `origin/host policy violation` because the harness requires a loopback Host, making the public deployment functionally inert while repository-backed automatic deployment remains unconfigured (deployment verified 2026-07-29T19:44:14Z; functionality verified 2026-07-29T19:56:08Z).
- Codex CLI v0.146.0 stalled during a headless `exec` run without an explicit sandbox policy, while `codex exec -s read-only --ephemeral --color never` with the prompt supplied through stdin performed the read-only audit autonomously (observed and rerun-verified 2026-07-30T11:18:15Z).
