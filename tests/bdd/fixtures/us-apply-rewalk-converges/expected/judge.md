# Expectations for `us apply 99.3 --fix` (re-walk converges)

The fixture seeds `docs/requirements.yaml` with an empty registry
(no scenarios). The story has two ACs (lineage `99.3-001` and
`99.3-002`).

Expected behavior under the refined `--fix` semantics:

1. **Walk #1** — both ACs miss their "present?" check. F: adds an
   entry for each AC. Walk #1 ends with every prompt passing for
   the rows the engine looked at, but two fixes were applied.
2. **Walk #2** — the new re-walk semantics kick in. AC-1 and AC-2
   are re-evaluated; both present, no duplicates, all prompts
   pass. `anyFixApplied=false` → fixpoint reached → canonical
   commit.

## What MUST be true after the run

1. `docs/requirements.yaml` contains **exactly one** scenario whose
   `user_stories[]` references `99.3-001`.
2. `docs/requirements.yaml` contains **exactly one** scenario whose
   `user_stories[]` references `99.3-002`.
3. No two scenarios in `docs/requirements.yaml` share the same
   `merged_steps` (no duplicates).
4. Each created scenario's `merged_steps.{given,when,then}` reflects
   the corresponding AC's `steps.{given,when,then}` from
   `docs/stories/99.3-rewalk-fixture.yaml`.
5. No files outside `docs/requirements.yaml` and the per-run `tmp/`
   directory are created or modified.

## What MUST NOT happen

- The string `RE-WALK 3/` MUST NOT appear in stdout. (Walk #2 was
  the clean confirmation walk — it must not itself trigger a third.)
- The string `Hit max apply attempts` MUST NOT appear in stdout.
  (The cap from `config.max_apply_attempts: 5` did not fire.)
- The canonical file MUST NOT be byte-identical to the empty seed
  `input/docs/requirements.yaml`.

## Tolerances

- Order of scenarios inside `docs/requirements.yaml` may differ.
- The created entries' `description`, `service`, `last_updated`, and
  exact ids (`INT-NNN` or `E2E-NNN`) are up to the F: handler.
- Files inside `tmp/` are scratch artifacts — ignore them.

Reply `PASS` if all the MUST-be-true rules hold AND all the
MUST-NOT-happen rules hold. Otherwise reply
`FAIL: <one-sentence reason>` describing the first violation you
find.
