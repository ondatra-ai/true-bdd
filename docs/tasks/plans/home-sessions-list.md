# Plan — Home page: live sessions list (`home-sessions-list`)

TESTS-ONLY plan (hard lane). Covers the e2e test layer and the H1 test-layer
surgery only. It plans NO production changes: the test-fixer derives the home
page from the failing specs alone (Spec-as-Source). Paths come from
`docs/context/paths.yaml`.

## Goal

Give the harness root (`/`) a real, live sessions list under test: the e2e suite
must pin every behavior in the brief (P1–P10) and retire the pre-workspace
surfaces (H1) so the protocol suite has no contradictory contract left behind.
The test-fixer, reading only the red specs, must be forced to build a `/` page
that renders one row per connected CLI session (folder realpath title, session
id, version), opens each session's workspace, stays live without a reload, shows
an honest empty state, and degrades to an unavailable notice on sustained read
failure — all styled from the prototype's sessions design.

## Non-goals

- No Test-connection control (dropped by the user; retire it from the contract).
- No session-detail (`/sessions/<sid>`) or run-view specs/testids are added or
  reworked beyond the p1/p2/p8 surgery H1 names — those surfaces stay out of
  scope, and their contract entries (used by other, out-of-scope specs) are left
  untouched. This task removes ONLY `test-connection` and the p1/p2/p8
  retired halves.
- No production/implementation planning: no files-to-touch list, no code
  sketches, no lib/route design. The plan stops at "what the specs assert" and
  "what empty scaffolding they need."
- No new engine endpoint: the specs drive only the existing `GET /api/sessions`
  registry read (and the existing `/sessions/<sid>/home` workspace entry).

## Current state

- `harness/src/app/page.tsx` is a placeholder Server Component returning 200
  with "Sessions list is not implemented yet" — no `session-row`, no list, no
  live behavior. This is the RED surface: every new sessions-home assertion
  fails as a missing-element assertion against this placeholder, never a crash
  or an unresolved route.
- `GET /api/sessions` (`harness/src/app/api/sessions/route.ts`) already returns
  `SessionSummary[]` = `{session_id, folder (canonical realpath), pid, version,
  connected_at}`, registry-only (every listed session is connected). The specs
  drive the live list entirely off this existing route.
- The workspace at `/sessions/<sid>/home` already renders (w1–w17); it is the
  Open-workspace navigation target and needs no new work — w18.3's RED is only
  the absent Open control on `/`.
- Contract (`tests/harness/helpers/ui.ts`): `routes.sessions = "/"`,
  `TID.sessionRow` (+`data-session-id`/`data-folder`), `TID.sessionFolder`,
  `TID.sessionVersion`, `TID.testConnection`; `helpers/README-testids.md`
  documents them under "Sessions list (`/`)".
- Legacy specs assert retired surfaces: **p1** (list-realpath half + a
  `/sessions/<sid>` detail-inventory half), **p2** (protocol registry-drop/404
  half + an open detail-page-clearing half), **p8** (Test-connection → version
  run → run view, entirely retired).
- Relay poll-lease expiry is 10s (`redis-relay.ts` `expiryMs: 10_000`): a silent
  CLI leaves the registry ~10s after its lease lapses, so a UI poll of a few
  seconds meets the one-minute disappearance bound (P4/P5).
- Design baseline: the runnable prototype route `/sessions`
  (`harness/design/proto-workspace/app/app/sessions/page.js` → `content/
  sessions.html`) — gradient top bar (`--gradient-spiral-soft`, single gradient
  use), TrueBDD wordmark (`data-token-probe="--text-inverse"`), tagline, h1
  "Sessions", `.row-list` rows (title=folder, meta="session <id> — connected",
  version chip). The prototype still shows a Test-connection button and has no
  empty state — parity is "prototype minus Test connection, plus empty state".
- Design gates today (w8/w9/w15/w16/w17) target workspace pages only; the
  sessions page joins the design layer through this plan's new deterministic
  parity case (w18.10) plus an optional codex vision judge (w19).

## Target state

- A new deterministic protocol-style suite `w18-sessions-home.spec.ts`
  (workspace project — 3-min timeout, no Claude) pins P1–P10 with strong,
  auto-waited, non-flaky assertions driven off `GET /api/sessions`, real CLI
  remotes, and browser-level route injection (the w10 idiom) for the failure/
  timing cases.
