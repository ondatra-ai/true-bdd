# Fixture workspace: build-tests-fix-happy-path

Synthetic host project driven by TrueBDD's `build tests --fix`.

- Executable tests live under `tests/` (Playwright integration specs under
  `tests/integration/`).
- When authoring a missing test, create it under `tests/` and reference the
  scenario id from `docs/scenarios.yaml` so the registry walk can find it.
- Never modify `docs/scenarios.yaml` or `true-bdd/`.
