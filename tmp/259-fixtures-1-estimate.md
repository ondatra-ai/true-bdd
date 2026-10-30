# Estimate: #259 fixture batch 1, the scen-check family

Written before any recording, 30 October 2026. Allowance: $35 of Claude across all attempts (the Owner's decision, of $80 for the whole migration).

## Count

- `tests/true-bdd/fixtures/scen-check-*`: 66 fixtures, of which 45 carry `catalogue:` or `phrases:` (3 carry both: `happy-path`, `validation-fails`, `fix-rewrites-description`).
- Of those 45, 19 run AI turns and 26 run none.
- Left for the third part (they pin the template form itself): 12 of the 26 that run no turn and 4 of the 19 that do. Their reasons go in the report and the commit.
- To migrate: 14 that run no AI turn (replayed only) and 15 that run AI turns (re-recorded).

## Recorded spend of the 15 to re-record

Claude spend is the sum of `total_cost_usd` in each cassette's result event; codex turns are not billed against the allowance.

| Fixture | Claude turns | Codex turns | Recorded $ | Expected turns on a concept host |
|---|---|---|---|---|
| scen-check-claude-turn-fails | 1 | 0 | 0.00 | 1 (narrowed, failing model id) |
| scen-check-codex-turn-fails | 0 | 1 | 0.00 | 1 (narrowed, failing model id) |
| scen-check-prompt-names-its-tiers | 1 | 4 | 0.18 | 5 (own one-Prompt checklist) |
| scen-check-fix-adds-measurable-criterion | 5 | 1 | 0.61 | 6 (narrowed) |
| scen-check-fix-adds-missing-step | 4 | 1 | 0.41 | 5 (narrowed) |
| scen-check-fix-rewrites-description-qualifier | 4 | 1 | 0.40 | 5 (narrowed) |
| scen-check-fix-rewrites-forbidden-verb | 4 | 1 | 0.43 | 5 (narrowed) |
| scen-check-fix-rewrites-implementation | 4 | 1 | 0.45 | 5 (narrowed) |
| scen-check-fix-rewrites-step-qualifier | 4 | 1 | 0.42 | 5 (narrowed) |
| scen-check-fix-rewrites-subjective-outcome | 5 | 1 | 0.62 | 6 (narrowed) |
| scen-check-fix-records-term | 2 | 1 | 0.32 | 3 (own D-only checklist) |
| scen-check-fix-records-mapping | 2 | 1 | 0.36 | 3 (own D-only checklist) |
| scen-check-happy-path | 10 | 0 | 0.94 | 13: the shipped walk gains the link, binding and word Prompts |
| scen-check-validation-fails | 10 | 0 | 0.95 | 11 to 13: the link may fail E2E-001, which skips the binding and word turns |
| scen-check-fix-rewrites-description | 24 | 1 | 1.85 | about 31: three walk passes, each concept pass three turns more |
| **Total** | **80** | **14** | **7.94** | about 120 |

A concept turn costs $0.05 to $0.09 in the existing concept-link recordings, so the 15 cost about $8.80 for one attempt each. With a second attempt on the three shipped-checklist walks, whose real-model link, binding and word verdicts the host must earn, the expected spend is $11 to $14, well inside $35.

## Not re-recorded

The 14 migrated fixtures that run no AI turn are replayed only; their goldens stay empty unless the run changes a file.
