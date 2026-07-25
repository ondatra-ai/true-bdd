# Decisions — conversational ledger

Choices made in conversation — what was chosen, why, and what was rejected —
extracted from task transcripts by the context archivist (`.claude/hooks/context.py`).
Append-only; newest at the bottom. A superseded entry is struck through, never
deleted.

## 2026-07-21 — The task investigated BDD failures and Opus model selection, then reverted the unanalysed changes while retaining the restored `bdd-cli/` directory.

_transcript: 20260721-190544-9858af28-let-s-start-from-fixing-bdd-tests-let-s.md_

- Peter chose to discard the unanalysed BDD/model-selection change set and retain only the restoration of `bdd-cli/`; everything else from that attempt was rejected because `stage_model.go` had been introduced without prior analysis (2026-07-21T19:34:33Z).
