# Plan: Connect the true-bdd CLI to the Vercel-deployed harness (Redis-backed relay)

> Task brief: `docs/tasks/connect-cli-to-vercel-harness.md`. Requirements tree:
> `docs/context/requirements.md` (`# System` → Redis state backend; `# Product` →
> connect CLI to Vercel harness). Plan path: this file. E2E dir:
> `harness/tests/e2e/`. Scaffolding: repo-root `docker-compose*.yml`. Production
> code: `harness/app/`, `src/`.

## Goal

A BDD System Architect can point `true-bdd remote` at the TrueBDD harness
deployed on Vercel and drive a host project end-to-end (register, dispatch,
watch output, answer `--fix` prompts), with the harness relay's COORDINATION
state (session registry, work queue, replies) backed by Redis so a browser
request and a CLI poll served by **separate serverless invocations** rendezvous.

## Non-goals (hard guardrails)

- **No access control** — anyone with the Vercel URL can connect a CLI, dispatch
  runs, and read state (deliberate).
- **Run output, prompts, and inventory are NOT moved to Redis.** They remain
  produced and durably held on the host CLI (`<folder>/tmp/true-bdd-state.db`).
  The relay only COORDINATES: the CLI's reply body transits Redis transiently
  (as the message-bus delivery, with a short TTL) — that is coordination, not
  durable storage. Do not redesign the CLI store or move inventory/output/prompts
  into the relay.
- **No CLI protocol redesign.** The Go `remote` is already remote-ready (no auth,
  no Origin/Host headers, default TLS). The register/poll/reply wire format and
  the browser-facing status map (200 / 504 cli_timeout / 404 session_gone / 413 /
  409 / 503) stay intact.
- **No new auth, no multi-tenant isolation.**

## Current state (verified in code)

- **Relay is an in-process `globalThis` singleton** with NO database.
  - `harness/app/lib/relay/hub.ts` — `RelayHub` holds `meta` (sessions),
    `waiters` (Promise/timer plumbing), a 2 s `setInterval` expiry sweep. The
    singleton is keyed `__trueBddRelayHub__` on `globalThis` so it survives
    Next.js route-module re-eval on ONE long-lived `next start` process.
  - `harness/app/lib/relay/registry.ts` — pure in-memory `createRelay()` core
    (session registry, bounded FIFO work queues, waiter lifecycle, atomic
    expiry, epoch/token correlation). Exercised by
    `harness/tests/unit/relay-registry.test.ts`.
- **The browser AWAITS the CLI reply within the SAME HTTP request.**
  `RelayHub.request()` enqueues a work item and resolves a Promise when the CLI
  `reply()` lands — bounded by `DEADLINE_MS` (status 5 s, runDetail 5 s, mutation
  10 s, **inventory 30 s**). The CLI long-polls ~5 s (`poll()` in `hub.ts`,
    sleep-50 ms loop, 5 s hold). This in-process Promise model **cannot span two
  serverless invocations** — it relies on shared process memory.
- **Origin/Host policy is loopback-only** (`harness/app/lib/origin-policy.ts`):
  every family requires `Host` ∈ {127.0.0.1, localhost}; mutations additionally
  require a matching Origin; agent routes accept only an ABSENT Origin. A
  Vercel-deployed harness (Host = the Vercel domain) is rejected 403 by every
  route. Pinned by `p6-origin-host.spec.ts`, `p6b-agent-origin.spec.ts`, and the
    unit suite `tests/unit/origin.test.ts`.
- **API routes** are thin and already delegate to `relayHub()`: register/poll/
  reply (`app/api/agent/*`) and the session-scoped browser surface
  (`app/api/sessions/*`). `readJsonBody` enforces content-type + a STREAMED byte
  cap; reply correlation travels in HEADERS outside the capped body. All
  route-level invariants are Vercel-safe (force-dynamic, nodejs runtime) EXCEPT
  the in-process await.
- **Go `remote` is remote-ready.** `src/cmd/remote.go` defaults
  `--server http://127.0.0.1:4517`; `relay_client.go` sends no Origin/Host, uses
  a 15 s HTTP ceiling. **No env-var fallback exists** — only `--server`.
- **Stale artifacts:** `next.config.ts` lists `better-sqlite3` in
  `serverExternalPackages` and `README.md` mentions SQLite, but NO sqlite store
  ships in the relay. `ServerController` still sets a `TRUE_BDD_HARNESS_DB` env
  var (unused by the relay).
- **Vercel:** project `true-bdd-app` (team `kavoon`) is linked; `REDIS_URL` is
  provisioned in Production/Preview/Development. No docker-compose and no Redis
  code exist yet.

## Target state

