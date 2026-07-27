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
