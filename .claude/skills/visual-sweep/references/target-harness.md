# visual-sweep target adapter — the true-bdd web harness

The only project-specific file in this skill. It tells a sweep how to launch
this repo's target app, where findings get pinned, and where fixes land. To
point visual-sweep at a different app, replace this file — the skill core and
probes stay unchanged. The **Pin target** and **Fix target** sections below
are forwarded by the driver agents into their crush prompts — crush is the
writer that must follow them.

Paths referenced by key come from `docs/context/paths.yaml`.

## Launch (once per run)

```bash
docker compose build harness && docker compose up -d   # harness :4517 + redis :6379
```

A workspace needs a registered CLI session. Check `curl -s
http://127.0.0.1:4517/api/sessions`; if the demo session (folder ending
`demo-host-inventory`) is missing:

```bash
# CLI binary if absent: mkdir -p ./bin && go build -o ./bin/true-bdd ./src
# demo host folder if absent (tmp/ was wiped):
#   mkdir -p tmp/demo-host-inventory
#   cp -R true-bdd templates tmp/demo-host-inventory/
#   cp -R tests/harness/fixtures/w-workspace-happy/input/docs tmp/demo-host-inventory/
(cd tmp/demo-host-inventory && env -u CLAUDECODE ../../bin/true-bdd remote --server http://127.0.0.1:4517 > remote.log 2>&1 &)
```

Poll `/api/sessions` until the session lists, then take its `session_id`.

## Entry URLs (probe targets)

`http://127.0.0.1:4517/sessions/<session_id>/architecture` — primary probe
entry (richest chrome: rail, sidebar, file view, chat dock). Also sweep
`/home`, `/product`, `/builds`, and the sessions list `http://127.0.0.1:4517/`.
The walk-shifts route harvest discovers the rest.

## Probe runtime

Playwright resolves from the e2e suite's tree:
`VS_REQUIRE_FROM=tests/harness/package.json`. Viewport 1440×900 (the default).

## Pin target (what the test-author follows)

- Specs: `tests/harness/` (paths.yaml `e2e_tests`), **workspace** Playwright
  project — file `w<N>-<theme>.spec.ts` at the next free number
  (`ls tests/harness/w[0-9]*.spec.ts`); one spec file per round, one `test()`
  per finding.
- Per-test environment: fresh `WorkspaceEnv` per test
  (`helpers/workspace-env.ts`), `test.use({ viewport: { width: 1440, height:
  900 } })`.
- Oracle patterns to copy: `w20-visual-stability.spec.ts` — buffered
  `observeLayoutShifts` via `addInitScript` with the `__lsReady` guard and
  flush, `expect.poll` settle-waits, `boundingBox` assertions with ≤2px
  tolerances.
- Locator/testid contract: `helpers/ui.ts` (`WTID`/`TID`, `wsRoutes`,
  `gotoWorkspace`); every NEW testid goes into `WTID` **and**
  `helpers/README-testids.md`.
- Typecheck gate: `cd tests/harness && npx tsc --noEmit`.

## Fix target (what the test-fixer touches)

- Production code: `harness/src/**` only (paths.yaml `harness_code_root` —
  gitignored, regeneratable from the e2e suite). Unit tests:
  `harness/src/tests/unit/`.
- Style truth: paths.yaml `design_system` — `harness/design/system/tokens.css`
  + the `harness/design/proto-workspace/` prototype. Never ad-hoc values.

## Run commands

```bash
cd tests/harness && TRUE_BDD_E2E_KEEP_STACK=1 npx playwright test w<N>-*.spec.ts w20-visual-stability.spec.ts
```

Regression canary per round: `w20-visual-stability.spec.ts` (must stay green
alongside the new specs). Spec runs build the `true-bdd-harness:e2e` image via
global-setup and boot their own per-test containers — they do not touch the
manual `:4517` stack, but they may take the shared Redis down on teardown.

## Restack (between rounds)

```bash
docker compose build harness && docker compose up -d
```

Then re-check `/api/sessions` — if the session is gone (Redis restarted),
re-register per Launch. Round N+1 must explore the rebuilt image, never the
stale one.

## Cross-run memory

Waivers + verified-clean surfaces: paths.yaml `visual_waivers`
(`docs/context/visual-waivers.md`). Already-pinned findings: the doc-comments
at the top of `tests/harness/w*.spec.ts`.

## Final regression (after the loop)

```bash
cd tests/harness && npx playwright test --project=workspace
```