- A **Redis-backed relay** is the single relay implementation: session registry,
  work queue, and replies live in Redis (read/written on every request), so no
  request depends on in-process state. The `globalThis` singleton, the in-memory
  `createRelay()` core, and the in-process Promise/timer waiter are REMOVED — but
  the `Relay` INTERFACE is PRESERVED so the existing unit suite is PORTED, not
  retired (Codex r1 #16).
- **The browser-facing contract is unchanged**: a read/mutation returns 200 with
  the CLI reply body, 504 cli_timeout, 404 session_gone, 413, 409, or 503. The
  server-side wait is capped to fit a serverless function (`maxDuration`, Codex
  r1 #14) AND stays below the Go client's 15 s ceiling. The dispatch route STILL
  awaits the CLI's transactional reply (201 created / 200 dedup / 409 conflict /
  400 invalid) within that shortened deadline — that transactional reply IS the
  "acknowledgement" in the brief; there is no new 202/pending state. Run
  PROGRESS is obtained through later `run_detail` reads, as today (Codex r1 #11).
- **Replies are consume-once transit, NOT a cache.** A CLI reply is written to
  `tb:reply:{work_id}` with a short TTL and DELETED on read by the waiting
  request. The reply body transits Redis exactly once (same as today's in-process
  Promise) — it is never re-served to a later request. There is NO
  `tb:latest:{view}` result cache (Codex r1 #2) and **NO cross-request query
  dedupe** (Codex r2 #1 showed "concurrent GETs consume one reply" contradicts
  consume-once — `GETDEL` admits exactly one consumer). Instead: **one HTTP
  request = one work_id = one consume-once reply** (a clean one-request/one-work
  state machine). The amplification cost (a fresh query per browser poll tick) is
  acceptable — the CLI serves each quickly, and this is the simplest model with
  no attach/consume races.
- **Explicit timeout/abort lifecycle + work state machine** (Codex r2 #2/#3/#4,
  r3 #3): each work item is a per-work Redis record with an explicit state —
  `queued → delivered → replied`, `queued → cancelled`, and
  `delivered → orphaned → (expired | late-replied)` — plus per-state TTLs. When
  the browser deadline/abort fires, an atomic Lua transition handles queued vs
  delivered differently — QUEUED work moves to `cancelled` (a read is cancelled;
  a queued mutation is dropped, no store-and-forward); DELIVERED work moves to
  `orphaned` (the CLI may be executing it) and accepts ≤1 late reply then retires
  WITHOUT storing the body (capacity is reclaimed by the terminal-marker TTL).
  **`timeout` and `reply` are COMPETING Lua transitions** on the same work record
  (exactly one wins) — never app-level read-then-delete. Replies are consumed
  atomically via `GETDEL`/Lua (not separate GET + DELETE). A request that already
  returned 504 CANNOT consume its later reply — reads are simply RE-EXECUTED on
  the next poll (fresh work_id, fresh read); mutations rely on CLI-side
  idempotency (`client_token` dedup, answer first-wins) exactly as today (r2 #3).
  The orphaned late-reply body is discarded, never re-served.
- **Atomicity is preserved with Lua scripts** (Codex r1 #3/#4/#5, r2 #4): register,
  enqueue, poll (queued→delivered), reply (authenticate epoch+token+work_id,
  consume-once, reject double/cross-session), timeout/cancel (above), and a
  lastSeen-based sweep that atomically fails waiters + drops queued mutations +
  cleans the session index + prevents a late poll from reviving the session.
  Bounded terminal markers retain deterministic 404/409/413/502 outcomes. Every
  state transition is one script (no app-level multi-command races).
- **Serverless-fit wait, deadlines NOT silently lowered** (Codex r3 #1): the
  agent `poll` ACTIVELY observes newly enqueued work via a Redis blocking primitive
  (`BLPOP`) or pub-sub wakeup + authoritative Lua claim — it does NOT wait for the
  next ~5 s poll cycle (otherwise worst-case work sits ~5 s, leaving under 3 s of
  an 8 s read deadline; status/runDetail are 5 s). The browser server-side wait in
  `request()` is `min(operation deadline, maxDuration − slack)` — on local
  `next start` the existing deadlines (5 s/10 s/30 s) STAY (no serverless limit
  there); on Vercel each route exports a `maxDuration` above one ~5 s poll cycle
  + processing/network slack, and the wait is clamped under BOTH that and the Go
  client's 15 s ceiling. Inventory stays 30 s on local; it is clamped only on
  Vercel — not lowered globally without evidence (existing tests assert 30 s).
- **Origin policy admits a deployed mode selected by an ENV signal**
  (`HARNESS_DEPLOYED=1`, NOT inferred from the request `Host` — Codex r1 #12:
  Host-inference is spoofable). In deployed mode: GETs allowed; mutations require
  `Origin` to match the EFFECTIVE public host — derived from trusted Vercel
  forwarded headers (`x-forwarded-host`/`x-forwarded-proto`) OR an explicit
  configured public origin, NOT the raw `Host` (Codex r2 #12: raw-Host compare
  breaks behind proxies and on preview/custom/alias domains). This is a CSRF
  request-integrity guard, NOT access control (consistent with the no-auth
  non-goal); agent routes accept only an ABSENT Origin (the Go client sends none).
  Loopback (non-deployed) keeps the existing discipline byte-for-byte, so P6/P6b
  + origin unit tests pass.
- **`docker-compose.yml`** runs Redis for local dev and the e2e/unit suites; the
  harness reads `REDIS_URL` (Vercel provisioned, local from compose).
- **`true-bdd remote` accepts a set-once env var** (`TRUE_BDD_SERVER`); the
  `--server` flag overrides, the default stays local loopback. Precedence uses
  `Flags().Changed("server")` (Cobra installs the loopback value as the default,
  so mere "flag at default" is ambiguous — Codex r1 #13).
- Run output, prompts, and inventory remain produced and durably held on the host
  CLI (`<folder>/tmp/true-bdd-state.db`). The CLI reply body transits Redis only
  as the transient, consume-once coordination message.

## End-to-end test cases (LEAD — `harness/tests/e2e/`, protocol project)

Each case is phrased so it **fails if the required behavior is absent**. They use
the hidden deterministic `version` / `prompt-probe` commands (no `claude`), so
they run under the `protocol` project (`p*`).

### p17-cross-instance-rendezvous.spec.ts — THE serverless proof
Two `next start` instances (A, B) on separate ports, **sharing one Redis**
(brought up by global-setup via docker-compose; both servers get the same
`REDIS_URL`; per-test key prefix isolates state — see Scaffolding). The test
pins register/enqueue/poll/reply to SPECIFIC instances with deterministic
routing (Codex r1 #7 — a partially-in-process relay must NOT pass):

- **Assert 1 (registry crosses instances):** a raw agent `register` against
  **B** creates the session; `GET /api/sessions` on **A** lists that exact
  session (same `session_id`, `pid`). Reverse (register on A, list on B) too.
  → Fails if the registry is in-process.
- **Assert 2 (work queue + reply cross instances, exact-reply):** RETAIN the
  browser request as an unresolved promise — `const pending = apiA.dispatchRunResponse(...)`
  (Codex r2 #6 — typed helpers await immediately). Then `poll` on **B** returns
  that exact `work_id` + payload; `reply` on **B** with the CLI result; `await
  pending` resolves to 200/terminal `ok`. A SECOND `poll` on B after the reply
  must NOT re-deliver the same `work_id` (no double-delivery). → Fails if
  work/replies live in either process's memory.
- **Assert 3 (read query crosses instances):** a `session_detail` read on **A**
  is served by the CLI polling **B**; the inventory in the reply (produced on the
  host) is returned to A. → Fails if a read's reply cannot be consumed by a
  different instance than the CLI replied to.
- **Assert 4 (capacity + rejection cross instances):** with the per-session
  queue cap filled on A, a further enqueue on **B** → 503 capacity; a
  bad-token/stale-epoch reply on B → 4xx. → Fails if capacity/correlation is
  in-process.

### p18-relay-restart-keeps-coordination-state.spec.ts — cold-start survival
One server, one Redis. Use a raw registered agent that **does NOT poll** (Codex
r1 #8 — a live CLI would claim work before the restart and mask nondurability):

- **Assert 1 (registry survives a memory wipe):** register (the raw agent path
  RETURNS `connection_epoch` + `capability_token`), then `server.restart()`;
  immediately `GET /api/sessions` STILL lists the session (same `session_id` +
  `pid`) WITHOUT waiting for a re-register, AND a raw agent `poll` on the
  restart with the SAME epoch+token SUCCEEDS (proves credentials persisted in
  Redis — the epoch/token are NOT exposed by the browser session summary, so they
  are asserted via the raw agent path; Codex r2 #9). → Fails under the in-memory
  model (restart wipes the registry → 404 → re-register with a NEW epoch).
- **Assert 2 (queued work survives, no pre-restart ID capture):** START the
  browser dispatch request on the pre-restart server but DO NOT let any agent
  poll; `server.restart()` BEFORE any poll (the `work_id` is opaque pre-poll —
  Codex r2 #5); THEN the agent `poll` after restart returns the pre-restart
  dispatch PAYLOAD (command + `client_token`), delivered ONCE. → Fails if queued
  work is wiped with the process.

> **P7 must be REWRITTEN by the test-author** (Codex r1 #6, r2 #9): today
> `p7-restart-idle.spec.ts` explicitly waits for the remote to get 404 and
> re-register after a relay restart. Under Redis the registration SURVIVES. The
> new P7 is a pure black-box test: across a Next restart, the session NEVER
> disappears from `GET /api/sessions` (no re-register wait), the SAME
> `session_id`+`pid` remain, and history is queryable immediately. Epoch/token
> persistence is tested separately via the raw agent (register returns them; a
> post-restart poll with the same creds succeeds). A SEPARATE case keeps genuine
> expiry-driven re-registration (no poll past `expiryMs`). p18 covers the
> queued-work survival invariant.

### p19-deployed-origin-policy.spec.ts — Vercel-readiness of the policy
Deployed mode is ENV-driven (`HARNESS_DEPLOYED=1`), so the test starts a server
with that env set and uses raw HTTP with a non-loopback `Host`. It exercises
EVERY route family AND the spoofing guard (Codex r1 #12, r2 #12 — the effective
host is derived from forwarded headers / configured origin, not raw Host):

- **GET** with non-loopback Host → 200 (not 403).
- **Agent register/poll/reply** with ABSENT Origin + non-loopback Host →
  accepted-class (the Go client sends no Origin).
- **Browser mutation** with `Origin` matching the non-loopback Host →
  accepted-class; with foreign / `null` / missing Origin → 403 (CSRF guard kept).
- **Spoof guard:** against a LOOPBACK server (no `HARNESS_DEPLOYED`), a request
  with `Host: true-bdd-app.vercel.app` is STILL rejected 403 — proving deployed
  mode is not inferable from the Host header.
→ Each fails under the current loopback-only policy OR under a naive
Host-inferred relaxation.

### p20-cross-instance-fix-loop.spec.ts — the --fix workflow across instances
Browser reads/mutations pinned to **A**, CLI poll/reply pinned to **B**, sharing
Redis (Codex r1 #9 — p17 only runs `version`; the Product --fix requirement is
otherwise uncovered). Drive the hidden `prompt-probe` driver (choice → clarify →
freetext, deterministic, no `claude`). For a DETERMINISTIC consume-once oracle
(Codex r2 #7 — mere prompt progression tolerates duplicate delivery), use the
RAW agent protocol per answer: RETAIN each browser answer request as an
unresolved promise, `poll` its exact answer `work_id` ONCE on B, `reply`, then
assert a REPEATED `poll`/`reply` cannot redeliver or re-resolve it; then assert
the run advances to the next prompt kind. The real-CLI fix-loop end-to-end
behavior stays in the `ai` suite.

→ Fails if prompt/answer coordination (work queue + replies) is in-process OR if
a reply is double-delivered.

### p21-incremental-output.spec.ts — output streams before completion
Use `prompt-probe`'s OWN prompt as the deterministic barrier (Codex r2 #8 — a
sleep-based barrier flakes; the prompt is a real sync point), split across
instances A/B:

- the pre-prompt output (chunk A) is visible in the run detail while the run is
  still ACTIVE (non-terminal, `pending_prompt` present); submit the answer; then
  the post-answer output (chunk B) appears and the run advances.
→ Fails if output is only served at completion (a relay that buffers the whole
reply instead of streaming incremental run_detail reads from the CLI store).

### p22-late-reply-run-recovery.spec.ts — the delivered→timeout→late-commit path
The recovery path that the consume-once + orphan model exists for (Codex r3 #2 —
no current test covers it; P13 resubmits after success, not after a lost
response). Uses the raw agent protocol against one server:

- a `dispatch` is enqueued and CLAIMED by the agent (state `delivered`); the
  browser `request()` is DELIBERATELY let past its deadline → 504 (no run visible
  yet). Then the agent POSTS the late CLI reply (the run commits); assert the
  relay does NOT retain the late reply body (a follow-up read cannot fetch it).
- the browser RE-DISPATCHES with the SAME `client_token` → 200 with the ORIGINAL
  `run_id`, and the project has EXACTLY ONE run for that token (CLI-side dedup
  recovers the committed run the 504'd request never saw).
- the equivalent LATE-ANSWER case: an `answer` delivered then timed-out, a late
  reply discarded, a repeat answer with the same value → accepted (first-wins),
  the run still advances once.
→ Fails if a late reply is retained/re-served OR if a run committed after a 504
cannot be recovered (the no-store-and-forward + idempotent-recovery contract).

### Existing suite still green (Harness requirement)
The p1–p16 protocol specs must pass UNCHANGED in behavior against the Redis-
backed relay (test-author only retrofits scaffolding + rewrites P7). The vitest
`relay-registry` suite is PORTED to a Redis-backed integration suite (same
interface, incl. two-client concurrency for atomicity/double-reply), NOT retired
(Codex r1 #16); the `origin` unit suite gains deployed-mode cases.

## Startup scaffolding (test-author creates; EMPTY of production logic)

Per `paths.md`, the test-author may create repo-root compose/Dockerfile and new
service dirs, left empty for the coder. Here the only scaffolding is Redis
plumbing so services start:

- **`docker-compose.yml`** (repo root) — one `redis:7-alpine` service, host port
  `6379:6379`, a named volume, a `redis-cli ping` healthcheck. No app service,
  no production logic. (A `docker-compose.test.yml` override MAY isolate a
  developer's local Redis from the suite.)
- **E2E wiring (test-owned, Codex r1 #15, r2 #10 for isolation):** `global-setup.ts`
  brings the Redis container up ONCE and waits on the healthcheck (never a fixed
  `sleep`); `global-teardown.ts` stops compose ONCE (per-test `compose down`
  would kill Redis other sequential tests reuse). Each test gets ONE UNIQUE
  `REDIS_KEY_PREFIX` shared by that test's A/B servers (prefix EVERY key incl.
  the session index, epoch counter, and capacity counters — NO logical-DB
  allocation, which has a small fixed range and does not isolate Lua counters
  unless every key is prefixed); scoped teardown deletes only that prefix.
  `ServerController` drops the unused `TRUE_BDD_HARNESS_DB`, injects
  `REDIS_URL` + `REDIS_KEY_PREFIX` into each `next start`, and exposes a second
  `next start` port for p17/p20/p21 (two instances, one Redis); both ports are
  RESERVED before spawn to avoid bind races. These are harness/test changes.

## Implementation (production code — coder; makes the tests pass)

### `harness/app/lib/relay/` — Redis-backed relay (replaces hub.ts + registry.ts core)
- Add a Redis client (from `REDIS_URL`; lazy-connected, reused across warm
  invocations). Add the runtime dep in `harness/package.json` (`ioredis` or
  `redis`); **do NOT touch `scripts`.** Key prefix from `REDIS_KEY_PREFIX` (test
  isolation).
- **Preserve the `Relay` interface** (Codex r1 #16); provide ONE Redis-backed
  implementation. The unit suite is PORTED to it (below).
- **Atomic operations via Lua** (Codex r1 #3/#4/#5) — never plain BLPOP + app-
  level checks:
  - **register** = one script: bump a persistent epoch, rotate the token,
    atomically invalidate prior-epoch in-flight work (fail their waiters, drop
    queued mutations), write the session hash + add to the session index.
  - **enqueue** = script: expireIfStale → capacity check (in-flight count incl.
    `delivered`) → append work_id to `tb:work:{sid}` (FIFO) + per-work record.
  - **poll** = ACTIVELY waits on the session's work list via a Redis blocking
    primitive (`BLPOP`) or pub-sub wakeup, then an authoritative Lua claim that
    expireIfStale (reject, never revive), renews lastSeen, and moves the oldest
    QUEUED item to DELIVERED (Codex r3 #1 — must observe new work promptly, NOT
    wait for the next ~5 s poll cycle). Capacity keeps counting DELIVERED until
    replied.
  - **reply** = script: authenticate epoch+token+work_id against the DELIVERED
    (or ORPHANED) record (reject unknown/cross-session/double/not-delivered/
    stale), enforce the byte budget, then WRITE the reply to `tb:reply:{work_id}`
    and consume the waiter (retire to a bounded terminal marker).
    `failOverCap`/`failInvalid` write a correlated 413/502 marker the SAME way.
  - **sweep** = script driven by `lastSeen` (NOT per-key TTL): atomically remove
    expired sessions, fail their waiters, drop queued mutations, clean the index.
  - **timeout/cancel** = script (Codex r2 #2/#4, r3 #3): on browser deadline/
    abort, QUEUED work → `cancelled` (queued mutation dropped — no store-and-
    forward); DELIVERED work → `orphaned` (accepts ≤1 late reply, then retired
    WITHOUT storing the body; capacity reclaimed by terminal TTL). **`timeout`
    and `reply` are competing Lua transitions** on the same work record (one
    wins); replies are consumed atomically via `GETDEL`/Lua (not GET then DELETE).
- **Replies are consume-once** (Codex r1 #1/#2, r2 #1): a browser `request()`
  enqueues work, then waits for `tb:reply:{work_id}` to appear and consumes it
  atomically (`GETDEL`/Lua); the TTL is a backstop only. **One HTTP request = one
  work_id = one consume-once reply** — NO cross-request query dedupe (r2 #1
  proved dedupe + consume-once are mutually exclusive). A late CLI reply for an
  already-504'd work_id is discarded — a NEW browser request gets a NEW work_id
  and its OWN reply; reads are RE-EXECUTED on each poll (fresh read), mutations
  are idempotent via CLI-side `client_token`/answer-first-wins (r2 #3). A run
  COMMITTED after the browser 504'd is recovered by re-dispatching the SAME
  `client_token` (CLI dedup → original `run_id`) — covered by p22.
- **Serverless-fit wait, deadlines NOT globally lowered** (Codex r2 #11, r3 #1):
  `request()` waits `min(operation deadline, maxDuration − slack)`; on local
  `next start` the existing 5 s/10 s/30 s deadlines STAY (no serverless limit);
  on Vercel each route exports a `maxDuration` above one ~5 s poll cycle +
  processing/network slack, and the wait is clamped under that AND the Go
  client's 15 s ceiling. `request()` returns 504 cli_timeout on expiry (the
  browser's `usePoll` retries next tick); Redis wait ops are abortable via the
  request `signal`.
- **Remove** the `globalThis` singleton, the in-process `waiters` Map, the
  `setInterval` sweep, the `sleep`-loop poll, and the pure in-memory
  `createRelay`. Keep route-facing method names/shapes so route handlers stay
  thin.

### `harness/app/lib/origin-policy.ts` — env-driven deployed mode (Codex r1 #12, r2 #12)
- `deployedMode = process.env.HARNESS_DEPLOYED === "1"` (NOT inferred from the
  request Host — Host-inference is spoofable and would defeat loopback CSRF
  protection on local dev).
- Loopback (non-deployed): the EXISTING discipline runs byte-for-byte.
- Deployed: GETs allowed; mutations require `Origin` to match the EFFECTIVE
  public host — derived from trusted Vercel forwarded headers
  (`x-forwarded-host` / `x-forwarded-proto`) OR an explicit configured public
  origin (`HARNESS_PUBLIC_ORIGIN`), NOT the raw `Host` (raw-Host compare breaks
  behind the Vercel proxy and on preview/custom/alias domains — r2 #12). This is
  a CSRF request-integrity guard, NOT access control. Agent routes accept only
  an ABSENT Origin (CLI sends none). Foreign / `null` / missing Origin mutations
  → 403; spoofed-Host on a loopback (non-deployed) server → 403 (the p19 guard).

### Route `maxDuration` (Codex r1 #14)
- Each relay route exports `export const maxDuration = <N>` (Vercel) chosen
  above the server-side wait + cold-start + Redis-latency slack, and the relay
  wait is tuned to sit comfortably below MIN(`maxDuration`, 15 s client ceiling).

### `harness/next.config.ts` — drop the stale sqlite carve-out
- Remove `serverExternalPackages: ["better-sqlite3"]` (no sqlite ships; the Redis
  client needs no native-addon carve-out on Vercel).

### `src/cmd/remote.go` — set-once env var (Codex r1 #13)
- Read `TRUE_BDD_SERVER`; resolve precedence with `cmd.Flags().Changed("server")`
  (the loopback value is the flag DEFAULT, so "flag at default" alone is
  ambiguous) — explicit `--server` wins; else env if set; else loopback default.
- Add Go command tests: unset env, env-only, explicit flag overrides env,
  empty/invalid env, HTTPS URL normalization. No other remote-client changes
  (the HTTP client is already remote-ready; TLS is default).

### Vercel / env
- `REDIS_URL` is already provisioned on Vercel; set `HARNESS_DEPLOYED=1` there
  too. Locally both come from docker-compose / `.env.local`.

## Codex rounds

### Round 1 — prompt `./tmp/codex-plan-r1.md`; answer `./tmp/codex-plan-r1.md`
16 findings, all KEPT (each composite ≥7 + all four gates pass). Gates key:
C=Correctness, E=Evidence, S=Scope fit, R=Regression risk.

| # | Area | Composite | C/E/S/R | Keep/Skip | Applied |
|---|---|---|---|---|---|
| 1 | Late CLI reply orphaned by per-retry work_id | 9 | ✓/✓/✓/✓ | KEEP | replies consume-once; new request ⇒ new work_id, never shares another's reply |
| 2 | `tb:latest:{view}` cache re-serves inventory/output (non-goal) | 9 | ✓/✓/✓/✓ | KEEP | removed the result cache; dedupe CONCURRENT in-flight queries only |
| 3 | BLPOP insufficient (delivered state, capacity, double-reply) | 9 | ✓/✓/✓/✓ | KEEP | Lua scripts for queued→delivered + reply auth/consume |
| 4 | Per-key TTL can't reproduce atomic sweep | 9 | ✓/✓/✓/✓ | KEEP | explicit lastSeen + Lua sweep; clean session index |
| 5 | Epoch invalidation not atomic across Redis | 8 | ✓/✓/✓/✓ | KEEP | register = one script (epoch bump, token rotate, invalidate old work) |
| 6 | P7 explicitly tests 404→re-register (not just stale comment) | 9 | ✓/✓/✓/✓ | KEEP | P7 rewritten by test-author; p18 covers survival; expiry/re-register kept separate |
| 7 | p17 passes with partially-in-process state | 9 | ✓/✓/✓/✓ | KEEP | p17 forces register/enqueue/poll/reply onto specific instances; exact-reply + reverse + capacity/rejection |
| 8 | p18 doesn't prove queued-work survival (live CLI masks it) | 9 | ✓/✓/✓/✓ | KEEP | p18 uses a no-poll agent + known token + asserts exact pre-restart work_id |
| 9 | No cross-instance --fix loop coverage | 9 | ✓/✓/✓/✓ | KEEP | added p20 (browser A / CLI B, all three prompt kinds, answer consumed once) |
| 10 | No incremental-output coverage | 8 | ✓/✓/✓/✓ | KEEP | added p21 (chunk A visible while active, chunk B after barrier) |
| 11 | "Immediate ack" vs the 8s wait contradiction | 8 | ✓/✓/✓/✓ | KEEP | clarified: CLI transactional reply IS the ack; no 202; progress via run_detail |
| 12 | Deployed-mode-from-Host is spoofable (CSRF hole) | 9 | ✓/✓/✓/✓ | KEEP | deployed mode is ENV-driven; mutations still Origin-match; spoofed-Host guard in p19 |
| 13 | CLI env var needs `Flags().Changed` + tests | 8 | ✓/✓/✓/✓ | KEEP | `Changed("server")` precedence + Go command tests |
| 14 | No `maxDuration`; 8s + cold start vs 15s client ceiling | 8 | ✓/✓/✓/✓ | KEEP | routes export maxDuration; wait tuned under min(maxDuration, 15s) |
| 15 | Shared Redis isolation/flakiness | 9 | ✓/✓/✓/✓ | KEEP | per-test key prefix/DB; healthcheck wait; ports reserved; compose only in global teardown |
| 16 | Retiring createRelay discards unit coverage | 9 | ✓/✓/✓/✓ | KEEP | Relay interface preserved; relay-registry suite PORTED to Redis integration (incl. two-client concurrency) |

Summary applied: redesigned reply lifecycle (consume-once + concurrent-query
dedupe instead of result cache); mandated Lua atomicity for register/enqueue/
poll/reply/sweep; env-driven deployed origin policy with CSRF guard kept;
strengthened p17/p18; added p20/p21; P7 to be rewritten; `maxDuration` + CLI
`Changed("server")` + tests; per-test Redis isolation; preserve+port the unit
suite.

### Round 2 — prompt `./tmp/codex-plan-r2.md`; answer `./tmp/codex-plan-r2.md`
12 findings, all KEPT (composite ≥7, all gates pass). Re-derived against the
round-1-applied plan; did not assume r1 fixes were correct.

| # | Area | Composite | C/E/S/R | Keep/Skip | Applied |
|---|---|---|---|---|---|
| 1 | consume-once vs dedupe contradiction (GETDEL = one consumer) | 9 | ✓/✓/✓/✓ | KEEP | dropped cross-request query dedupe; one-request/one-work/one-reply state machine |
| 2 | timeout/abort lifecycle for queued vs delivered undefined | 9 | ✓/✓/✓/✓ | KEEP | added atomic timeout/cancel Lua script; delivered→expiring orphan, ≤1 late reply, no body stored |
| 3 | 504'd request can't consume later reply; hidden retry semantics | 9 | ✓/✓/✓/✓ | KEEP | stated: reads re-executed, mutations idempotent via client_token/answer-first-wins; orphan body discarded |
| 4 | missing timeout/cancel (and dedupe) Lua scripts | 8 | ✓/✓/✓/✓ | KEEP | enumerate keys/transitions; every transition one script (dedupe removed) |
| 5 | p18 "capture work_id" not implementable (only poll exposes it) | 9 | ✓/✓/✓/✓ | KEEP | p18 restarts BEFORE any poll; asserts pre-restart payload delivered once |
| 6 | p17 needs explicit retained-promise sequencing | 8 | ✓/✓/✓/✓ | KEEP | `const pending = apiA...Response()`, poll/reply via B, then `await pending` |
| 7 | p20 "consumed once" lacks deterministic oracle | 8 | ✓/✓/✓/✓ | KEEP | raw-agent oracle per answer; assert repeat poll/reply can't redeliver |
| 8 | p21 timing-only assertion flaky | 8 | ✓/✓/✓/✓ | KEEP | use prompt-probe's prompt as the barrier; no sleeps |
| 9 | epoch/token not in GET /api/sessions; P7 is black-box | 9 | ✓/✓/✓/✓ | KEEP | P7 asserts session survives visibly; epoch/token tested via raw agent only |
| 10 | REDIS_KEY_PREFIX/DB underspecified; A/B share namespace | 8 | ✓/✓/✓/✓ | KEEP | one prefix per test shared by A/B, every key prefixed, no logical DB |
| 11 | no concrete wait/maxDuration budgets | 8 | ✓/✓/✓/✓ | KEEP | ~8s wait (>5s poll cycle, <15s client ceiling); add cross-poll-boundary + intentional-504 tests |
| 12 | Origin==raw Host fragile behind Vercel/proxies/preview domains | 8 | ✓/✓/✓/✓ | KEEP | effective host from forwarded headers / HARNESS_PUBLIC_ORIGIN; preview-domain cases |

Summary applied: simplified to a one-request/one-work/one-reply model (no
dedupe); added atomic timeout/cancel lifecycle + explicit retry semantics;
strengthened p17/p18/p20/p21 oracles (retained promise, no pre-poll ID capture,
raw-agent answer oracle, prompt-as-barrier); P7 made black-box with epoch/token
tested via raw agent; prefix-only Redis isolation; concrete ~8s wait budget;
effective-host origin derivation.

### Round 3 — prompt `./tmp/codex-plan-r3.md`; answer `./tmp/codex-plan-r3.md`
3 findings, all KEPT (composite ≥7, all gates pass). Final round — loop stops
here (cap is 3).

| # | Area | Composite | C/E/S/R | Keep/Skip | Applied |
|---|---|---|---|---|---|
| 1 | poll must ACTIVELY observe new work (BLPOP/pub-sub); don't lower inventory 30s→8s without evidence | 9 | ✓/✓/✓/✓ | KEEP | poll = BLPOP/pub-sub wakeup + Lua claim (no wait-for-next-poll); wait = min(deadline, maxDuration−slack); local keeps 30s; inventory clamped only on Vercel |
| 2 | no test for delivered→timeout→late-commit run_id recovery | 9 | ✓/✓/✓/✓ | KEEP | added p22 (claim→504→late reply discarded→re-dispatch same client_token→original run_id, one run; + late-answer first-wins) |
| 3 | "orphan" not an explicit state; GET+DELETE isn't atomic consume-once | 9 | ✓/✓/✓/✓ | KEEP | explicit per-work states queued/delivered/replied/cancelled/expired/orphaned + TTLs; timeout vs reply = competing Lua transitions; consume via GETDEL/Lua |

Summary applied: poll actively observes new work (no next-cycle wait);
deadlines kept on local / clamped only on Vercel (inventory NOT silently
lowered); added p22 covering the late-commit recovery path; made the work state
machine explicit with competing timeout/reply Lua transitions and atomic
consume.

**Loop complete: 3/3 rounds run; 31 findings total, 31 kept, 0 skipped.**

## Test-author Codex rounds (e2e layer)

Run by the test-author agent on the e2e test layer + scaffolding (distinct from
the planner rounds above). Gates: C=Correctness, E=Evidence, S=Scope fit,
R=Regression risk.

### Test-author round 1 — prompt `./tmp/codex-test-author-r1.md`; answer `./tmp/codex-test-author.md`

6 findings: 5 KEPT, 1 SKIP (composite ≥7 + gates pass for keeps).

| # | Area | Composite | C/E/S/R | Keep/Skip | Applied |
|---|---|---|---|---|---|
| 1 | p18 sleep(1000) enqueue barrier | 7 | ✓/✓/✓/✓ | SKIP (fix couples to unbuilt Redis key scheme) | added a documenting comment; route enqueues synchronously, so the wait is the least-coupling barrier |
| 2 | p17 omits cross-instance capacity proof | 9 | ✓/✓/✓/✓ | KEEP | Assert 4a: `HARNESS_MAX_PER_SESSION_QUEUE=2`, fill via 2 claimed reads, overflow → 503 on A and B |
| 3 | p19 missing agent poll/reply routes | 9 | ✓/✓/✓/✓ | KEEP | poll → 204 + reply → 409 unknown_work (deployed Host, absent Origin) |
| 4 | p19 raw-Host "matching Origin" case wrong | 8 | ✓/✓/✓/✓ | KEEP | raw-Host+matching-Origin (no forwarded) → 403; forwarded-headers admission retained |
| 5 | p20 dispatches fix:false but claims the --fix workflow | 8 | ✓/✓/✓/✓ | KEEP | dispatch fix:true + asserts `getRun().fix === true` |
| 6 | p21 proves event-count grew, not an output event | 9 | ✓/✓/✓/✓ | KEEP | asserts `type==="output"` events by seq before/after the prompt barrier |

### Test-author round 2 — prompt `./tmp/codex-test-author-r2.md`; answer `./tmp/codex-test-author-r2.md`

4 findings: 3 KEPT, 1 SKIP.

| # | Area | Composite | C/E/S/R | Keep/Skip | Applied |
|---|---|---|---|---|---|
| 1 | p17 capacity fill arrival-order race | 7 | ✓/✓/✓/✓ | KEEP | claim each fill read via a held raw-agent poll (queued→delivered, deterministic) before overflow |
| 2 | p19 reply `not.toBe(403)` too weak | 8 | ✓/✓/✓/✓ | KEEP | deterministic `toBe(STATUS.conflict)` (route maps non-stale rejection → 409) |
| 3 | p22 fabricates run_id/inventory; "exactly one run" unprovable raw-agent | 9 | ✓/✓/✓/✓ | KEEP | restructured: Assert 1 raw-agent orphan/no-retain (2nd late reply → 409); Assert 2 real-remote token dedup → 1 run; dropped fabricated late-answer (p20 covers it) |
| 4 | codex-loop.md modified outside scope | 7 | ✓/✓/✗/✓ | SKIP | false attribution — the planner modified it (workflow log 2026-07-31T20:40Z), not the test-author |

Summary applied: deterministic p17 capacity fill (claim-based); deterministic p19
reply 409; p22 made honest (raw-agent orphan/no-retain + real-remote dedup, no
fabricated ids). Suite verified RED: all 7 specs (p7 rewrite + p17–p22) FAIL
cleanly on the absent Redis behavior; no crashes/collection errors; global-setup
brings Redis up, servers boot.

## Challenges

No blocker required user escalation. Significant events recorded for reviewer context:

- **Reviewer agent could not spawn** (process deviation): the `implement-task-reviewer`
  agent (the only one carrying the Playwright MCP tool set) failed at spawn with
  "Prompt is too long" on all attempts (Fable AND an Opus override). Root cause: the
  combined size of this repo's system context (large CLAUDE.md + skills registry) plus
  the ~30 Playwright MCP tool schemas exceeds the spawn limit; the Opus planner and
  Sonnet coder (no Playwright tools) spawned fine. Resolution: the orchestrator ran
  Phase 3 inline — an independent read-only Codex review round (this file's Workflow log)
  plus a live Vercel smoke test. The DoD "Codex loops ran" item is met by the planner's
  3 rounds + this inline reviewer round; the test-author's own loop was deferred to this
  round (its first run blew context mid-loop; a tight resume confirmed the RED).

- **p2-reachability-abandon regression** (found by the orchestrator's independent
  full-suite run, NOT by the coder's spot-check of p1/p6/p6b/p11). The spec is
  byte-identical to baseline and was green before; it is a coder-attributable regression.
  - **Failing assertion:** after SIGSTOP'ing the remote, the already-open session page
    never renders `unavailable-state` within 20s (line 83), even though the backend
    correctly evicts the session — `listSessions` excludes it and direct calls to
    `/api/sessions/{id}`, status, and run detail all return 404 `session_gone` (lines
    70–78 pass).
  - **Root cause (confirmed by the coder via the Playwright trace/error-context):** the
    Redis `SWEEP_SCRIPT` evicted the dead session's hash + index entry but left its
    in-flight browser work items trapped in `RedisRelay.request()`'s `getdel` wait loop
    (up to the 30s inventory deadline). Since `usePoll` schedules the next tick only
    after the fetch resolves, the page never re-issued a poll to hit the now-404
    `hasSession` gate — so it sat on its last live value (~30s) past the 20s threshold.
    Direct API calls 404'd promptly because they check `hasSession` at the route top and
    never enter the wait loop.
  - **Fix (production code, `harness/app/lib/relay/redis-relay.ts` only):** `SWEEP_SCRIPT`
    now also writes a consume-once `{"status":404,"body":{"error":"session_gone"}}` marker
    to `<p>:reply:<wid>` for each swept session's in-flight `queued`/`delivered` work,
    WITHOUT flipping work state or touching inflight sets. The trapped `getdel` picks it
    up within ~50ms → page poll returns 404 → `session_gone` (terminal) → renders
    `unavailable-state` at ~SIGSTOP+10s. State-preserving marker keeps p22's
    `delivered→orphaned→late_replied` path intact (an earlier `state=expired` attempt
    broke p22 with 409 `not_delivered`).
  - **Verdict:** CODE-FIX (no test change). Re-verified by the orchestrator: full
    protocol suite 50/50 green; off-limits + package-`scripts` clean.

## Workflow log

(filled by the orchestrator across phases.)

- 2026-07-31T20:40Z — Baselines captured to `tmp/implement-baselines/`: production
  manifest (257 files), off-limits manifest (249 files), package-`scripts` snapshot,
  change-surface content copy, git HEAD `4e5d245` (clean tree).
- 2026-07-31T20:40Z — Phase 1.1 DONE. Planner (Opus) wrote this plan; 3 Codex rounds,
  31 findings, 31 kept (composite ≥7 + all gates). Test list: p17–p22 new + p7 rewrite
  + ported unit suite.
- 2026-07-31T20:40Z — Decision: codified the Codex status-line format in
  `references/codex-loop.md` — every round now reports `Codex round N out of 3 is
  running in the background with prompt: <path>. I'll wait for it to complete.` Applies
  to all four agents going forward.
- 2026-07-31T21:44Z — Phase 1.2 DONE. Test-author (Opus) wrote p17–p22 + p7 rewrite +
  helpers + `docker-compose.yml` scaffolding. Its first run terminated early (context
  grew too large mid-Codex-loop); a tightly-scoped RESUME confirmed service readiness
  (Redis via docker-compose boots; `next build` green) and a VALID RED: all 7 specs
  fail on genuine assertions about the absent Redis-backed relay (0 passed / 7 failed,
  exit 1, no collection errors). Reproduce block captured. Test-author's own Codex loop
  deferred to the Phase-3 reviewer.
- 2026-07-31T21:44Z — Phase 1.3 DONE. Production-only manifest diff clean — test-author
  touched no production code (`src/`, `templates/`, `true-bdd/`, `harness/app/`).
- 2026-07-31T21:44Z — Phase 2 START. Coder "before" snapshots: off-limits manifest
  (257 files) and package-`scripts` (unchanged from baseline) saved to
  `tmp/implement-baselines/`.
- 2026-07-31T22:26Z — Phase 2 DONE. Coder (Sonnet) implemented the Redis relay
  (`redis-relay.ts` new, `hub.ts`/`origin-policy.ts`/7 API routes/`remote.go` modified,
  `ioredis` dep added; scripts + tests untouched — verified by diff). Orchestrator's
  INDEPENDENT full-protocol-suite run caught a p2 regression the coder's spot-check
  missed; coder fixed it (consume-once `session_gone` marker in `SWEEP_SCRIPT`).
  Re-verified: orchestrator ran the full suite → **50/50 green (3.1m)**; off-limits +
  scripts clean both before and after the fix.
- 2026-07-31T22:26Z — Reviewer true-content-diff built: `tmp/implement-review/change-surface.diff`
  (807 lines; baseline vs current change-surface snapshot, plain `diff -r` — note macOS
  `diff` rejects GNU `--no-index`).
- 2026-08-01T06:33Z — Phase 3 INLINE (reviewer agent could not spawn — see Challenges).
  Per the user ("the goal is to make it work on Vercel — deploy + test on Vercel"), the
  orchestrator deployed the Redis-backed harness to Vercel production and proved it there.
- 2026-08-01T06:33Z — **Vercel deploy blocker found+fixed**: `harness/package.json`
  `engines.node` was `">=26.0.0"` (matches `.nvmrc` 26.5.0, pre-existing — not introduced
  this task) but Vercel's platform supports only ≤ Node 24, so the build was rejected.
  Relaxed to `">=24.0.0"` (Vercel builds on 24.x; local dev stays on 26.5.0; both satisfy).
- 2026-08-01T06:35Z — **Vercel smoke PASS** (production URL `https://true-bdd-app.vercel.app`,
  deployment `dpl_5PXreiamjqznZeqZHg6FiHCeB6ix`): CLI `true-bdd remote --server <vercel>`
  registered over HTTPS → session appeared in `GET /api/sessions` and the browser UI
  (cross-serverless-invocation Redis rendezvous); `GET /api/sessions/{id}` returned the
  host-CLI inventory scan (reply-via-Redis round-trip); browser-dispatched `build tests` →
  host CLI executed + output streamed back to the Vercel run page (terminal/error exit 1 —
  bare host folder has no config; the streaming mechanism is what's proven); after killing
  the CLI, the session was evicted by the Redis abandonment sweep (`{"sessions":[]}`).
  Screenshot: `tmp/vercel-smoke-run-streaming.png`.
- 2026-08-01T06:37Z — Inline read-only Codex review round running
  (`tmp/codex-vercel-review-prompt.md` → `tmp/codex-vercel-review.md`); the reviewer
  agent's own loop couldn't run (agent won't spawn), so the orchestrator runs it.
- 2026-08-01T06:50Z — **Codex reviewer round 1 DONE (3 findings, scored by orchestrator):**
  (1) `redis-relay.ts` CANCEL leaked inflight capacity for aborted `delivered` work →
  **KEEP (7)**, APPLIED — `CANCEL_SCRIPT` now mirrors `TIMEOUT`'s `delivered→orphaned`
  branch (frees capacity + renews lease; late reply still authenticates). (2) deployed
  origin-policy trusts `x-forwarded-host` → **SKIP (4)** — on Vercel the edge overwrites
  that header authoritatively (code comment + p19 confirm), the non-goal disclaims access
  control, and the proposed fix (require `HARNESS_PUBLIC_ORIGIN`) would break the working
  deploy; residual note for non-Vercel topologies. (3) p22 "real CLI" half doesn't test
  late-reply-after-504 → **SKIP (5)** — that path IS deterministically covered by p22
  Assert 1 (raw agent, same reply route); making the real-CLI half deterministic is
  impractical. Round yielded 1 keep ⇒ applied; loop stops (no round 2 needed).
- 2026-08-01T06:50Z — **DISCOVERY**: a reviewer-agent spawn that "failed to spawn" had in
  fact done extensive hardening before its context blew — it authored p23 (sweep capacity
  reclamation) + p24 (orphan-TTL expiry), modified p17–p22 + the unit suites, deleted
  `registry.ts`, added `HARNESS_ORPHAN_TTL_SEC`, the `agent/poll` route change, and a Go
  test (`remote_internal_test.go`). All verified coherent: Playwright protocol **52/52**,
  vitest **133 passed/5 skipped**, `go test ./src/...` green, typecheck + lint clean.
- 2026-08-01T06:50Z — **Re-deployed to Vercel production** with the CANCEL fix
  (`true-bdd-q13vgrpx3-kavoon.vercel.app`, Ready); alias `true-bdd-app.vercel.app`
  serves HTTP 200 with Redis `/api/sessions` working. Vercel now runs the final code.
