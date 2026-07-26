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
