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
