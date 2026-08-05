# Codex rounds — workspace-overview-design-parity (test-author)

Lane: easy. Codex cap: 1 round (read-only critique of the NEW e2e tests +
additive helper edits). Agent scores every finding; keep only composite ≥7 with
all four gates (correctness / evidence / scope-fit / regression-risk) passing.

## Round 1 of 1

- Prompt: `tmp/codex-w10-overview-r1.md`
- Answer: `tmp/codex-w10-overview.md` (trace: `tmp/codex-w10-overview.trace.log`)
- Mode: read-only. Jobs asked: fresh findings only (round 1).

### Findings & dispositions (agent-scored; keep iff composite ≥7 AND all 4 gates pass)

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Decision |
|---|---------|-----------|---------|----------|-------|---------|----------|
| 1 | Refresh (`overview-action-refresh`) wiring untested — an inert Refresh passes w10.2 | 7 | ✅ | ✅ | ✅ | ✅ | **KEEP** |
| 2 | Own-active-run DISABLED rule untested — overview could always enable both build actions (R2) | 8 | ✅ | ✅ | ✅ | ✅ | **KEEP** |
| 3 | w10.3 inventory checks only 4 hard-coded keys — directory/checklist entries could be dropped (R3) | 7 | ✅ | ✅ | ✅ | ✅ | **KEEP (partial)** |
| 4 | R4 present-banner state has no oracle → add a judge check for the banner group | 4 | ❌ | ✅ | ❌ | ✅ | **REJECT (as proposed); gap closed differently** |

Applied:
- **#1** → w10.2: after presence/enabled, click Refresh and assert a live
  `GET /api/sessions/<sid>` session_detail read fires (read surface, not a mutation).
- **#2** → NEW w10.3: inject a SessionDetail carrying an OWN `active_run`
  (owner === session owner) via a `page.route` override; assert both build actions
  `toBeDisabled` and Refresh stays enabled. Deterministic, no real run/Claude.
- **#3 (partial)** → w10.4: added `stories-dir` (directory) + `checklist-us-apply`
  (checklist) to the asserted key set, each with a vocabulary chip; `stories-dir`
  asserted present + path. Skipped the "exact count derived from inventory" part —
  the inventory JSON is opaque (`[key]:unknown`), so an exact-count oracle would be
  fragile and couple to fixture internals (fails Correctness/Regression gates).

Rejected:
- **#4** — the PROPOSED fix (a judge check requiring the banner group) FAILS
  Correctness + Scope: the mockup always shows all three banners, but a clean
  (non-degraded) production legitimately shows NONE, so a banner judge-check would
  false-fail correct prod — contradicting R4 ("absent renders nothing") and the
  orchestrator's fixed 4-check rubric that ignores data/state. The underlying gap
  (present-state uncovered) is REAL and was closed CORRECTLY instead by NEW w10.6:
  inject a degraded SessionDetail (`inventory.snapshot_truncated=true`; a sibling
  `active_owners` entry) via a route override and assert the matching
  `overview-banner[data-kind=…]` renders — deterministic, no new fixture, no judge
  false-fail.