- A new `w19-sessions-design-judge.spec.ts` adds the P7 holistic codex vision
  gate (production `/` vs the live-booted prototype `/sessions`), skipping only
  when `codex` is absent — secondary to the deterministic w18.10 gate.
- H1 landed in the SAME step: p1/p2 reworked to their kept halves, p8 deleted,
  `ui.ts` + `README-testids.md` updated (drop `test-connection`; add the new
  sessions-home testids), and the two design helpers extended. No contradictory
  source of truth remains in the BINDING contract: after removing p8 and the
  `ui.ts`/README entries, no active protocol/workspace spec or contract file
  carries a positive `test-connection` requirement. (The string legitimately
  survives — out of scope, untouched — in the prototype `/sessions` baseline, the
  historical `SPEC.md`/task docs, a `src/cmd/version.go` comment, and as w18.10's
  negative-assertion literal.)
- New `ui.ts` sessions-home testids (test-author adds; test-fixer emits):
  `TID.sessionRow`/`sessionFolder`/`sessionVersion` (kept), plus
  `session-meta` (meta line carrying the session id), `session-open` (per-row
  Open-workspace `<a>`, href = `wsRoutes.home(sid)`), `sessions-list` (row-list
  container — the exact-count list assertions scope rows to it), `sessions-empty` (empty
  state), `sessions-unavailable` (P10 notice). A P5 NEGATIVE contract is added
  too — `session-disconnected`, `session-unreachable`, `session-reconnect` —
  documented as markers production must NEVER render (w18.5 asserts each count 0).
  `test-connection` removed.

## End-to-end test cases

Each case fails when its behavior is absent (RED against the placeholder or
against a broken implementation). Env choice: multi-session cases use
`ProtocolEnv` directly (multiple `startRemote` against the `bare-host` fixture);
single-session cases may use `ProtocolEnv` + one remote or `WorkspaceEnv`.
"exactly N rows / count 0" assertions use Playwright locator auto-wait with a
generous (≤60s) timeout — never a fixed sleep.

### `w18-sessions-home.spec.ts` (workspace project; deterministic)

- **w18.1 — P1: one row per connected session.** Start two remotes; navigate to
  `/`. Assert the `sessions-list` container is visible, that
  `sessionsList.getByTestId(TID.sessionRow)` has count exactly 2 — rows scoped to
  the documented container, pinning the `sessions-list` contract and the
  prototype `.row-list` parity — and that a row with each session's
  `data-session-id` is visible. *Absent → placeholder has no list/rows.*
- **w18.2 — P2: row identity (folder realpath / id / version).** With one
  connected session, read its `SessionSummary` from `GET /api/sessions`. Assert
  the row's `session-folder` text EQUALS `summary.folder` (a realpath), its
  `session-meta` text CONTAINS `summary.session_id`, and its `session-version`
  text EQUALS `summary.version` (non-empty). *Absent → no such cells.* (The
  symlink→realpath proof stays in reworked p1; this asserts the general row
  anatomy.)
- **w18.3 — P3: Open workspace enters the session's workspace (per-row link
  contract).** Start TWO sessions. For EACH row, assert its `session-open` is an
  anchor (`<a>`) whose `href` resolves EXACTLY to `wsRoutes.home(thatRowSid)` —
  pins the row→session association; a shared href, a `<button>` with imperative
  navigation, or a handler unrelated to its row fails here. Then click ONE row's
  control and assert `page.url()` matches `wsRoutes.home(thatSid)` AND the
  workspace shell for THAT session renders (`WTID.rail`, `WTID.sidebar`,
  `WTID.contentBreadcrumb` visible). *Absent → no Open control, wrong/missing
  href, or the wrong destination.*
- **w18.4 — P4: a newly connected CLI appears live without reload.** Load `/`
  with one session and assert its row. From this point, count main-document
  navigation requests (`request.isNavigationRequest() && resourceType() ===
  "document"`). Start a second remote; assert its row becomes visible on the same
  page (`expect(sessionRow(page, sid2)).toBeVisible({timeout: 60_000})`) AND the
  navigation-request count stayed 0 — proving a live poll, not a
  `location.reload()`. *Absent → a one-shot page never shows row 2, or a
  reloading page trips the navigation counter.*
