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
