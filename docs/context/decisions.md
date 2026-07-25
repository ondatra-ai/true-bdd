# Decisions — conversational ledger

Choices made in conversation — what was chosen, why, and what was rejected —
extracted from task transcripts by the context archivist (`.claude/hooks/context.py`).
Append-only; newest at the bottom. A superseded entry is struck through, never
deleted.

## 2026-07-21 — The task investigated BDD failures and Opus model selection, then reverted the unanalysed changes while retaining the restored `bdd-cli/` directory.

_transcript: 20260721-190544-9858af28-let-s-start-from-fixing-bdd-tests-let-s.md_

- Peter chose to discard the unanalysed BDD/model-selection change set and retain only the restoration of `bdd-cli/`; everything else from that attempt was rejected because `stage_model.go` had been introduced without prior analysis (2026-07-21T19:34:33Z).

## 2026-07-25 — The task reconciled independent source-level checklist branch inventories and began designing measurable BDD coverage from the verified branch model.

_transcript: 20260725-120421-f9fa25d3-task-is-to-write-more-bdd-tests-the-idea.md_

- Measure checklist content coverage as both the pass and semantic-fail outcome of every Q plus one end-to-end fix-effective chain for every authored F; protocol-coerced failures must not count as semantic-fail coverage, and shared clarify/refine, re-walk, stop-reason, and exit-code mechanics should be covered by global regression fixtures rather than duplicated per checklist, because the investigators verified that F text is validator prompt content rather than an applied/converged state (verified 2026-07-25T13:32:16Z).
