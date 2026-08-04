# workspace-overview-design-parity — test-author mini-plan (easy lane, tests-only)

**Goal.** Make the production workspace-overview page (`/sessions/<sid>/home`) reach
design parity with `harness/design/mockups/workspace-overview.html` on the canvas
content + breadcrumb (icon rail + docked sidebar chrome stays, per w1–w7). Author
RED e2e specs; the icon-rail-tolerant judge gates the visual comparison.

**E2E cases (all drive the `w-workspace-happy` fixture via `WorkspaceEnv`).**
- w10.1 (R1): under `overview-title`, `overview-meta` contains the session folder
  (from `GET /api/sessions`) AND the `sid`.
- w10.2 (R2): `overview-action-build-tests|build-code|refresh` visible+enabled;
  clicking Build Tests/Code POSTs `command:"build-tests"/"build-code"` to
  `…/runs` (route-intercepted so no real run/claude spawns).
- w10.3 (R3): `overview-inventory` list renders a row per key with path+kind+chip;
  `architecture` row chip `data-status="present"`; chips ∈ the status vocabulary.
- w10.4 (R4): overview canvas present AND `overview-banner` count 0 on the clean
  (non-degraded) fixture; present-state left to the judge.
- w10.5 (R5): `content-breadcrumb` = two-level trail — a "Sessions" link to `/`
  then a current `aria-current="page"` "Workspace overview" crumb.
- w11.1 (R6): codex vision judge pair, mockup vs prod `/home` @1440×900, named
  checks title_metadata/actions_row/inventory_health/breadcrumb_trail; rail
  tolerated, data ignored; skips (named reason) when codex absent.

**Files to touch (tests only — NO production files).** New:
`tests/harness/w10-workspace-overview.spec.ts`,
`tests/harness/w11-workspace-overview-judge.spec.ts`. Additive helper/contract
edits: `helpers/ui.ts` (overview `WTID` + locators), `helpers/README-testids.md`
(document surfaces), `helpers/design-conformance.ts` (reuse judge — add the
canvas-parity profile, don't fork). No startup scaffolding needed (the `/home`
route + shell already resolve; RED is assertion-only). Codex rounds → see
`workspace-overview-design-parity.codex.md`.

- 2026-08-04T13:29Z orchestrator: fixer green 8/8 (verified independently); visual Playwright compare confirms canvas parity (title+meta, actions row, inventory chips, 2-level trail). Fixer-FLAGGED unpinned behaviors for the reviewer regeneratability audit: (1) dispatch-triggered SessionDetail refresh + monotonic stale-response guard (unit-only), (2) request.signal forwarding on GET /api/sessions/[sid] (untested), (3) open item: architecture row can report present for a schema build-code cannot walk (documented in code).