- **w18.5 — P4/P5: a stopped CLI vanishes live, structurally, with no marker.**
  Load `/` with two sessions; count main-document navigation requests from here
  (as in w18.4). Stop remote B (kill/SIGSTOP so its lease lapses). Assert every
  element carrying `data-session-id="<sidB>"` reaches count 0 within 60s — the
  row AND any per-B chip/status/reconnect control (STRUCTURAL absence, never a
  body-text scan: the prototype's own copy legitimately contains the words
  "reachability"/"reconnection", so banning those strings would reject faithful
  design). Row A stays visible, and the navigation-request count stayed 0 (live
  drop, not a reload). Also assert the P5 NEGATIVE contract holds THROUGHOUT the
  disconnect window: install a `MutationObserver` (or equivalent continuous
  recorder) BEFORE stopping B that records any appearance of the forbidden markers
  `session-disconnected`/`session-unreachable`/`session-reconnect`, then assert
  the recorded set stayed empty — a final `toHaveCount(0)` alone would miss a
  marker that flashed and was removed, and a generic disconnected/reconnect
  affordance carrying no `data-session-id` would evade the structural check.
  *Absent → B lingers, a dead-session/reconnect marker appears (even transiently),
  or the page reloaded.*
- **w18.6 — P6/P7: honest empty state in the design language.** Start a server
  with NO connected remote; navigate to `/`. Assert `sessions-empty` is visible,
  containing a "no sessions connected" message AND a short hint on how to connect
  one (assert both text fragments), `TID.sessionRow` has count 0, AND the
  `sessions-list` container is absent (count 0 — the empty state REPLACES the
  list, mirroring how P10's notice replaces it). Assert the
  empty frame keeps the prototype design language: the gradient top bar and the
  `--text-inverse` wordmark still render, and the empty-state message resolves a
  token-driven muted color (computed `color` == `--text-muted` via `cssVarRgb`).
  *Absent → placeholder text, or an unstyled empty block.*
- **w18.7 — P8: stable oldest-first order across live refreshes.** Start remote A
  and await its `SessionSummary` in `GET /api/sessions` (capture
  `a.connected_at`) BEFORE starting remote B; then await B's summary and assert
  `b.connected_at > a.connected_at` as a setup guard (defeats a millisecond
  `Date.now()` tie — retry setup if it ties). Navigate `/`; read the DOM order of
  `session-row` `data-session-id`s and assert it equals the ids sorted by
  `connected_at` ascending (A before B). Arm two `page.waitForResponse` promises
  (each matching method `GET` + exact pathname `/api/sessions` + `status()===200`)
  BEFORE the polls fire, await each COMPLETED response, then re-read the DOM order;
  assert it is UNCHANGED. *Absent → arrival/hash order, or a reshuffle across
  polls.*
- **w18.8 — P9: no empty-state flash before the first read resolves.** Install a
  `page.route` on `**/api/sessions` whose handler resolves an `intercepted`
  handshake promise on ENTRY, then AWAITS a separate `release` gate before
  fulfilling 200 `{sessions: []}`. Navigate to `/`; AWAIT `intercepted` — proving
  the first read is genuinely in flight, not merely a not-yet-mounted client that
  would trivially show 0 empty states. While the handler is still blocked, assert
  `sessions-empty` has count 0. Resolve `release`; assert `sessions-empty`
  becomes visible. *Absent → the empty state flashes while the read is pending;
  the handshake blocks a false green from an un-mounted client.*
- **w18.9 — P10: unavailable ONLY after the sustained-failure threshold, then
  auto-return.** Install ONE `page.route` on `**/api/sessions` whose handler owns
  a `mode`: `pass` → `route.continue()`; `fail` → fulfills HTTP 503 with a JSON
  error body (the canonical, deterministic P10 stimulus — not a discretionary
  "503-or-abort", which exercises a different fetch branch) AND resolves a
  `firstFailure` handshake on the first injected failure. ONE canonical ordered
  sequence (avoids the navigate-then-fail contradiction): (1) `await
  page.clock.install()` BEFORE navigation; (2) route starts in `pass`; (3)
  navigate `/` and await the live row (a real read succeeded); (4) set
  `mode = "fail"`; (5) advance the clock by one poll interval so a poll fires,
  await `firstFailure`, and measure the threshold FROM that handshake. Assert both
  directions: `sessions-unavailable` stays ABSENT while advancing the clock to
  just under the threshold (~29s of sustained failure) and BECOMES visible after
  advancing past it (present by ~45s), with the stale row gone at that point
  (`sessionRow(sid)` count 0 — stale rows are not carried past the staleness
  verdict). Recovery (w10 waiter-BEFORE-trigger; fake-clock timers do NOT fire on
  their own): (a) arm an exact-path `GET`, `status()===200` `waitForResponse`;
  (b) set `mode = "pass"`; (c) ADVANCE the clock by ≥1 poll interval to trigger
  the next poll; (d) await the armed response; (e) assert BOTH the row RETURNS and
  `sessions-unavailable` reaches count 0 — no manual reload. Fallback ONLY if the
  test-author finds the poll cannot be `page.clock`-driven (document the reason):
  real time with a comfortable margin — notice ABSENT at ~20–25s (a first-503 flip
  fails here), PRESENT by ~45s, real timers driving recovery — so a compliant ~30s
  impl is never false-failed. *Absent → a first-503 notice, a banner left up after
  recovery, or no recovery.*
- **w18.10 — P7: deterministic sessions-design parity (primary gate).** With one
  live session at `/` and the desktop viewport (`DESKTOP_VIEWPORT` 1440×900),
  assert the prototype's sessions frame is reproduced from tokens: a gradient top
  bar whose computed `background`/`background-image` resolves
  `--gradient-spiral-soft`; the TrueBDD wordmark consuming `--text-inverse` via
  the live-mutation token probe (set `--text-inverse` on `:root`, read the
  wordmark's recomputed `color` — proves a token, not a literal); the tagline
  present; exactly ONE visible `h1` whose accessible name/text is exactly
  `Sessions` (not `Home`, not a generic heading — matches the prototype's
  `<h1>Sessions</h1>`); each row exposes title (`session-folder`), meta
  (`session-meta`), and version chip (`session-version`). Assert the PRE-WORKSPACE
  frame explicitly (brief P7 "no icon rail, no sidebar"): `WTID.rail`,
  `WTID.sidebar`, `WTID.appShell`, and `WTID.workspaceMain` each have count 0 on
  `/`. AND the page has ZERO `test-connection` controls
  (`page.getByTestId('test-connection')` count 0 — P7 drop). *Absent → hardcoded
  colors, missing frame parts, a leaked workspace shell, or a lingering
  Test-connection control.*

### `w19-sessions-design-judge.spec.ts` (workspace project; codex vision — secondary)

- **w19.1 — P7 holistic parity (codex vision judge).** `test.skip` when `codex`
  is not on PATH; call `testInfo.setTimeout(12 * 60_000)` BEFORE booting — the
  proto helper's 180s readiness budget + up-to-360s `npm ci` exceed the 3-min
  workspace default (mirror w15). Boot the prototype live (`bootPrototype()`),
  retaining the `PrototypeServer` handle, and stop it via `stopPrototype()` in
  `afterAll` (and `stop()` in a `finally` on every failure path — never leak the
  child). Screenshot its `/sessions` baseline; render production `/` with one
  live session at `DESKTOP_VIEWPORT`, settle `document.fonts.ready`, screenshot
  it; submit both to `runDesignJudge` with `profile: SESSIONS_PARITY_PROFILE`
  (named checks `top_bar`, `wordmark_tagline`, `page_heading`, `row_list_anatomy`)
  whose rubric TOLERATES the prototype's Test-connection button and IGNORES all
  data/text values. Assert judge `exitCode === 0`, a structurally valid verdict
  via `auditVerdict(verdict, SESSIONS_PARITY_CHECK_NAMES)` (NOT the default
  `JUDGE_CHECK_NAMES`), and zero failed checks. *Absent → the placeholder `/`
  fails every named check.* Mirrors w9/w11/w15: every boot/readiness/judge error
  is a test failure, not a skip.

### H1 test-layer surgery (same step; retire the contradictory sources)

- **p1** — rework: keep the API realpath check (`session.folder === realpath`,
  not the symlink) AND the list-level render (navigate `/`, assert
  `sessionRow(sid)` visible with `session-folder` text = realpath). DELETE the
  `/sessions/<sid>` detail-inventory half (`gotoSession` + `inventoryDoc(config)
  = missing`). p1 stays a `protocol` spec proving symlink→realpath at the list.
- **p2** — rework: keep the protocol disconnect facts (SIGSTOP → session leaves
  `GET /api/sessions`; session/status/run endpoints 404 `session_gone`). DELETE
  the open detail-page-clearing UI half (`gotoSession` → `unavailableState`
  visible + `sessionRow` count 0). The UI live-drop behavior is re-homed and
  upgraded to w18.5 (list-level). p2 becomes API-only.
- **p8** — DELETE the file entirely (Test-connection → version run → run view is
  fully retired; the `version` command stays in the enum, still exercised by p2's
  in-flight dispatch).
- **`helpers/ui.ts`** — remove `TID.testConnection`; keep `sessionRow`/
  `sessionFolder`/`sessionVersion` and `routes.sessions`/`session`/`run`
  (session/run kept for out-of-scope specs); add `session-meta`, `session-open`,
  `sessions-list`, `sessions-empty`, `sessions-unavailable`, and the P5
  negative-contract markers `session-disconnected`/`session-unreachable`/
  `session-reconnect` (never emitted; asserted count 0) (+ a `sessionOpen`
  locator helper). Do NOT touch the session-detail/run-view testids.
- **`helpers/README-testids.md`** — rewrite the "Sessions list (`/`)" section to
  the new contract: drop the `test-connection` row; add the new rows incl.
  `sessions-list` (row container) and the P5 negative-contract markers
  (`session-disconnected`/`session-unreachable`/`session-reconnect`, documented
  "never rendered"); document the live-poll behavior (list stays live without
  reload; empty state only after a successful zero read; unavailable notice after
  ~30s sustained failure with auto-return; sessions gone on disconnect with no
  marker); note the design baseline is the prototype `/sessions` minus
  Test-connection plus the empty state.
- **`helpers/design-conformance.ts`** — add `SESSIONS_PARITY_PROFILE` +
  `SESSIONS_PARITY_CHECK_NAMES` for w19.
- **`helpers/proto-baseline.ts`** — add `sessions: "/sessions"` to
  `PROTO_BASELINE_ROUTES` so w19's baseline route is warmed.

## Startup scaffolding

**None required.** The three surfaces the specs drive already resolve today: the
`/` route, the `GET /api/sessions` route, and the `/sessions/<sid>/home`
workspace entry all return 200. Because the routes already boot, no empty
production file must be pre-created for the specs to run — every new
sessions-home spec goes RED as a missing-element/behavior assertion against the
live root. No new compose service, image, or route module is introduced. How the
home page is built — component placement, the client/server boundary, and any
polling mechanism — is left entirely for the task-blind test-fixer to derive from
the failing specs; the plan does not name an implementation file or steer the
architecture.

## Codex rounds

See `docs/tasks/plans/home-sessions-list.codex.md` (ledger beside this plan).

## Challenges

- **Design-baseline choice (P7).** Two viable patterns: the live-booted
  prototype (w15) — chosen for w19, no committed binary, natural `/sessions`
  route — vs a committed golden (w9/w11) captured via `goldens.update.spec.ts`.
  The deterministic w18.10 token/structure gate is the PRIMARY, non-flaky P7
  guard so CI without `codex` still enforces parity; w19 is holistic backup.
- **Prototype divergence.** The prototype `/sessions` still has a
  Test-connection button and no empty state. The vision rubric must tolerate the
  button and ignore data; the empty-state parity is checked only deterministically
  (w18.6), never by the judge (the prototype has no empty state to compare).
- **P9/P10 timing without flake.** Both use browser-level route injection with
  explicit handshakes, never fixed sleeps. P9 resolves an `intercepted` promise
  on handler entry then blocks on a `release` gate (the "no flash" check runs
  only once the first read is provably in flight). P10 installs `page.clock`
  before navigation as the PRIMARY deterministic contract — advance to ~29s
  (notice absent) then past ~30s (notice present) — with a comfortable-margin
  real-time fallback (~20–25s absence) when the poll can't be clock-driven, so a
  compliant ~30s impl is never false-failed. P10 recovery arms the
  `status()===200` `waitForResponse` BEFORE flipping the handler to `pass` (w10
  waiter-before-trigger convention) and asserts BOTH the row returns AND the
  notice clears.
- **Live "no reload" proof (P4/P5).** Rather than trusting prose, the live
  add/remove cases count main-document navigation requests after first load and
  assert the count stays 0 while the list mutates — an implementation that
  `location.reload()`s to refresh trips the counter and fails.
- **P5 negative assertion.** Asserted STRUCTURALLY, never by body-text scan — the
  prototype's own copy legitimately contains "reachability"/"reconnection", so a
  string ban would reject faithful design. Two-part check: (1) every element
  carrying the dead session's `data-session-id` reaches count 0 page-wide while
  the surviving row stays; (2) the P5 negative-contract markers
  (`session-disconnected`/`session-unreachable`/`session-reconnect`) have count 0
  page-wide throughout — catching a generic disconnected/reconnect affordance
  that carries no `data-session-id`.
- **H1 blast radius.** Only `test-connection` and the p1/p2/p8 retired halves are
  touched; `gotoSession`/`gotoRun` and session-detail/run-view testids remain
  (depended on by out-of-scope specs p3/p4/p5/p7/p9/p14 + AI specs). Verified by
  grep: within the BINDING contract + active specs, `test-connection` is
  referenced only by p8, `ui.ts`, and the README (all edited here). The remaining
  repo occurrences — the prototype `/sessions` baseline (intentional), `SPEC.md` +
  historical task docs, a `src/cmd/version.go` comment, and w18.10's
  negative-assertion literal — are out of scope and left untouched.

## Workflow log

- Read paths.yaml, the brief, codex-loop/codex mechanics, `ui.ts`,
  `README-testids.md`, p1/p2/p8, the prototype `/sessions` page + `sessions.html`,
  `SPEC.md`, the env helpers (`protocol-env`, `workspace-env`, `proto-baseline`),
  the design-conformance profiles, `api-client` session methods, and the w9/w10/
  w11 precedent specs.
- Mapped the H1 blast radius by grep: `test-connection` used only by p8 + the two
  contract files; `gotoSession`/`gotoRun` used widely by out-of-scope specs, so
  their testids are kept.
- Drafted the tests-only plan (P1–P10 → w18; P7 holistic → w19; H1 surgery).
- Codex critique loop: see the ledger.
- 2026-08-05 ~09:55 orchestrator: lane hard declared; baselines captured
  (production+off-limits manifests, package-scripts snapshot, change-surface
  copy at tmp/implement-task/baselines/home-sessions-list/). Planner completed
  (3 Codex rounds: 12/6/7 keeps). Plan reviewed; spawning test-author.
- 2026-08-05 ~10:35 orchestrator: test-author completed (3 Codex rounds; tsc
  green; RED = 10× w18 + w19.1 + reworked p1; p2 API-only green; p8 deleted).
  Scope verified: production manifest + package scripts CLEAN. Off-limits
  fixer-before snapshot taken; spawning task-blind test-fixer (reproduce block
  + ledger path only).
- 2026-08-05 ~11:20 orchestrator: test-fixer completed (3 fixed Codex rounds).
  Off-limits tree + package scripts CLEAN after fixer. Orchestrator re-ran the
  gates itself: tsc clean, unit 79/79, lint clean on the new files (the 1735
  repo lint errors are pre-existing design-prototype sweep noise). Full
  workspace project: 79 passed, 2 failed (w4, w11) — both re-run in ISOLATION
  and green (contention flakes, not regressions); p1/p2 green. Diff artifact
  generated (13 files, 1808 lines); spawning reviewer.
- 2026-08-05 ~12:00 orchestrator: reviewer completed — PASS. 3 Codex rounds
  (6+1 keeps, dry at R3); 7 findings applied, ALL as e2e hardening (w18 →12
  cases: +w18.11 hung-read notice, +w18.12 malformed-200≠empty; w18.5/w18.10
  strengthened; w19.1 deflaked). Live smoke PASS (CLI remote registered +
  browser: rows, Open workspace → workspace shell). Orchestrator FINAL re-run:
  tsc clean, w18+w19 13/13, p1+p2 2/2 — all green. Proceeding to report +
  retro + close.
