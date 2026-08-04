# Design conformance tests

## Goal

The harness workspace UI must provably match its design source — the mockups and
design system under `harness/design/` — verified by e2e tests in the Playwright
suite. Design requirements live as tests only (no separate requirements doc); the
mockup is the spec artifact.

## Requirements

- **R1 — token conformance (deterministic).** An e2e spec must assert that a
  rendered workspace page uses ONLY design-system values: the computed colors and
  font families of its visible elements must resolve to values defined by the
  custom properties in the design-system tokens file (paths.yaml →
  design_system). Ad-hoc colors or typography outside the token set must fail the
  spec with the offending element and value named.
- **R2 — design judge (AI vision).** An e2e spec must screenshot a designed
  mockup page (paths.yaml → design_system mockups) and the corresponding
  production workspace page at the same desktop viewport, submit both images to
  the local `codex` CLI with a rubric of concrete named checks derived from the
  design spec (persistent sidebar/breadcrumb/canvas frame, sidebar fixed width,
  breadcrumb bar with hairline border, canvas padding), and receive a
  schema-forced JSON verdict. The spec must fail with the named failed checks
  when the verdict reports violations. Content differences (fixture data vs
  designed placeholder data) must NOT fail the judge — only structure, layout,
  spacing, and typography intent count.
- **R3 — suite conventions.** The specs live in the existing e2e suite
  (paths.yaml → e2e_tests), follow its conventions (per-test server,
  filename-based project routing, testid contract), and skip cleanly with a
  named reason when the `codex` CLI is unavailable (R2 only).
- **R4 — red first, gate forever.** Against the current app the specs must be
  RED as assertion failures (not crashes) wherever the app genuinely deviates
  from the design source; once green they are the permanent design gate.
