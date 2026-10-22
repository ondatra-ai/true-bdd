# Fixture workspace: build-tests-fix-default-write-roots

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


The host now runs its generated Scenarios. The suite shim in tests/mcp/scenarios uses the real bddgo harness with the existing steps.State and steps.Suite. Author Register(suite Suite) in a new definitions file, with callbacks func(*State, []string) error. Reuse the State in state.go, its BaseURL, Client and Response; do not redeclare the types. The shim starts the host HTTP service and sets MCP_PORT to its address for each Scenario. Never edit the shim or state.go.
