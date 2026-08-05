# Codex ledger — `home-sessions-list` (planner)

Hard lane, ≤3 rounds, early-exit on a dry round. Read-only Codex; the AGENT
scores (composite 1–10 + four gates: Correctness / Evidence / Scope fit /
Regression risk). Keep only composite ≥7 AND all four gates pass.

Artifacts under `tmp/` (paths.yaml → codex_artifacts): prompt
`tmp/codex-home-sessions-list-rN.md`, answer `tmp/codex-home-sessions-list.md`
(+ `.trace.log`).

## Round 1

- Prompt: `tmp/codex-home-sessions-list-r1.md` · Answer:
  `tmp/codex-home-sessions-list.md` (+ `.trace.log`). Fresh-findings round.
- 12 findings; ALL 12 kept (each grounded in verified repo evidence, tests-only
  scope, no regression risk).

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Keep |
|---|---|---|---|---|---|---|---|
| 1 | P4 doesn't prove "without reloading" (prose only) | 8 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 2 | P9 empty-count can pass on un-mounted client (no handshake) | 8 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 3 | P10 doesn't test the sustained-failure threshold (first-503 passes) | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 4 | P10 recovery lacks a deterministic failure-start/recover boundary | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 5 | P7 pre-workspace frame (rail/sidebar absent) not asserted | 8 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 6 | P3 doesn't pin per-row Open href → wsRoutes.home(sid) | 8 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 7 | P5 body-text ban rejects faithful prototype copy; "for B" undefined | 9 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 8 | P8 order: connected_at ms tie + weak refresh isolation | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 9 | w19 auditVerdict must take SESSIONS_PARITY_CHECK_NAMES | 8 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 10 | w19 needs 12-min per-test timeout (proto boot budgets) | 8 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 11 | w19 needs guaranteed prototype stop() teardown | 8 | ✓ | ✓ | ✓ | ✓ | KEEP |
| 12 | Scaffolding names impl file/polling arch (steers production) | 9 | ✓ | ✓ | ✓ | ✓ | KEEP |

Applied: w18.3 (F6 per-row href), w18.4/w18.5 (F1 nav-request counter; F7
structural P5), w18.7 (F8 connected_at guard + waitForResponse), w18.8 (F2
handshake), w18.9 (F3+F4 firstFailure handshake + threshold both-directions +
mode-switch recovery), w18.10 (F5 rail/sidebar/appShell/workspaceMain count 0),
w19.1 (F9 auditVerdict check-names; F10 12-min; F11 stopPrototype teardown),
Startup scaffolding (F12 stripped impl direction). Swept Challenges: P9/P10
handshake note, new no-reload note, P5 structural-only note.

## Round 2

- Prompt: `tmp/codex-home-sessions-list-r2.md` · Answer: `tmp/codex-home-sessions-list.md`.
- Verify/challenge/fresh. Verification: 10/12 RESOLVED; **F3 (P10 threshold
  precision) and F4 (P10 recovery waiter race) NOT-RESOLVED** — botched
  applications, re-fixed this round (they count as applied findings for round 3
  to verify). No skips to reverse. 5 fresh findings: 4 kept, 1 skipped.

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Disp |
|---|---|---|---|---|---|---|---|
| V3 | P10 threshold still imprecise ("well under 30s"/"prefer clock" optional) | 7 | ✓ | ✓ | ✓ | ✓ | FIX |
| V4 | P10 recovery waiter created after flip → registration race | 8 | ✓ | ✓ | ✓ | ✓ | FIX |
| FF1 | Blank stale rows during the 0–30s grace period | 5 | ~ | ✓ | **FAIL** | ~ | SKIP |
| FF2 | Recovery never proves the notice CLEARS | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| FF3 | `sessions-list` in contract but no case asserts it | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| FF4 | Structural check misses a marker with no `data-session-id` | 8 | ✓ | ✓ | ✓ | ✓ | KEEP |
| FF5 | P10 stimulus "503s/aborts" is discretionary | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |

