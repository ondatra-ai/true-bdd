# Codex rounds — design-conformance-tests (test-author)

Lane: easy → cap 1 round. Codex is read-only; the agent scores every finding
(composite 1–10 + four gates: Correctness / Evidence / Scope-fit / Regression-risk).
Keep only composite ≥7 AND all gates pass.

## Round 1 of 1
- Prompt: `tmp/codex-design-conformance-r1.md`
- Answer: `tmp/codex-design-conformance.md` (+ `.trace.log`)

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| F1 | color sweep ignores SVG fill/stroke, outline, text-decoration, shadow, gradient stops | 6 | ✓ | ✓ | partial | risk of false-positives | SKIP — workspace uses none of these ad-hoc (system forbids shadows/icons; gradient only on non-workspace pages); broad parsing adds false-positive risk to a permanent gate |
| F2 | visibility checked on element only → opacity:0/hidden ancestor's children swept as visible (false positives) | 8 | ✓ | ✓ | ✓ | ✓ (fewer FPs) | KEEP — use `checkVisibility({opacityProperty,visibilityProperty})` + rect guard in both sweeps |
| F3 | text checks require a direct text node → `<textarea>` editor / inputs escape color+font check | 7 | ✓ | ✓ | ✓ | ✓ | KEEP — treat input/textarea/select as text-bearing in both sweeps |
| F4 | monospace exception hard-coded to a small family list (SF Mono / Cascadia rejected) | 7 | ✓ | ✓ | ✓ | ✓ (fewer FPs) | KEEP — accept any `mono`-primary or generic `monospace` fallback |
| F5 | `document.fonts.check()` doesn't prove per-element Poppins paint | 5 | ✓ | ✓ | out (pixel-proof beyond R1) | — | SKIP — family sweep + fonts.check is a defensible deterministic proxy; per-pixel paint proof is disproportionate/out of R1 scope |
| F6 | schema allows `checks:[]` / missing names → `{"verdict":"pass","checks":[]}` born green | 9 | ✓ | ✓ | ✓ | ✓ | KEEP — schema minItems/maxItems=4 AND runtime `auditVerdict` completeness check |
| F7 | verdict only type-cast, not runtime-validated for completeness/consistency | 8 | ✓ | ✓ | ✓ | ✓ | KEEP — `auditVerdict` (names present once, verdict↔checks consistent); spec asserts problems==[] |
| F8 | `testInfo.attach()` calls unawaited (unhandled rejection / missing artifacts) | 7 | ✓ | ✓ | ✓ | ✓ | KEEP — awaited both |
| F9 | fixed repo-global tmp path clobbers across runs | 6 | ✓ | ✓ | ✓ | low | KEEP (partial) — keep under `tmp/design-judge/` (orchestrator-required artifacts dir) but per-test label `…-${testId}` |
| F10 | judge infra failure (timeout/auth/no verdict) throws before any assertion → crash not assertion | 5 | partial | ✓ | out | — | SKIP — fail-loud on a broken judge is correct; converting to skip could mask a broken gate. Our env has codex working, so w9 produces a real assertion verdict |

Kept: F2, F3, F4, F6, F7, F8, F9. Skipped: F1, F5, F10 (reasons above).
Round dry? No — kept findings applied; at lane cap (1), loop ends.

# Codex rounds — design-conformance-tests (test-fixer)

Fixed 3 rounds, every run, no early exit (test-fixer lane rule — independent of
task lane). Task-blind: grounded in the reproduce block + `w8-design-tokens.spec.ts`
/ `w9-design-judge.spec.ts` / `helpers/design-conformance.ts` (read from disk) +
the full current production/unit-test diff, per round. Codex is read-only; the
agent scores every finding (composite 1–10 + four gates: Correctness / Evidence /
Scope-fit / Regression-risk). Keep only composite ≥7 AND all gates pass.

Baseline (before any fix): 3 failed, 0 passed (w8.1 color, w8.2 font, w9.1
persistent_frame + breadcrumb_hairline) — matches the supplied reproduce block
verbatim; no drift.

