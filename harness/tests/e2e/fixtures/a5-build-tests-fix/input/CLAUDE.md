# Fixture workspace: a5-build-tests-fix

Synthetic host project driven by TrueBDD's `build tests --fix` from the
web harness.

- Executable tests live under `tests/` (Playwright integration specs under
  `tests/integration/`).
- When authoring a missing test, create it under `tests/` and reference the
  scenario id from `docs/scenarios.yaml` so the registry walk can find it.
- The FIRST line of the test name MUST start with the scenario id followed
  by `: ` (e.g. `INT-901: ...`).
- Never modify `docs/scenarios.yaml` or `true-bdd/`. Never write outside
  `tests/`.