- FF1 SKIP rationale (Scope-fit FAIL): the brief's "stale rows never silently
  presented as current" is a consequence of the notice replacing the list at the
  ~30s threshold; it does NOT mandate blanking rows during the grace window.
  Requiring immediate row-removal on the first failure invents a grace-period UX
  the requirement does not demand and could false-fail a reasonable transient-blip
  impl. P10's testable core (threshold + notice + recovery) is already covered.
- Applied: w18.9 (V3 clock-primary ~29s/past-30s + real-time fallback; V4+FF2
  waiter-before-flip + assert notice clears; FF5 canonical 503-JSON stimulus),
  w18.1 (FF3 rows scoped to `sessions-list`), w18.5 (FF4 forbidden
  `session-disconnected`/`-unreachable`/`-reconnect` count 0), w18.6 (FF3
  `sessions-list` absent on empty). Swept: Target-state + H1 ui.ts/README bullets
  (negative-contract markers + `sessions-list`), Challenges P9/P10 + P5 bullets.

## Round 3 (final — cap reached)

- Prompt: `tmp/codex-home-sessions-list-r3.md` · Answer: `tmp/codex-home-sessions-list.md`.
- Verify/challenge-skip/fresh. Verification: V4/FF2/FF5/FF3/FF4 RESOLVED; **V3
  NOT-RESOLVED** (internal contradiction: "navigate then get a live row" vs
  "install clock before navigation, start in fail") — re-fixed. One stale FF3
  overclaim purged ("every asserting case scopes rows to it" → "the exact-count
  list assertions"). FF1 skip CHALLENGED — HELD (see below). 5 fresh findings: 4
  kept, 1 folded.

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Disp |
|---|---|---|---|---|---|---|---|
| V3 | w18.9 clock sequence internally contradictory | 8 | ✓ | ✓ | ✓ | ✓ | FIX |
| f1 | Fake-clock recovery hangs (timers don't self-fire after mode flip) | 8 | ✓ | ✓ | ✓ | ✓ | KEEP (into w18.9) |
| f2 | H1 "exists nowhere"/grep claims overstate blast radius | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| f3 | w18.5 forbidden markers "throughout" not proven by final count-0 | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| f4 | w18.10 h1 "present" too weak (P7 wants exactly "Sessions") | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| f5 | w18.7 refresh waiter under-specified (GET/200/pre-arm) | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| FF3-oc | stale "every asserting case scopes rows" overclaim | 7 | ✓ | ✓ | ✓ | ✓ | FIX |

- FF1 skip HELD (survives challenge). Codex argues "stale rows never shown as
  current" is independent of the notice state, so a normal row visible for 29s
  fails P10. Reconciliation: P10 sentence 1 mandates the notice only after >~30s
  SUSTAINED failure precisely to DEBOUNCE transient blips; forcing rows to blank
  on the FIRST failure contradicts that debounce and would false-fail a compliant
  impl. In THIS design there is no stale/reachability-marker vocabulary (P5), so
  the invariant is correctly pinned WHERE staleness is declared — w18.9 already
  asserts the stale row is gone when the notice appears at the threshold. Skip
  stands (Scope-fit / Regression-risk).
- Applied: w18.9 (V3 single canonical clock sequence: install→pass→navigate→row→
  fail→advance→firstFailure; f1 advance-clock-for-recovery), w18.5 (f3
  MutationObserver "throughout"), w18.10 (f4 exactly-one h1 == "Sessions"), w18.7
  (f5 GET/200/pre-armed waiters), Target state + Challenges (f2 precise blast
  radius; FF3-oc wording). Verified grep: `test-connection` outside the binding
  contract lives only in the prototype, SPEC.md, historical docs, a version.go
  comment, and w18.10's negative literal.

## Outcome

Loop closed at the hard-lane cap of 3 rounds (round 3 was not dry, but the cap is
binding). Round 1: 12/12 kept. Round 2: 2 botched-application re-fixes + 4/5
fresh kept (1 skipped). Round 3: 1 botched-application re-fix + 1 overclaim purge
+ 5 fresh kept; FF1 skip held under challenge. Net: the plan's P4/P5/P8/P9/P10
determinism, the P3/P7 assertion strength, the P5 negative contract, and the
tests-only scope discipline were materially hardened.

---

# Codex ledger — `home-sessions-list` (TEST-AUTHOR)

Hard lane, ≤3 rounds, early-exit on a dry round. Read-only Codex; the AGENT
scores (composite 1–10 + four gates). Keep only composite ≥7 AND all gates pass.
Artifacts: prompt `tmp/codex-home-sessions-list-author-rN.md`, answer
`tmp/codex-home-sessions-list-author.md` (+ `.trace.log`).

## Round 1 (fresh only)

- Prompt: `tmp/codex-home-sessions-list-author-r1.md`. 2 findings; both KEPT.

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Keep |
|---|---|---|---|---|---|---|---|
| A | w18.7 two `waitForResponse` armed concurrently can BOTH resolve from ONE poll response → only proves a single refresh | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| B | w18.6 empty-frame wordmark only asserted visible, not `--text-inverse` consumption (a hardcoded-colour empty fork passes) | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |

- Applied A: w18.7 refresh handshake armed+awaited SEQUENTIALLY (`sessionsRead()`
  helper), DOM order re-checked after EACH of two distinct GET-200 polls.
- Applied B: w18.6 adds the live `--text-inverse` sentinel probe on the
  empty-frame wordmark (mirrors w18.10), asserting it recomputes to the sentinel.
- `npx tsc --noEmit` (tests/harness) green after both.

## Round 2 (verify + challenge + fresh)

- Prompt: `tmp/codex-home-sessions-list-author-r2.md`. Round-1 A/B verified applied
  (A got a follow-on render-commit concern → F1). 4 fresh findings: 3 KEEP, 1 SKIP.

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Disp |
|---|---|---|---|---|---|---|---|
| F1 | w18.7 `waitForResponse` resolves before React commits → read may see stale DOM, false-green a reshuffle | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| F2 | token probe removeProperty doesn't restore a pre-existing INLINE `--text-inverse` | 4 | **FAIL** | ✓ | ✓ | ✓ | SKIP |
| F3 | gradient assertion accepts ANY element with the token, not a top bar | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| F4 | page-wide `getByText` wordmark/tagline strict-fail a responsive dual-node DOM | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |

- F2 SKIP (Correctness FAIL): `:root`'s `--text-inverse` is defined in tokens.css
  (a STYLESHEET rule), so the inline `style` value is empty before the probe and
  `removeProperty` restores the stylesheet default cleanly. No pre-existing inline
  override exists to clobber — the premise does not hold for this codebase.
- Applied F1: `flushRender(page)` double-rAF after each poll response in w18.7 (a
  test-only, deterministic commit barrier — NOT Codex's production
  render-generation marker, which would steer implementation, out of scope).
- Applied F3: `gradientBandCount` → `gradientTopBarCount` — adds a top-anchored
  (`rect.top<=24`) + viewport-wide (`>=0.85*innerWidth`) geometry gate; used by
  w18.6 + w18.10. No new testid (stays within the plan's binding contract).
- Applied F4: `.filter({ visible: true })` on the wordmark (w18.6/w18.10) and the
  tagline (w18.10). No new testid.
- `npx tsc --noEmit` (tests/harness) green after all three.

## Round 3 (final — cap reached)

- Prompt: `tmp/codex-home-sessions-list-author-r3.md`. Verified F4 clean; F2 skip
  HELD (challenge found no inline `--text-inverse` writer). F1/F3 re-flagged as
  incompletely applied → re-fixed (count as applied findings). 2 findings, both KEEP.

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Disp |
|---|---|---|---|---|---|---|---|
| F1' | double-rAF is NOT a commit barrier — read can still see the pre-refresh DOM | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |
| F3' | geometry gate still admits a full-page bg / tall hero / offscreen element (no top lower-bound, no height cap) | 7 | ✓ | ✓ | ✓ | ✓ | KEEP |

- F2 skip HELD under challenge: grep found no code setting an inline
  `--text-inverse` on `:root`/`documentElement`; the token is stylesheet-defined,
  so `removeProperty` restores cleanly. Skip stands.
- F4 verified clean (`.filter({ visible: true })` is valid PW 1.62 API; 0 visible
  matches on the placeholder still fails `toBeVisible` → RED preserved).
- Applied F1': replaced the `flushRender` double-rAF with an AUTO-RETRYING
  `expect.poll(readOrder).toEqual(expectedOrder)` after each poll (a real commit
  barrier on the LIVE DOM). Kept the plan's real-remote method rather than Codex's
  synthetic-injection rewrite (hard lane = follow the plan; only the assertion is
  made race-free). `flushRender` helper removed (no dangling symbol).
- Applied F3': `gradientTopBarCount` now requires `rect.top ∈ [-24,24]` AND
  `rect.height <= 240` (a shallow top BAND) in addition to viewport width —
  excludes a full-page background, a tall hero, and an offscreen element.
- `npx tsc --noEmit` (tests/harness) green.

## Outcome (test-author)

Loop closed at the hard-lane cap of 3 rounds. R1: 2/2 kept (w18.7 dual-waiter,
w18.6 token proof). R2: 3/4 kept (render-race, gradient-not-a-topbar, unscoped
locators), 1 skipped (restore-inline — premise false). R3: 2 re-fixes of the
render-race + geometry gate; F2 skip held. Net: the P8 order-stability proof, the
P7 gradient-top-bar structural gate, and responsive-DOM robustness were hardened;
tests-only scope held (no new testids beyond the plan's contract, no production
steering). `npx tsc --noEmit` green throughout.

---

# Codex ledger — `home-sessions-list` (TEST-FIXER)

Task-blind — grounded in the reproduce block + the e2e specs read from disk (no
brief, no plan). Fixed 3 rounds always (no early exit). Read-only Codex; the AGENT
scores (composite 1-10 + four gates: Correctness / Evidence / Scope fit /
Regression risk). Keep only composite >=7 AND all four gates pass. Artifacts:
prompt `tmp/codex-home-sessions-list-fixer-rN-prompt.md`, answer
`tmp/codex-home-sessions-list-fixer-rN.md` (+ `.trace.log`).

Note: round 1's FIRST invocation collided the prompt-file path with the `-o`
answer-file path (`tmp/codex-home-sessions-list-fixer-r1.md` used for both),
overwriting the prompt before it could be preserved as an artifact — Codex still
answered coherently (it read the changed files itself via its own read-only repo
access), but the run was discarded and redone cleanly with distinct paths
(`-r1-prompt.md` in, `-r1.md`/`.trace.log` out) per paths.yaml's codex_loop
guidance. Only the redone, correctly-isolated round 1 is scored below.

## Round 1 (fresh findings only)

- Prompt: `tmp/codex-home-sessions-list-fixer-r1-prompt.md` - Answer:
  `tmp/codex-home-sessions-list-fixer-r1.md` (+ `.trace.log`). 6 findings.

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Disp |
|---|---|---|---|---|---|---|---|
| 1 | A hung (not just erroring) `/api/sessions` request permanently stalls the poll loop — no timeout on `fetch()`, so P10's sustained-failure notice + auto-recovery could never be reached | 8 | Y | Y | Y | Y | KEEP |
| 2 | Unmount doesn't cancel the in-flight fetch; `cancelled` is checked before `res.json()` but not after, so a late resolve after unmount can still call `setState` | 7 | Y | Y | Y | Y | KEEP |
| 3 | A malformed 200 (`{}`, `{sessions:null}`, non-array) is silently treated as a genuine empty read via `body.sessions ?? []`, contradicting P6/P9's "only after a genuine successful zero read" | 7 | Y | Y | Y | Y | KEEP |
| 4 | Equal `connected_at` has no deterministic tie-breaker in `sortSessionsByConnectedAt` | 5 | ~ | Y | **FAIL** | ~ | SKIP |
| 5 | New CSS literals (22px, 1100px, -1px, 2px, 6px, 3px 9px, 10px 18px) bypass design tokens | 4 | **FAIL** | ~ | Y | ~ | SKIP |
| 6 | No `aria-live`/`role="status"` region announces list state transitions to assistive tech | 6 | Y | Y | ~ | Y | SKIP |

- Finding 4 SKIP (Scope-fit FAIL): w18.7 itself documents connected_at ties as
  deliberately UNTESTABLE — its own comment says a tie makes the sort "ambiguous,
  so fail as a SETUP problem rather than pass wrongly." The driving spec does not
  want production to resolve ties; it wants the test run to fail loudly instead.
  Adding a tie-breaker would invent behavior the spec explicitly declines to pin.
- Finding 5 SKIP (Correctness FAIL): every flagged literal is a pixel-for-pixel
  copy of `harness/design/proto-workspace/app/public/vendor/mockups.css` (the
  `design_system` prototype baseline, paths.yaml) — `.mockup-brand{font-size:22px}`,
  `.sessions-canvas{max-width:1100px}`, `.list-row__meta{margin-top:2px}`,
  `.chip{gap:6px;padding:3px 9px}`, `.btn{padding:10px 18px}` — none map onto an
  existing `--space-*`/`--fs-*` token (verified: `--space-1..10` = 5/10/20/30/40/
  60/80/120/160/200px; `--fs-*` = 20/32/52/84/14/16px; no 22/1100/2/6/3/9/10/18px
  token exists), and the codebase's own pre-existing `globals.css` already uses the
  identical convention dozens of times (`-1px` row-overlap throughout; `width:14px`,
  `width:6px`, `font-size:13px`, `padding:2px 6px` in the pre-existing `.ws-*`
  rules) — replacing these with an approximated token would DIVERGE from the
  passing w19 vision-judge baseline, not converge on it. Codex's suggested
  `--pad-button-*` token doesn't exist in tokens.css either.
- Finding 6 SKIP (below threshold): no spec asserts ARIA live-region behavior;
  reasonable but purely additive polish, not demanded by any driving red spec.
- Applied 1+2 together: `harness/src/app/page.tsx` — per-tick `AbortController`
  with a new `SESSIONS_REQUEST_TIMEOUT_MS` (10s, well under the 30s unavailable
  threshold) that aborts a hung request (classified as a poll failure, loop
  continues); `cancelled` re-checked after `await res.json()`; effect cleanup now
  calls `inFlight?.abort()`.
- Applied 3: new pure `decodeSessionsPayload()` in
  `harness/src/app/lib/sessions/poll.ts` (validates `{sessions: SessionSummary[]}`
  shape field-by-field) wired into `page.tsx`'s success branch — an invalid 200
  now routes through `applyFailure`, never `applySuccess([])`. 8 new Vitest cases
  in `harness/src/tests/unit/sessions-poll.test.ts` (`describe("decodeSessionsPayload")`).
  This is beyond what any w18/w19/p1 assertion directly demands (no spec sends a
  malformed 200) — pinned only by the new unit tests; flagged for the reviewer.
- Verification: `npx tsc --noEmit` (tests/harness + harness) clean; `npm run
  test:unit` 68/68 green (was 59); full re-run of w18 (10/10), w19 (1/1), p1/p2
  (2/2) all still pass after the change.

## Round 2 (verify + challenge + fresh)

- Prompt: `tmp/codex-home-sessions-list-fixer-r2-prompt.md` - Answer:
  `tmp/codex-home-sessions-list-fixer-r2.md` (+ `.trace.log`). Verification: V1/V2
  RESOLVED; **V3 NOT-RESOLVED** (botched application, re-fixed this round — counts
  as an applied finding for round 3 to verify). Challenges C4/C5/C6 all HELD (no
  action). 2 fresh findings: F1 kept, F2 not actionable (see below).

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Disp |
|---|---|---|---|---|---|---|---|
| V1 | Hung-request timeout — verified correctly implemented | - | - | - | - | - | RESOLVED |
| V2 | Unmount cancellation — verified correctly implemented | - | - | - | - | - | RESOLVED |
| V3 | `decodeSessionsPayload`'s `isSessionSummary` never validates `pid` (declared on the `SessionSummary` interface, part of the wire contract) despite claiming "validates every field" | 8 | Y | Y | Y | Y | FIX |
| C4 | Tie-breaker skip — challenged, held (w18.7 itself treats ties as untestable) | - | - | - | - | - | HELD |
| C5 | Token-literal skip — challenged, held (values are prototype-source copies, no matching tokens exist) | - | - | - | - | - | HELD |
| C6 | aria-live skip — challenged, held (not demanded by any spec) | - | - | - | - | - | HELD |
| F1 | The new AbortController/timeout/cleanup poll-loop wiring (V1/V2's fix) has NO unit-test coverage — only the pure state functions are tested | 7 | Y | Y | Y | Y | KEEP |
| F2 | Codex's own sandbox got `EPERM` running `npm run test:unit` (Vitest temp-dir creation) and could not independently reproduce the 68/68 result | - | - | - | - | - | N/A (env-local to Codex's sandbox; the agent re-ran the same command directly and confirmed 68/68 green, then 79/79 after this round's fixes) |

- Applied V3: `isSessionSummary` in `harness/src/app/lib/sessions/poll.ts` now also
  validates `pid` (`typeof === "number" && Number.isFinite`) and tightens
  `connected_at` to `Number.isFinite` too. 4 new malformed-entry cases added to
  `sessions-poll.test.ts`'s `decodeSessionsPayload` table (missing pid, wrong-typed
  pid, non-finite pid, non-finite connected_at).
- Applied F1: extracted `pollOnce(fetchImpl, signal)` — a pure(-ish), dependency-
  injected fetch-outcome classifier (`FetchLike`/`FetchLikeResponse` types) — out
  of `page.tsx`'s effect into `poll.ts`; `page.tsx`'s `tick()` now calls
  `pollOnce(fetch, controller.signal)` instead of inlining fetch/decode logic. 7
  new Vitest cases cover success / non-2xx / malformed-200 / rejected-fetch
  (network error or abort) / JSON-parse failure / argument pass-through / genuine
  zero-length success. The timer/scheduling/abort-on-unmount loop itself stays in
  `page.tsx` (real-timing behavior already covered by w18.9's e2e case; a fake-
  timer unit harness for the scheduling loop itself was judged not worth the
  added complexity/regression-risk for a task-blind test-fixer — flagged for the
  reviewer as a possible follow-up, not applied).
- Verification: `npx tsc --noEmit` (tests/harness + harness) clean; `npm run
  test:unit` 79/79 green (was 68); full re-run of w18 (10/10), w19 (1/1), p1/p2
  (2/2) all still pass after the change.

## Round 3 (final — cap reached)

- Prompt: `tmp/codex-home-sessions-list-fixer-r3-prompt.md` - Answer:
  `tmp/codex-home-sessions-list-fixer-r3.md` (+ `.trace.log`). 2 findings (labeled
  "V1"/"F1" by Codex, reclassified below by what they actually are).

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Disp |
|---|---|---|---|---|---|---|---|
| a | Re-raise of round-2's declined sub-decision: the poll loop's scheduling/timeout/abort-on-unmount lifecycle (not just `pollOnce`'s classification) has no fake-timer unit coverage | 6 | Y | Y | ~ | **FAIL** | SKIP (reaffirmed) |
| b | `.sessions-app-shell{min-height:100vh}` never actually bounds the box — `overflow-y:auto` has nothing to scroll against, so a list longer than the viewport is silently clipped by the page-wide `html,body{overflow:hidden}` | 8 | Y | Y | Y | Y | KEEP |

- Finding (a) SKIP, reaffirming round 2's conscious non-application: mocking
  `setTimeout`+`fetch`+`AbortController` together to unit-test the SCHEDULING loop
  is exactly the fragility class `w18-sessions-home.spec.ts`'s own header comment
  documents rejecting for this precise codepath ("the sessions home ... whose
  poll+debounce+React re-render cannot be guaranteed to advance under
  Playwright's fake clock ... a clock-driven contract risks turning a compliant
  impl into a hang" — the SAME reasoning applies to a Vitest fake-timer harness
  around the identical `setTimeout`-driven loop). The real-time behavior is
  already gated end-to-end by w18.9 with a generous, passing margin. Regression-
  risk gate fails: chasing this coverage would add exactly the flakiness class
  the driving spec explicitly designed around.
- Finding (b) KEEP (real, cheap, zero-risk correctness fix in code I authored —
  not a spec-driven behavior, but a bugfix to my own implementation, not new
  behavior). Applied: `harness/src/app/globals.css`'s `.sessions-app-shell` now
  uses `height: 100vh` (a bounded box) instead of `min-height: 100vh`, so
  `overflow-y: auto` becomes a real scroll container for any session count
  larger than the two-session e2e fixtures exercise.
- Verification: `npx tsc --noEmit` (tests/harness + harness) clean; `npm run
  test:unit` 79/79 green (unchanged — pure CSS fix, no new logic to test); full
  re-run of w18 (10/10), w19 (1/1), p1/p2 (2/2) all still pass after the change.

## Outcome (test-fixer)

Loop closed at the fixed 3-round cap (no early exit, per the test-fixer's
non-negotiable "fixed 3 rounds, always" rule). Round 1: 6 findings, 3 kept
(hung-request timeout, unmount-cancellation, malformed-200-as-failure), 3 skipped
(tie-breaker — spec itself treats it as untestable; CSS-token-literal claim —
values are verified 1:1 prototype-source copies with no matching token; aria-live
— not demanded). Round 2: 1 botched round-1 application re-fixed (`pid` validation
gap), 1 fresh finding applied (extracted `pollOnce` for unit-testable fetch-
outcome classification, +7 cases), 3 challenged skips all held. Round 3: 1
reaffirmed skip (fake-timer coverage of the scheduling loop — explicitly rejected
per the driving spec's own documented rationale against clock-driven control of
this codepath), 1 fresh fix applied (a real `min-height`→`height` CSS scroll-
container bug in the shell). Net: the sessions-home poll loop is now robust to a
hung request, an unmounted-component race, and a malformed API response — all
beyond what any single e2e assertion directly exercises but consistent with the
README-testids.md P6/P9/P10 contract — and a real (if untested-by-e2e) CSS
overflow bug was fixed. Every round verified `npx tsc --noEmit` clean and the
full w18/w19/p1/p2 suite green before moving on; final state: 79/79 unit tests,
13/13 e2e specs (10 w18 + 1 w19 + p1 + p2) passing.

---

# Codex ledger — `home-sessions-list` (REVIEWER)

Final review, hard lane, ≤3 rounds. Read-only Codex; the AGENT scores (composite
1-10 + four gates: Correctness / Evidence / Scope fit / Regression risk). Keep only
composite >=7 AND all four gates pass. Artifacts: prompt
`tmp/codex-home-sessions-list-reviewer-rN.md` (also the answer file — codex.sh's
`-o` overwrites the prompt path after stdin is consumed; Codex read the full prompt
via stdin first, so each round is intact), trace `.trace.log`.

## Round 1 (fresh findings only)

- Prompt/answer: `tmp/codex-home-sessions-list-reviewer-r1.md`. 7 findings; 6 KEPT,
  1 informational (acceptable regeneration-loss list — no action, recorded in
  residual risk).

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Disp |
|---|---|---|---|---|---|---|---|
| 1 | Hung read (vs 503) not e2e-pinned; a regen without a per-request timeout stalls the poll loop forever, notice never appears | 7 | Y | Y | Y | Y | KEEP |
| 2 | test-connection drop is testid-only — a faithful prototype button WITHOUT the retired testid passes (most-likely wrong regen) | 8 | Y | Y | Y | Y | KEEP |
| 3 | P5 affordance check is testid-only — a `<button>Reconnect</button>`/`Disconnected` banner with no reserved id or dead session-id slips through | 7 | Y | Y | Y | Y | KEEP |
| 4 | Unavailable notice can be a BLANK box and pass — w18.9 never asserts explanatory copy; copy pinned by no test | 7 | Y | Y | Y | Y | KEEP |
| 5 | Malformed-200-as-failure pinned only by gitignored unit tests; a naive `?? []` regen shows "No sessions connected." on garbage | 8 | Y | Y | Y | Y | KEEP |
| 6 | Acceptable regeneration-loss list (sort immutability, firstFailureAt bookkeeping, 3s interval, cache/signal, abort-on-unmount) | - | Y | Y | Y | Y | INFO (residual risk) |
| 7 | w19.1 row readiness is best-effort 5s → slow CI feeds the judge a loading frame → flaky false design-fail | 7 | Y | Y | Y | Y | KEEP |

- Applied 1: NEW `w18.11` — hung route (never fulfills) must still surface the
  unavailable notice within 80s + recover (pins per-request-timeout resilience). No
  production timing change (current impl surfaces at ~52s, literally P10-compliant
  ">~30s"; changing the failure-onset model adds regression risk to the passing
  w18.9). `test.setTimeout(240_000)`.
- Applied 2: `w18.10` — assert no button/link with accessible name
  `/test[- ]?connection/i` (count 0), in addition to the testid-0 check.
- Applied 3: `w18.5` — the MutationObserver scan now also records a synthetic
  `labelled-affordance` marker when any button/link/`role=status|alert` element
  carries `/\b(reconnect|disconnect|unreachable)/i` text or aria-label — scoped to
  controls/status roles so the prototype's legitimate prose never false-fails.
- Applied 4: `w18.9` — `toContainText(/unavailable|unable to load/i)` on the notice.
- Applied 5: NEW `w18.12` — a malformed 200 (`{garbage:true}`) must NEVER render the
  honest empty state across ~8s of malformed polls; a genuine `{sessions:[]}` then
  does. Catches the naive `?? []` regen.
- Applied 7: `w19.1` — REQUIRE the production row visible (30s) before the parity
  screenshot instead of a 5s best-effort catch.
- `npx tsc --noEmit` (tests/harness) green after all six.

## Round 2 (verify + challenge + fresh)

- Prompt: `tmp/codex-home-sessions-list-reviewer-r2.md` · Answer:
  `tmp/codex-home-sessions-list-rev2-out.md` (+ `.trace.log`). Verification: R1
  findings 1/2/4/5/7 RESOLVED; **finding 3 (w18.5 affordance scan) NOT-RESOLVED** —
  the MutationObserver omitted `characterData`, so a text-node-only flash to a
  forbidden VOCAB word could evade the scan (re-fixed this round → counts as an
  applied finding for round 3 to verify). All acceptances + standing skips HELD. No
  fresh finding beyond the characterData gap.

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Disp |
|---|---|---|---|---|---|---|---|
| V3 | w18.5 observer omits `characterData` → a text-only flash to "Disconnected"/"Reconnect" is missed | 7 | Y | Y | Y | Y | FIX |

- Applied V3: `w18.5` observer options now include `characterData: true`.
- `npx tsc --noEmit` (tests/harness) green.

## Round 3 (final — cap reached, DRY)

- Prompt: `tmp/codex-home-sessions-list-reviewer-r3.md` · Answer:
  `tmp/codex-home-sessions-list-rev3-out.md` (+ `.trace.log`). Verification: V3
  (w18.5 `characterData`) RESOLVED — the observer now catches text-node-only flashes
  and cannot false-fail the current markup. All standing skips/acceptances HELD under
  challenge. NO fresh findings. Round is DRY.

## Outcome (reviewer)

Loop closed at the hard-lane cap of 3 rounds; round 3 was DRY. R1: 7 findings, 6 kept
(all regeneratability/assertion-strength hardening), 1 informational (accepted
regeneration-loss → residual risk). R2: 5/6 applications verified RESOLVED, 1
re-fixed (`w18.5` `characterData`), all acceptances + standing skips held, no new
finding. R3: re-fix verified, dry. Net: pinned two user-observable behaviors that
were unit-only (hung-read resilience → `w18.11`; malformed-200-as-failure →
`w18.12`), strengthened the P7 Test-connection drop and P5 affordance-absence proofs
against faithful-prototype regeneration (accessible-name + role-scoped vocabulary
scans), pinned the unavailable-notice copy (`w18.9`), and removed a `w19.1` slow-CI
false-fail. `npx tsc --noEmit` (tests/harness) green throughout.