Fix applied before round 1: (a) global CSS reset so `button/input/textarea/select`
inherit the Poppins font + token colour from `body` instead of the UA default
(fixed w8.1/w8.2); (b) a new persistent breadcrumb bar (`content-breadcrumb`,
pure `breadcrumbCrumbs()` helper + unit tests, `workspace-main` wrapper column)
with a token hairline bottom border above the content pane (fixed w9.1's
`persistent_frame`/`breadcrumb_hairline`). Confirmed green pre-round-1: literal
reproduce command → `3 passed`; `tsc --noEmit` → exit 0.

## Round 1 of 3
- Prompt: `tmp/codex-design-conformance-fixer-r1.md`
- Answer: `tmp/codex-design-conformance-fixer-r1.md` (`-o` output; trace:
  `tmp/codex-design-conformance-fixer-r1.trace.log`)

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| F1 | form controls only visible in OTHER states (chat open: `chat-dock-new/-send/-input`; feature-picker/new-story-form) still resolve UA-default off-token `background-color`/`border-color` (Chromium button `rgb(239,239,239)` + black outset border; input `rgb(118,118,118)`) — the r1 reset only touched `font-family`/`color` | 8 | ✓ (empirically verified via a real Chromium `getComputedStyle` probe, not just Codex's claim) | ✓ | ✓ — same defect class as w8's own token-gate concern, closes an incomplete application of the very reset I'd just added, in the same file | ✓ — visual-only change; no test asserts these controls' background/border; matches the existing `.ws-picker-option` (`background:transparent;border:none`) convention already in the file | KEEP — extended the reset: `button{background-color:transparent;border:none}` (matches `.ws-picker-option`'s existing convention); `input,textarea,select{background-color:transparent;border:var(--border-width) solid var(--border-hairline)}` (matches the hairline-border convention used throughout — sidebar/file-view/chip). Elements with their own higher-specificity rule (`.ws-rail-utility`, `[data-testid="chat-dock-toggle"]`, `.ws-picker-option`) are unaffected. |
| F2 | `content-pane` padding (`var(--space-3)`=20px) doesn't match `design/SPEC.md` §1's "40px inner padding" / mockup's `var(--space-5)`=40px `.mockup-canvas` | 5 | ✓ | ✓ | ✗ — `canvas_padding` is NOT a failing check (it PASSED in both the baseline red run and my green run per the reproduce block's own note); the nondeterministic vision judge already accepts 20px as "generous"; not demanded by any currently-failing assertion | ✗ — global change affecting every page's content-pane, incl. the hard-coded `.ws-file-view-body{height:calc(100vh - 220px)}` magic number tied to current layout assumptions; unverified blast radius across w1–w7's bounding-box assertions | SKIP — out of scope (Spec-as-Source: not demanded by any red test) and unverified regression risk for a check that already passes |

Kept: F1. Skipped: F2 (reasons above).
Post-round-1 verification: `harness && npm run typecheck` exit 0; `npm run
test:unit` 29/29 passed; full `--project=workspace` suite (39 specs) run to
check for regressions from the DOM/CSS change — 38 passed, 1 failed
(`w4-persistence.spec.ts` w4.1–w4.3, timing out on `file-view-editor` after a
full container **restart** + reconnect + reload, unrelated to any file I
touched); isolated re-run of `w4-persistence.spec.ts` alone in progress to
confirm pre-existing flake vs regression before deciding round 2's scope.
Round dry? No — 1 keep applied.

Follow-up confirmation: isolated re-run of `w4-persistence.spec.ts` alone
(4 tests) → **4/4 passed**, including w4.1–w4.3. Confirms the full-suite
failure was a pre-existing timing flake (container **restart** + reconnect
under concurrent Docker/CPU load from the just-finished 39-spec run), not a
regression from this task's changes.

## Round 2 of 3
- Prompt: `tmp/codex-design-conformance-fixer-r2.md`
- Answer: `tmp/codex-design-conformance-fixer-r2.md` (`-o` output; trace:
  `tmp/codex-design-conformance-fixer-r2.trace.log`)

Round 2 asked Codex to (a) verify F1's application, (b) challenge the F2 skip,
(c) find fresh issues.

**(a) F1 verification: RESOLVED.** Codex read the current `globals.css` cascade
line-by-line (button/input reset → `.ws-rail-utility` → `.ws-picker-option` →
`[data-testid="chat-dock-toggle"]` → `[data-testid="file-view-editor"]` →
non-form elements) and confirmed every element that needs its own
background/border still has it via higher specificity/later declaration; no
accidental loss.

**(b) F2 challenge: overturned — I now agree, reversing the round-1 skip.**
Codex's case: `design/SPEC.md` §1's frame diagram literally states "canvas
(content, 40px inner padding)" and the mockup implements that exact number
(`var(--space-5)`, `mockups.css:280`) — `design/SPEC.md` is the design-system's
declared spec artifact (paths.yaml → design_system) that `w9`'s rubric is
itself derived from, so conforming to its explicit number is fixing a
concrete, textually-specified deviation, not scope creep. `canvas_padding`
passing today does not prove it's safe long-term — the judge is a real,
nondeterministic vision model, so a prior pass is not a stability guarantee.
Verified independently: `design/SPEC.md:26` and `mockups.css:280` both say
40px / `var(--space-5)`; re-read `w6-chat.spec.ts`/`w7-shell-quality.spec.ts`
— neither asserts an exact `content-pane` padding or height. New score:
composite 8, all 4 gates pass. **KEEP** — changed `content-pane` padding
`var(--space-3)` → `var(--space-5)` in `harness/src/app/globals.css`.

**(c) Fresh findings:**

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| F3 | `breadcrumbCrumbs()` linkifies EVERY intermediate segment, but `product/stories` has no index page (only `product/stories/[storyId]`, per `routes.ts`/the app router tree) — a story-detail breadcrumb's "Stories" crumb would link to a 404. Not caught by w9 (rubric explicitly ignores label/content, judges the bar visually only). | 8 | ✓ (verified: no `product/stories/page.tsx` exists, only `[storyId]/page.tsx`; every other collection segment does have an index page) | ✓ | ✓ — fixing a real bug in code I wrote this session, not new scope | ✓ — additive `linkable` field, only changes the one "stories" crumb's render (span instead of Link); everything else unchanged | KEEP — added `BreadcrumbCrumb.linkable` + a `NON_ROUTABLE_SEGMENTS` set (currently just `"stories"`); `Breadcrumb` renders a non-linkable, non-current crumb as plain text instead of `<Link>`. Unit test added for the nested-story case. |
| F4 | Unconditional `decodeURIComponent(segment)` throws `URIError` on a malformed percent-escape (e.g. a literal `%` in the pathname); since `Breadcrumb` renders inside the PERSISTENT `WorkspaceShell`, one bad segment would crash the entire app shell, not just that crumb. Not exercised by any current w1–w9 route (defensive fix). | 8 | ✓ (verified: `decodeURIComponent("%")` really throws `URIError` in Node) | ✓ | ✓ — hardening code I wrote this session; no new feature | ✓ — pure additive try/catch, identical output on every currently-valid path | KEEP — added a `safeDecode()` wrapper (catches `URIError`, falls back to the raw segment) used in place of the bare `decodeURIComponent` call. Unit test added for a malformed-escape segment. |

Also re-confirmed (Codex, static-only per this round's read-only/no-e2e
constraint): the `content-pane` → `workspace-main` restructuring breaks no
current w1–w7 assertion (`w6.1`'s relative width/right-edge check and `w7.3`'s
scroll-ownership check are the only relevant ones, and neither depends on the
DOM nesting level or the padding value). `harness && npm run typecheck`,
`tests/harness && npx tsc --noEmit`, and a targeted ESLint on the changed
files all passed; `npm run test:unit` couldn't run inside Codex's read-only
sandbox (Vitest needs to write a temp SSR dir) — not a defect, just an
environment limit of the read-only critique sandbox (the agent runs the real
suite itself outside that sandbox, see below).

Kept: F2 (reversed from round-1 skip), F3, F4. Skipped: none this round.
Applied: `content-pane` padding → `var(--space-5)`; `breadcrumb.ts` gained
`linkable` + `NON_ROUTABLE_SEGMENTS` + `safeDecode()`; `WorkspaceShell.tsx`'s
`Breadcrumb` renders non-linkable crumbs as text; 2 new unit tests (now 31
total, all passing) plus updated existing ones for the new `linkable` field.
Post-round-2 verification: `npm run typecheck` exit 0; `npm run test:unit`
31/31 passed; full `--project=workspace` e2e suite (39 specs) re-run to
confirm no regression from the padding/breadcrumb changes — **39/39 passed**
(the earlier w4 flake did not recur).
Round dry? No — 3 keeps applied (1 reversed skip + 2 fresh).

## Round 3 of 3 (final — mandatory regardless of round 2's outcome)
- Prompt: `tmp/codex-design-conformance-fixer-r3.md`
- Answer: `tmp/codex-design-conformance-fixer-r3.md` (`-o` output; trace:
  `tmp/codex-design-conformance-fixer-r3.trace.log`)

Asked Codex to re-verify F1–F4 in the current state (cross-checking F3 against
the actual `routes.ts` + physical `app/sessions/[sid]/(workspace)/**/page.tsx`
tree, and F2's padding change against the `file-view-body` magic-number height
at both 900px and 938px viewports) and to look for any final fresh finding.

| # | Verdict | Evidence |
|---|---|---|
| F1 | RESOLVED | Current `globals.css` reset confirmed still in place; more-specific component rules (`.ws-rail-utility`, `[data-testid="chat-dock-toggle"]`, `.ws-picker-option`, `[data-testid="file-view-editor"]`) correctly override it. |
| F2 | RESOLVED | `content-pane` padding is `var(--space-5)` (40px), matching `SPEC.md` and `.mockup-canvas`. `calc(100vh - 220px)` file-view height verified sane (680px @900px viewport, 718px @938px); any overflow is absorbed by `content-pane`'s own `overflow-y: auto`. |
| F3 | RESOLVED | Cross-checked the full route tree: index pages exist for `home`/`architecture`/`product`/`product/features`/`product/scenarios`/`builds`; only `product/stories` lacks one (`[storyId]/page.tsx` only) — exactly what `NON_ROUTABLE_SEGMENTS` targets. Reused `.ws-breadcrumb-link` styling for the non-linkable `<span>` confirmed still token-only (`var(--text-muted)`, inherited Poppins) — no new w8 exposure. |
| F4 | RESOLVED | `safeDecode()` cannot throw for any `string` input — verified the try/catch covers every `decodeURIComponent` failure mode. |
| Fresh | **None** | Codex explicitly returned a clean bill of health: "no remaining correctness, scope-fit, token-conformance, layout, or regression-risk issue." |

Kept: none (nothing new to apply — round 3 is a pure verification pass).
Skipped: none. **Round dry** (every prior application verified clean, no
fresh finding passed the gates) — this is round 3, the fixed cap, so no
further round is run regardless.

## Final state

- `harness && npm run typecheck` → exit 0.
- `harness && npm run test:unit` → 31/31 passed.
- `tests/harness && npx tsc --noEmit -p tsconfig.json` → exit 0.
- `cd tests/harness && TRUE_BDD_E2E_SKIP_BUILD=1 npx playwright test --project=workspace w8-design-tokens.spec.ts w9-design-judge.spec.ts` → **3 passed**.
- `cd tests/harness && npx playwright test --project=workspace` (full w1–w9, 39 specs) → **39 passed** (post round-2 fixes; the one w4 flake seen mid-loop did not recur and was independently confirmed as pre-existing/timing-related, not a regression).

# Codex rounds — design-conformance-tests (reviewer / final review)

Lane easy → floor 1 round. Round 1 produced MULTIPLE keeps → escalated to the
full cap (≤3) and orchestrator notified. Codex read-only; the agent scores
(composite 1–10 + four gates) and applies. Prod change surface this phase:
NONE — reviewer hardened the e2e tests + contract only (no production edits;
the fixer's prod changes were verified live, see smoke below).

## Round 1 of ≤3
- Prompt: `tmp/reviewer-r1-prompt.md` (first launch collided its `-o` path with
  the prompt file → empty prompt; re-launched clean, label `reviewer-r1`).
- Answer: `tmp/codex-reviewer-r1.md` (trace `tmp/codex-reviewer-r1.trace.log`).

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| R-High-1 | w9 vision judge is soft on EXACT dims (SPEC §1: 280px sidebar / 48px breadcrumb / 40px canvas); prod sidebar is 260px, breadcrumb has no 48px height → judge passes anyway. Fix: add deterministic bbox asserts + change prod dims. | 6 | partial (SPEC does state the numbers) | ✓ | ✗ — the fix mutates the PRE-EXISTING 260px sidebar, which is NOT in this task's diff; a hard 280px assert forces a workspace-wide layout change | ✗ — changing sidebar width risks w6.1 relative-width / w7.3 scroll asserts | SKIP — scope + regression gates fail; recorded as residual risk (the exact-dimension gate belongs in a follow-up layout task, not this tests-only pass) |
| R-High-2 | w8 sweeps ONLY the architecture page, chat CLOSED → the chat panel / feature-picker / new-story-form form controls (the very ones the fixer's global reset fixed) are never swept — a permanent-gate coverage hole. | 8 | ✓ (verified: both w8 tests call only `openArchitecture`) | ✓ | ✓ — strengthening the token gate is the task's whole point | ✓ — empirically verified via the live smoke that the chat-open controls are token-conformant, so the added sweep stays green (locks in the fix, doesn't force new prod work) | KEEP — added `w8.3`: open the docked chat, then colour+font sweep. (Scoped to chat-open, the persistent always-reachable surface; feature-picker/new-story-form left as residual risk — still covered by the global reset, just not asserted.) |
| R-Med-1 | palette allow-set = EVERY hex in tokens.css incl. gradient stops `#B98CF5`/`#C9A7F2`; doesn't enforce `var(--token)` usage. Fix: parse only declared color custom-props + a static CSS gate rejecting literals in harness/src. | 5 | ✓ technically | ✓ | ✗ — the static-CSS-literal gate is a new, broader mechanism beyond this task; the workspace is monochrome so no element uses those gradient hexes (theoretical hole) | risk — a var()-enforcement gate has real false-positive surface | SKIP — low value + scope creep; the extra gradient hexes resolve to rgb no workspace element paints |
| R-Med-2 | w9 screenshots BOTH pages without awaiting `document.fonts.ready`; `font-display:swap` can capture the fallback face → judge sees wrong type / flakes. | 8 | ✓ | ✓ | ✓ (flakiness hardening of the new spec) | ✓ (await-only; safe) | KEEP — `await page.evaluate(async()=>{await document.fonts.ready})` before each of the prod + mockup screenshots |
| R-Med-3 | `runDesignJudge` returns any verdict regardless of `exitCode`; a codex that writes a passing verdict then exits nonzero passes w9. | 7 | ✓ | ✓ | ✓ | ✓ — a healthy run exits 0, so asserting it never bites the happy path; only a broken judge fails | KEEP — `expect(exitCode).toBe(0)` in w9 after the artifact attaches (verdict untrustworthy on nonzero exit) |
| R-Low-1 | new persistent `content-breadcrumb` / `workspace-main` testids absent from the binding contract (`README-testids.md`) + typed source (`WTID` in `ui.ts`). | 7 | ✓ (grep: no matches) | ✓ | ✓ (keep the contract truthful for the new permanent shell region) | ✓ (doc + append-only WTID keys) | KEEP — documented both in README-testids.md App-shell table + added `WTID.workspaceMain`/`WTID.contentBreadcrumb` |

Kept: R-High-2, R-Med-2, R-Med-3, R-Low-1 (4). Skipped: R-High-1, R-Med-1.
Multiple keeps → orchestrator notified + escalated to the full cap.

## Round 2 of ≤3
- Prompt: `tmp/reviewer-r2-prompt.md`; Answer: `tmp/codex-reviewer-r2.md`
  (trace `tmp/codex-reviewer-r2.trace.log`). Asked to (a) verify the 4 applied
  fixes, (b) challenge the 2 skips, (c) fresh findings.

Verification (a): Codex confirmed all four round-1 applications correct + complete
(w8.3 sweeps chat-open, fonts.ready awaits precede both screenshots, exitCode
assertion unbypassable, WTID/doc consistent) — `tsc --noEmit` green.

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| R2-High | (challenge of R-High-1) A SAFE test-ONLY dimension gate DOES exist: assert the ALREADY-CORRECT values THIS task set — canvas padding = `--space-5` (40px) + breadcrumb `--border-width` hairline — WITHOUT touching the pre-existing 260px sidebar. | 8 | ✓ (globals.css:123 padding var(--space-5); :91 border-bottom var(--border-width)) | ✓ | ✓ — asserts the design values THIS task introduced; explicitly avoids the out-of-scope sidebar | ✓ — token-anchored, no prod change, no w1–w7 dependency on content-pane padding/breadcrumb border | KEEP — added `w8.4`: token-anchored deterministic assert of content-pane 40px padding (4 sides) + breadcrumb solid `--border-width` hairline. Overturns R-High-1's blanket skip with the safe subset. |
| R2-Med-1 | w8.3 asserts only the panel; the collectors silently skip invisible/absent controls, so w8.3 could pass VACUOUSLY. Assert chatDockNew/Input/Send visible first. | 8 | ✓ | ✓ | ✓ (hardens the w8.3 just added) | ✓ (smoke-verified those controls are visible on chat-open → stays green) | KEEP — added `expect(...).toBeVisible()` for chat-dock-new / -input / -send before both sweeps. |
| R2-Med-2 | colour sweep ignores pseudo-elements/background-image/outline/box-shadow/text-decoration/SVG fill+stroke; concrete escapee `rgba(1,115,215,0.15)` (spiral-blue@15%) at globals.css:314-320 (file-view flash). | 5 | partial (the flash is a TOKEN colour at reduced alpha — legitimate, not an ad-hoc colour) | ✓ | ✗ — broad paint-property parsing is exactly the test-author's SKIPPED F1 (false-positive risk); the design forbids shadows/icons | ✗ — sweeping alpha-token/pseudo paint would FALSE-FAIL legitimate token-at-alpha usage (the flash), forcing prod changes or gate special-cases | SKIP — reintroduces the dispositioned false-positive risk; recorded as residual risk (the token gate models full-opacity token colours only; transient/pseudo/alpha paint like the file-view flash is unswept). |

Kept: R2-High, R2-Med-1 (2). Skipped: R2-Med-2. Not dry → round 3 (final).

## Round 3 of 3 (final — mandatory at the escalated cap)
- Prompt: `tmp/reviewer-r3-prompt.md`; Answer: `tmp/codex-reviewer-r3.md`
  (trace `tmp/codex-reviewer-r3.trace.log`).

Verification (a): Codex confirmed both round-2 applications correct + complete
(w8.4 token-anchored asserts match globals.css; w8.3 visibility gates reference
real testids); `tsc --noEmit` green. Challenge (b): both standing skips
(R2-Med-2, R-Med-1) upheld — no safe in-scope tightening.

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| R3-Low | w8.4 asserts the breadcrumb border WIDTH + STYLE but not that its COLOUR is specifically `--border-hairline` (w8.1 only gates it to SOME palette colour); the SPEC contract is `border-bottom: var(--border-width) solid var(--border-hairline)`. | 7 | ✓ (globals.css:91 uses --border-hairline; w8.4 didn't check colour) | ✓ | ✓ — completes the w8.4 assertion just added; anchors to the exact prod contract | ✓ — prod IS --border-hairline, so probe-resolved compare stays green | KEEP — w8.4 now probe-resolves --border-hairline and asserts `crumb.borderBottomColor` equals it (+ rejects an unresolved token). |

Kept: R3-Low (1). Skipped: none. Round 3 is the escalated cap → loop ends
regardless (R3-Low applied + self-verified via `tsc` + the final suite run below).

## Final state (reviewer)

- e2e suite typecheck `npx tsc --noEmit -p tests/harness/tsconfig.json` → exit 0.
- `harness && npm run test:unit` → 31/31 passed. `npm run typecheck` → clean.
  (`npm run lint`: 2 PRE-EXISTING errors in `harness/design/proto-workspace/*.js`
  — NOT this task's diff, out of scope.)
- Live smoke: CLI N/A (zero Go/`templates/`/`true-bdd/` changes — harness-UI-only
  task). UI driven interactively via Playwright MCP against a real workspace
  container: persistent breadcrumb bar renders with a hairline bottom border,
  three-region frame (rail/sidebar/canvas) intact, Home/Product/Features trail
  correct (links vs current), chat opens with token-conformant controls; only a
  benign favicon 404 in console.
- Final gate re-run recorded at the end of this file.

Final gate (reviewer, post-hardening):
`TRUE_BDD_E2E_SKIP_BUILD=1 npx playwright test --project=workspace
w8-design-tokens.spec.ts w9-design-judge.spec.ts` → **5 passed (49.3s)**
(w8.1 colours, w8.2 type, w8.3 chat-open sweep [NEW], w8.4 frame dims [NEW],
w9.1 codex vision judge 17.0s). No leftover containers/codex procs.