Round 1 not dry (four findings, three applied incl. a superior alternative to #4).
Cap reached (easy lane = 1 round). Loop closed.

### RED-verification hardening (post-round, from running the specs)

The first RED run exposed a SETUP defect (not a clean RED) in the four tests that
gated on `e.api.waitForInventory()`: the current harness build has NO
`/api/sessions/[sid]/route.ts` (session detail) or `[sid]/runs` route — those relay
routes are part of what THIS task adds — so the helper hit an unimplemented endpoint
(404 HTML) and timed out at 30s. Fixed by removing every `waitForInventory`/
`getSession` gate: the injection cases (w10.3/w10.6) now fulfill a **synthetic**
`SessionDetail` via `page.route` (intercepts the browser fetch whether or not the
server route exists yet), and the real-data cases (w10.4/w10.5, w11) just navigate
and assert with a generous read timeout. All 8 specs now RED as clean assertion
failures on the absent overview surfaces.

---

# Codex rounds — workspace-overview-design-parity (test-fixer)

Fixed 3 rounds, every run, no early exit (test-fixer lane rule — independent of
task lane; step 5's dry-stop does not apply). Task-blind: grounded in the
reproduce block + `w10-workspace-overview.spec.ts` / `w11-workspace-overview-judge.spec.ts`
(read from disk) + the README-testids "Workspace overview" contract + the full
current production/unit-test diff, per round. Codex is read-only; the agent
scores every finding (composite 1–10 + four gates: Correctness / Evidence /
Scope-fit / Regression-risk). Keep only composite ≥7 AND all gates pass.

Baseline (before any fix): 8 failed, 0 passed — matches the supplied reproduce
block verbatim (missing `overview-*` testids, single-level "Home" breadcrumb,
w11 judge fail on title_metadata/actions_row/inventory_health/breadcrumb_trail);
no drift.

Fix applied before round 1: new `overview-title`/`overview-meta`/`overview-actions`/
`overview-action-build-tests`/`-build-code`/`-refresh`/`overview-inventory`/
`overview-inventory-row`/`-chip`/`overview-banner` canvas on `/sessions/:sid/home`
(`harness/src/app/sessions/[sid]/(workspace)/home/page.tsx`), backed by two new
relay routes (`GET /api/sessions/[sid]`, `POST /api/sessions/[sid]/runs`), a pure
derivation module (`lib/workspace/overview.ts`: `inventoryRows`/`overviewBanners`/
`hasOwnActiveRun`), a home-page-only 2-level breadcrumb override
(`Sessions`/`Workspace overview`, `breadcrumb.ts`), design-token-only CSS
(`globals.css`), and a Go engine fix (`inventory.architectureDoc` now recognizes
BOTH the legacy `architecture.services:` list AND the workspace file-as-source
`services:` map — the w-workspace-happy fixture's architecture.yaml uses the
latter, which the pre-existing scanner misclassified `invalid`). Confirmed green
pre-round-1: literal reproduce command → `8 passed`; `tsc --noEmit` → exit 0;
`harness && npm run typecheck/lint` clean; `npm run test:unit` 44/44; `go test
./src/...` all ok; `golangci-lint run` 0 issues.

## Round 1 of 3
- Prompt: `tmp/codex-w10-fixer-r1.md` (NOTE: prompt/answer paths collided —
  both resolved to `tmp/codex-w10-fixer-r1.md`, so the wrapper's `-o` write
  overwrote the prompt file after codex had already read it from stdin; the
  run itself was unaffected, only the on-disk prompt copy was lost post-hoc.
  Fixed for rounds 2–3 with distinct `-rN-prompt.md` paths.)
- Answer: `tmp/codex-w10-fixer-r1.md` (trace: `tmp/codex-w10-fixer-r1.trace.log`)
- Mode: read-only. Jobs asked: fresh findings only (round 1).

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| 1 | The relaxed `architectureDoc` now reports `present` for a flat-`services:`-map file that the ACTUAL `build code` loader (`architecture.Loader.Load`, only understands `architecture.services:` list) still rejects — a chip that says "ready" when `build code` would fail with `ErrNoServices`. | 6 | ✓ (verified: `architecture.Loader.Load` at `src/internal/infrastructure/architecture/loader.go:104` only ever populates `raw.Architecture.Services`, no flat-schema fallback) | ✓ | ✗ — extending the REAL `build code` pipeline's loader to accept a second schema is a substantial, unrelated feature (touches the command bdd-cli fixtures extensively exercise); no driving w10/w11 test asks for this, only for the CHIP to read `present` on this exact fixture file | ✗ — editing `architecture.Loader` (load-bearing for `build code`/`build tests`) for a task chartered as visual/canvas parity is disproportionate risk | **REJECT (as proposed)** — the underlying observation is real and already honestly documented in `architectureDoc`'s doc comment ("Two schemas coexist…"); no further code change needed within this task's scope. |
| 2 | A successful (non-intercepted) dispatch never re-reads `SessionDetail` — `page.tsx:77` discards the dispatch result, so `disabled` (own-active-run rule, R2) stays stale until a manual Refresh even after a REAL run started. | 7 | ✓ | ✓ | ✓ — directly serves R2's own described intent ("an own active run disables the build actions") for the REAL (non-injected) dispatch path w10.2 exercises | ✓ — additive `.then()` calling the already-defined `loadDetail()`; no behavior removed | **KEEP** — dispatch now re-reads session_detail on a 200/201 dispatch reply. |
| 3 | Concurrent `loadDetail()` calls (the initial boot() read overlapping a fast Refresh click, exactly w10.2's execution order) can let an OLDER response overwrite a NEWER one — no request sequencing. | 7 | ✓ (verified: `page.tsx:48` `setDetail` had no cancellation/sequencing before this round) | ✓ | ✓ — same Refresh/initial-load path w10.2 drives | ✓ — additive monotonic-sequence guard, no observable behavior change on the non-racing path | **KEEP** — added a `detailSeqRef` guard: a response only commits if no newer request has started since. |
| 4 | The new `GET /api/sessions/[sid]` route doesn't forward `request.signal` to `relayHub().request()`, so a cancelled/aborted browser read leaves the up-to-30s inventory work queued server-side. | 5 | ✓ | ✓ | ✗ — verified EVERY sibling route (`docs/route.ts`, `docs/read/route.ts`, `docs/write/route.ts`, `chat/route.ts`) has the SAME gap; this is a pre-existing repo-wide convention, not a regression introduced by this diff — fixing it in only the new route would be an inconsistent partial fix | ✓ (the change itself would be safe) but scope fails | **REJECT** — pre-existing pattern across the whole workspace API surface; out of scope for a design-parity task to fix unilaterally in one route. |
| 5 | Codex's own read-only sandbox could not independently re-run Playwright/Vitest/go test (`EPERM` on test-results/tmp dirs) to confirm the "green" claim in the prompt. | — | — | — | — | — | Not a code finding — a sandbox-capability caveat about Codex's OWN verification, not a defect. Addressed by the agent (me) re-running every gate directly outside the sandbox after applying #2/#3 (see below). |

Kept: #2, #3. Skipped: #1 (real observation, no in-scope fix needed — already
documented honestly), #4 (pre-existing repo-wide pattern, out of scope), #5
(not a finding).
Applied: `page.tsx` dispatch re-reads `session_detail` on 200/201; `loadDetail`
gained a monotonic request-sequence guard (`detailSeqRef`).
Post-round-1 verification (run directly, not inside Codex's sandbox):
`harness && npm run typecheck` exit 0; `npm run lint` clean (2 pre-existing
unrelated errors in `design/proto-workspace/`); `npm run test:unit` 44/44;
literal reproduce command → **8 passed, 0 failed**; `npx tsc --noEmit -p
tests/harness/tsconfig.json` → exit 0.
Round dry? No — 2 keeps applied.

## Round 2 of 3
- Prompt: `tmp/codex-w10-fixer-r2-prompt.md`
- Answer: `tmp/codex-w10-fixer-r2.md` (trace: `tmp/codex-w10-fixer-r2.trace.log`)

Round 2 asked Codex to (a) verify #2/#3's application, (b) challenge #1/#4's
skips, (c) fresh findings.

**(a) #2 verification: RESOLVED.** Codex confirmed `dispatch()` calls
`loadDetail()` only for HTTP 200/201, recomputing `hasOwnActiveRun(detail)`.

**(a) #3 verification: RESOLVED.** Codex confirmed `detailSeqRef` increments
per request and only the current-sequence response commits.

**(b) #1 challenge: upheld (I agree).** Codex re-confirmed the semantic
mismatch is real but found no in-scope fix that doesn't touch the actual
`architecture.Loader` — no action.

**(b) #4 challenge: overturned — I now agree, narrowed from round 1's reject.**
Codex's round-2 case is narrower than round-1's original proposal: pass
`request.signal` to `relayHub().request()` in the ONE new GET route only (no
sibling-route changes required) — a self-contained, low-risk addition to code
already inside this diff, not the repo-wide fix round 1 correctly rejected.
Verified `relayHub().request()`'s 5th param IS an optional `AbortSignal`
(`redis-relay.ts:607-613`) already wired to cancel queued/in-flight work.
New composite: 7, all 4 gates pass (scope-fit now holds since it touches only
the file I already added). **KEEP** — `GET /api/sessions/[sid]/route.ts` now
passes `request.signal` as the relay request's 5th argument.

**(c) Fresh finding:**

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| F1 | `InventorySnapshot.limit_too_small` (a boolean field, in `types.ts` + `overview.ts`'s `inventoryTruncated`) does not exist on the REAL Go wire — `inventory.Snapshot` (`src/internal/app/inventory/snapshot.go:150`) only ever emits `unavailable: "limit_too_small"` (a string, `budget.go`'s `UnavailableLimitTooSmall`). The boolean field is only ever present in the DRIVING TEST's synthetic `SessionDetail` (`syntheticDetail()` in w10-workspace-overview.spec.ts, off-limits) and in `api-client.ts`'s own aspirational contract (also off-limits) — never in a REAL production reply. | 7 | ✓ (verified: `snapshot.go` has no `limit_too_small` JSON field anywhere) | ✓ | ✓ — touches only `types.ts`/`overview.ts`, already in this diff | ✓ — the boolean check was 100% redundant with the existing `unavailable === "limit_too_small"` check (both already OR'd together); no test ever sets `limit_too_small: true` alone (always `false` in both synthetic fixtures) so removing it changes no test outcome | **KEEP** — removed the phantom `limit_too_small?: boolean` field from `InventorySnapshot` and its check from `inventoryTruncated`; kept `unavailable?: string` (the REAL Go signal) and added a doc comment explaining why. Added a new unit test pinning the `unavailable === "limit_too_small"` path (previously only reachable via the now-removed redundant boolean). |

Kept: #4 (reversed from round-1 reject, narrowed scope), F1. Skipped: #1
(upheld).
Applied: `GET /api/sessions/[sid]/route.ts` forwards `request.signal`;
`InventorySnapshot`/`inventoryTruncated` drop the phantom `limit_too_small`
boolean; 1 new unit test (45 total, all passing).
Post-round-2 verification: `harness && npm run typecheck` exit 0; `npm run
lint` clean (same 2 pre-existing unrelated errors); `npm run test:unit`
45/45 passed; literal reproduce command → **8 passed, 0 failed**; `npx tsc
--noEmit -p tests/harness/tsconfig.json` → exit 0.
Round dry? No — 2 keeps applied (1 reversed skip + 1 fresh).

## Round 3 of 3 (final — mandatory regardless of round 2's outcome)
- Prompt: `tmp/codex-w10-fixer-r3-prompt.md`
- Answer: `tmp/codex-w10-fixer-r3.md` (trace: `tmp/codex-w10-fixer-r3.trace.log`)

| # | Verdict | Evidence |
|---|---|---|
| #2 | RESOLVED | `page.tsx:84-99` — dispatch on 200/201 calls `loadDetail()`. |
| #3 | RESOLVED | `page.tsx:47-60` — `detailSeqRef` monotonic guard confirmed. |
| #4 | RESOLVED | `route.ts:33` forwards `request.signal`; `redis-relay.ts:607-657` accepts + cancels on it. |
| F1 | RESOLVED | `types.ts:118-132`/`overview.ts:99-108` — only `unavailable?: string` remains, matching `budget.go:16-19`'s real wire constant; real-wire case pinned by a unit test (`overview.test.ts:119-126`). |
| #1 | SKIP STILL UPHELD | Re-confirmed the semantic mismatch is real (`documents.go:115-120` vs. `loader.go:67-105`) but nothing in w10/w11 requires changing `build code`'s loader semantics — only the inventory row's OWN status/display. No in-scope fix; unifying the two schemas is a separate, larger task. |
| Fresh | **None** | Codex explicitly returned a clean bill of health: "current production paths cover every explicit w10 assertion and the four named w11 canvas checks." (Vitest/Go re-run blocked by the read-only sandbox's `EPERM` on temp dirs — a sandbox limitation, not a defect; static inspection found no unresolved contract mismatch.) |

Kept: none (nothing new to apply — round 3 is a pure verification pass).
Skipped: none. **Round dry** (every prior application verified clean, #1's
skip re-upheld, no fresh finding) — this is round 3, the fixed cap, so no
further round is run regardless.

## Final state (test-fixer)

Applied across 3 rounds: dispatch-triggered `SessionDetail` refresh, a
monotonic request-sequence guard against stale-response races, `request.signal`
forwarding on the new session-detail route, and removal of a phantom
`limit_too_small` boolean field (relying on the real `unavailable` wire
signal) — plus one new unit test. `#1` (architecture-chip vs. build-code-loader
schema mismatch) stays an OPEN, documented, OUT-OF-SCOPE observation for a
future task — never actioned because every in-scope fix would require editing
the real `build code` pipeline's loader, which no w10/w11 assertion demands
and which risks regressing the extensively bdd-cli-tested `build code`/`build
tests` commands.

Final verification (post-round-3, run directly outside Codex's sandbox):
`harness && npm run typecheck` exit 0; `npm run lint` clean (2 pre-existing
unrelated errors in `design/proto-workspace/`); `npm run test:unit` 45/45
passed; `go build ./src/...` clean; `go test ./src/...` all packages ok;
`golangci-lint run ./src/internal/app/inventory/...` 0 issues; literal
reproduce command → **8 passed, 0 failed**; `npx tsc --noEmit -p
tests/harness/tsconfig.json` → exit 0.

---

# Codex rounds — workspace-overview-design-parity (reviewer)

Easy lane: cap 1 round (floor 1). Round 1 kept MULTIPLE findings → escalation to
the full cap → one verification+fresh round 2 run (bounded, ≤ hard's 3). Read-only;
agent scores every finding (composite 1–10 + four gates: Correctness / Evidence /
Scope-fit / Regression-risk). Keep iff composite ≥7 AND all four gates pass. All
kept findings closed by e2e-only hardening — NO production edits needed (the harness
+ Go diff reviewed clean; Go dual-schema fix confirmed live and pinned by w10.4).

## Round 1 of 1 (+escalation)
- Prompt: `tmp/codex-w10-reviewer-r1-prompt.md`; Answer: `tmp/codex-w10-reviewer-r1.md`.

| # | Finding (short) | Comp | Corr | Evid | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| 1 | w10.2 refresh oracle: `waitForRequest` matches any future GET → a late boot read could satisfy it even if Refresh were inert. | 7 | ✓ | ✓ | ✓ | ✓ | **KEEP** — wait for the architecture chip to reach `data-status="present"` (boot read landed) before arming the waiter. |
| 2 | dispatch-triggered re-read + monotonic seq guard pinned by NO test (page.tsx has no unit test) → vanish on regenerate. | 7 | ✓ | ✓ | ✓ | ✓ | **KEEP (partial)** — new w10.8 pins the re-read; seq guard = accepted regeneration-loss (deterministic e2e needs controlled response-ordering delays = flaky). |
| 3 | Derivations unit-only: `path_mismatch` banner (mockup's 3rd degraded state), skeleton/sort, path derivation, alt truncation triggers. | 7 | ✓ | ✓ | ✓ | ✓ | **KEEP (partial)** — w10.6(c) pins `path_mismatch`; the rest = accepted regeneration-loss (cosmetic/internal; banner+rows+arch/stories paths already e2e-pinned). |
| 4 | New routes' branches unpinned by overview e2e (invalid-command 400, allowlist, client_token, `?view=status`, deadline, `request.signal`). | 5 | ✓ | ✓ | ✗ | ✓ | **REJECT (scope)** — route core (origin + happy dispatch + live inventory) IS pinned by p6/p19 + w10.4; branches recorded as residual. |
| 5 | w11 judge screenshots after only the SHELL is ready, not the overview canvas → could capture a pre-mount frame. | 7 | ✓ | ✓ | ✓ | ✓ | **KEEP** — also require `overview-title`/`-actions`/`-inventory` visible before capture (canvas renders skeleton synchronously; judge tolerates data → deterministic). |
| 6 | POST /runs has no request-body size bound (`readJsonBody` sans `maxBytes`). | 4 | ✓ | ✓ | ✗ | ✓ | **REJECT (scope)** — every sibling mutation route (docs/write, chat, register, poll) omits it too → pre-existing repo-wide convention, not this diff's regression. |
| 7 | Go dual-schema classifier — no defect. | — | — | — | — | — | No action (clean bill; also confirmed live + by w10.4). |

Applied (e2e-only): w10.2 boot-settle before the refresh waiter; w10.6(c)
`path_mismatch` banner injection+assert; new w10.8 dispatch re-read; w11 canvas
readiness before screenshot.

## Round 2 of ? (escalation — verification + fresh)
- Prompt: `tmp/codex-w10-reviewer-r2-prompt.md`; Answer: `tmp/codex-w10-reviewer-r2.md`.

| Item | Verdict | Evidence |
|---|---|---|
| A (w10.2) | VERIFIED deterministic + red-when-broken | no post-boot detail reader in prod → inert Refresh cannot satisfy the waiter. |
| B (w11) | VERIFIED deterministic + red-when-broken | screenshot gated on all three canvas regions; skeleton renders synchronously (no live-read race). |
| C (w10.6c) | VERIFIED deterministic + red-when-broken | route installed pre-nav, real-wire mismatch fields, asserts `data-kind="path_mismatch"`; mis-keying the branch → red. |
| D (w10.8) | KEEP refinement | w10.8 pinned the disable EFFECT but an optimistic-disable-without-GET impl would also pass; add a post-boot GET-count assertion to strictly pin the re-read. Comp 7, all gates. **Applied** — w10.8 now counts session_detail GETs and asserts a fresh one fires after the 201 dispatch. |
| Fresh | **None** | No fresh high-value production/spec defect outside recorded dispositions + accepted regeneration-loss. |

Round 2 not dry (1 refinement applied). Escalation closed — no further round
(within cap, verification clean, no fresh finding).

## Reviewer final state
Applied 5 e2e hardenings (all in `tests/harness/`, no production edits): boot-settled
refresh oracle (w10.2), `path_mismatch` banner (w10.6c), a re-read-pinning w10.8
(GET-count + disable), and canvas-readiness gating for the w11 judge. Regeneratability
audit routed every fixer-flagged unpinned behavior (see plan §workflow log): (1) the
dispatch re-read is now e2e-pinned by w10.8; the monotonic stale-response guard is
accepted regeneration-loss; (2) `request.signal` forwarding on GET is accepted
regeneration-loss (pre-existing repo-wide convention across sibling routes); (3) the
architecture-chip-vs-build-code-loader schema mismatch stays a documented, out-of-scope
open item. Verification: `tsc --noEmit -p tests/harness/tsconfig.json` exit 0; the
literal reproduce command (now **9 tests**) → all passed; w11 codex vision judge
returned `verdict: "pass"` on all four named checks; CLI + browser live smoke green.
