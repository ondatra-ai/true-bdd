<!-- See docs/context/requirements-guide.md. System = architecture/infrastructure
     decisions only ("must use X"); every behavior → Product, from a role. -->
# Home page — live sessions list

## Goal:

Replace the harness root's placeholder with the real home page: a live list of
the CLI sessions currently connected to the harness, from which the user opens
a session's workspace — rendered in the prototype's sessions design.

## Current behavior:

`/` renders a hardcoded placeholder ("Sessions list is not implemented yet") —
left deliberately by the workspace-file-as-source task, whose plan scoped `/`
as container-readiness scaffolding only. The workspace itself is implemented at
`/sessions/<sid>/home|architecture|product|builds`, and its icon rail already
links back to `/` via the "Sessions ↩" utility item. `GET /api/sessions`
(registry-only: every listed session is connected by definition) works and is
what the e2e helpers use to resolve a session id. A designed sessions page
exists in the design prototype (`harness/design/proto-workspace` →
`content/sessions.html`); an older sessions-list test contract
(`session-row`/`session-folder`/`session-version`/`test-connection` at `/`)
predates the workspace redesign, and its session-detail/run-view assertions
are unsatisfiable against the current app.

## Requirement

### Harness

- **H1 [suggested]** A Developer should be able to run the protocol suite
  without failures caused by retired pre-workspace UI surfaces: the legacy
  specs that assert the Test-connection control, the old `/sessions/<sid>`
  detail view, or run views (p1's detail-inventory half, p2's open-page
  clearing, p8 entirely) are reworked or retired to this task's contract, and
  the shared contract files (`tests/harness/helpers/ui.ts`,
  `helpers/README-testids.md`) change in the same step — no contradictory
  sources of truth left behind.

### System

(none new — Next.js App Router and the CLI-as-persistence-backend are already
established by the workspace-file-as-source brief.)

### Product

- **P1 [revealed]** A BDD System Architect should land on the harness root and
  see every CLI process currently connected to the harness — one row per
  connected session.
- **P2 [suggested]** A BDD System Architect should identify each session by
  its host project's canonical folder path (the realpath, never a symlink) as
  the row title, with the session id and the CLI version visible on the row.
- **P3 [suggested]** A BDD System Architect should enter a session's workspace
  from its row via the row's Open workspace control, landing on that session's
  workspace overview (Home).
- **P4 [revealed]** A BDD System Architect should see the list stay live
  without reloading the page: while sessions reads are succeeding, a newly
  connected CLI appears on its own and a stopped CLI disappears, each within
  at most one minute of the event — sooner is fine (~15 s is the expected
  feel given the relay's 10 s poll-lease expiry).
- **P5 [suggested]** A BDD System Architect should see a disconnected session
  simply vanish from the list — never an unreachable/disconnected marker, a
  reachability state, or a reconnect affordance.
- **P6 [revealed]** A BDD System Architect should see an explicit empty state
  when no CLI is connected — a message that no sessions are connected plus a
  short hint how to connect one.
- **P7 [suggested]** A BDD System Architect should see the home page rendered
  in the prototype's sessions design — the pre-workspace frame (no icon rail,
  no sidebar), the gradient top bar, the TrueBDD wordmark, and the session
  row-list — adapted to this task's scope: no Test-connection control, and
  the empty state styled in the same design language.
- **P8 [suggested]** A BDD System Architect should see rows hold a stable
  order across live refreshes — ordered by connection time, oldest first —
  so the list never reshuffles under the cursor as it polls.
- **P9 [suggested]** A BDD System Architect should never see the empty state
  flash while the first sessions read is still in flight — the "no sessions"
  message appears only after a successful read returns zero sessions.
- **P10 [suggested]** A BDD System Architect should see the list replaced by
  an explicit unavailable notice once sessions reads have been failing for
  more than ~30 seconds — stale rows are never silently presented as
  current — and the list return by itself on the next successful read.

## Non-goals

- No Test-connection control on the rows (explicitly dropped by the user;
  arrives with the future runs/builds work — today its result would have no
  visible surface).
- No session-detail (`/sessions/<sid>`) or run views — this task is the home
  page only.
- No special guard for clicking Open workspace in the instant after that
  session died — whatever the workspace route itself does then applies.

## Established facts

- `harness/src/app/page.tsx` is a placeholder returning 200 ("Sessions list is
  not implemented yet"); `docs/tasks/plans/workspace-file-as-source-ui.md`
  scoped `/` as scaffolding only (its §routes line 79, §528).
- `GET /api/sessions` (`harness/src/app/api/sessions/route.ts`) is
  registry-only: `{session_id, folder (canonical realpath), pid, version,
  connected_at}`; origin-gated via `browserReadAllowed`; no reachability, no
  active_run_id — every listed session is connected by definition.
- Relay poll-lease expiry is 10s (`harness/src/app/lib/relay/redis-relay.ts`
  `expiryMs: 10_000`) — a silent CLI leaves the registry ~10s after its lease
  lapses, so a UI poll of a few seconds meets the one-minute disappearance
  bound end-to-end.
- Design baseline: `harness/design/proto-workspace/app/content/sessions.html`
  — gradient-band top bar (`--gradient-spiral-soft`, the page's single
  gradient use), TrueBDD wordmark with `data-token-probe="--text-inverse"`,
  tagline "Spec-Anchored CLI — connected sessions", h1 "Sessions", `.row-list`
  rows (title = folder, meta = "session <id> — connected", version chip,
  actions). Pre-workspace frame: no rail/sidebar (`harness/design/SPEC.md`
  §1); removed sessions never render a disconnected marker (§5). NOTE: the
  prototype page still shows a Test-connection button and has no empty state —
  parity is "prototype minus Test connection, plus empty state" (P7).
- Existing test contract: `tests/harness/helpers/ui.ts` — `routes.sessions =
  "/"`, `TID.sessionRow` (+`data-session-id`, `data-folder`),
  `TID.sessionFolder` (text = realpath), `TID.sessionVersion`,
  `TID.testConnection`; documented in `helpers/README-testids.md` → "Sessions
  list (`/`)". p1 asserts realpath rendering then a session-detail inventory
  chip; p2 asserts an open page clears on disconnect; p8 asserts Test
  connection → version run → run view. Session-detail and run views do not
  exist in the current artifact, so those assertions are unsatisfiable today;
  dropping Test connection additionally makes p8's UI contract stale.
- Workspace entry route: `/sessions/<sid>/home` (`wsRoutes.home`); the
  workspace rail links back to `/` (rail-utility "Sessions" item,
  `WorkspaceShell.tsx:105`).
- Design gates: w8/w9 (token sweep + vision judge), w16 (workspace scale),
  w17 (pixel parity over committed golden crops in `tests/harness/goldens/`)
  — page goldens exist for workspace pages only; whether/how the sessions page
  joins those gates (and gets its own goldens) is decided by the implementing
  task's plan.
- Playwright projects (`tests/harness/playwright.config.ts`): protocol =
  `p[0-9]*`, workspace = `w[0-9]*`, ai = `a[0-9]*`.
