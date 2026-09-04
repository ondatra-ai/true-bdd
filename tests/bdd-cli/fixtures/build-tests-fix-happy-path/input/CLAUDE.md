# Fixture workspace: build-tests-fix-happy-path

Synthetic host project driven by TrueBDD's `build tests --fix`.

- Scenarios live in `docs/scenarios.yaml`. They are not copied into test
  files — a scenario-driven runner reads them at run time.
- What makes a scenario executable is a step definition: a
  `suite.Step(`<regexp>`, <func>)` call in the `steps/` package of the
  suite that owns it.
- Which suite owns it comes from `docs/architecture/architecture.yaml`:
  the tests/<service>/ tree whose `service:` matches the
  scenario's. Here that is `mcp`, rooted at `tests/mcp`, so definitions
  belong in `tests/mcp/steps/`.
- Never modify `docs/scenarios.yaml` or `true-bdd/`.
