# Codex ledger — product-section-prototype-parity (planner)

Loop: hard lane, ≤3 rounds. Read-only Codex suggests; the planner scores every
finding (composite 1–10 + four gates: Correctness, Evidence, Scope fit,
Regression risk) and keeps only composite ≥7 with ALL gates passing. Findings that
conflict with the tests-only constraint (no production design) or the brief's
non-goals are rejected regardless of score.

Artifacts: prompts `tmp/codex-product-parity-plan-rN.md`; answers
`tmp/codex-product-parity-plan.md` (+ `.trace.log`).

## Round 1 — fresh findings

Prompt `tmp/codex-product-parity-plan-r1.md`; answer `tmp/codex-product-parity-plan.md`.
17 findings. Gates: C=Correctness, E=Evidence, S=Scope-fit, R=Regression-risk.
Kept 16, skipped 1 (F16 merged into F10).

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| F1 | rail active tile targets wrong token (`--surface-inverse`); actually `--surface-page`+`--text-primary` | 8 | ✓✓✓✓ | KEEP — w12.2 now asserts surface-page bg + text-primary fg contrasting inactive + inverse rail (verified proto-extra.css:180) |
| F2 | R7 only tests FileView; escalate-only leaves fixer no red listing spec | 7 | ✓✓✓✓ | KEEP-partial — strengthened to a 3-source conflict + explicit conditional w13.4; REJECTED inventing feature-pill listing assertions (no real prototype scenarios surface has them) |
| F3 | R1 guide line/real-items/group anatomy/non-PRD selection get no new red assertions | — | C✗ | SKIP — already red-covered by w1.4/w7.1a/w5.1a/w1.6; re-asserting is redundant + out of the delta. Added a coverage cross-ref note instead |
| F4 | rail label case-insensitive compare passes title-case (no small-caps enforced) | 8 | ✓✓✓✓ | KEEP — assert computed `text-transform:uppercase` + `--ls-label` + full label (verified proto-extra.css:190) |
| F5 | R3/R6 file-card anatomy underspecified (no shared header bar / gutter-body siblings) | 7 | ✓✓✓✓ | KEEP — additive `file-view-header` bar containing path+count; gutter/editor siblings (monospace already in w7.2, not duplicated) |
| F6 | "muted subtitle" reduced to non-empty | 7 | ✓✓✓✓ | KEEP — assert `file-view-meta` color = `--text-muted` + below title (verified tokens.css:21) |
| F7 | R5 "aggregated live" only partial | 7 | ✓~✓✓ | KEEP-narrow — added the NEW Requirements-row picker reassign for E2E-601; REJECTED broad "live uncovered" (w5.4/w5.7 cover general live) |
| F8 | unaligned row omits linked title | 8 | ✓✓✓✓ | KEEP — assert unaligned row linked title + scenarios destination + `(none)` pill as siblings (verified feature/[id]/page.js:145) |
| F9 | story pill anatomy not pinned (single text button passes) | 7 | ✓✓✓✓ | KEEP — additive `feature-pill-label`/`value`/`change` distinct elements; only activation reveals picker |
| F10 | R8 false-pass via infra skips beyond the authorized codex skip | 9 | ✓✓✓✓ | KEEP — ONLY missing codex skips; every infra failure fails with logs+cleanup (merges F16) |
| F11 | launcher conflicts with `-p 3999` scripts / start needs build | 8 | ✓✓✓✓ | KEEP — spawn `next` binary directly w/ explicit `-p <port>` or cached build+start; parse ready URL from stdout |
| F12 | allocatePort race, no retry contract | 7 | ✓✓✓✓ | KEEP — bounded EADDRINUSE retries + fresh port + child-exit monitoring |
| F13 | readiness too weak (lazy route compile) | 8 | ✓✓✓✓ | KEEP — per-baseline-route navigate + route DOM anchor + fonts |
| F14 | npm install non-deterministic/network-dependent | 7 | ✓✓✓✓ | KEEP — prefer `npm ci` from lockfile only when deps absent; fail not skip; own timeout; not per-worker |
| F15 | teardown not guaranteed on boot/setup failure | 7 | ✓✓✓✓ | KEEP — retain child on spawn, finally-kill, graceful+forced, idempotent |
| F16 | R9 vs all-skip on boot failure | 8 | ✓✓✓✓ | MERGED into F10 |
| F17 | R3 edit round-trip not exact (YAML validity/receipt) | 7 | ✓✓✓✓ | KEEP — append YAML comment + await `save-state=saved`; reference w5.4a's receipt oracle rather than duplicate |
| F18 | plan contains production implementation guidance | 9 | ✓✓✓✓ | KEEP — removed files-to-touch + parameterization directives; reframed as observable test-side guards (Spec-as-Source) |
| F19 | breadcrumb change breaks the unit contract | 7 | ✓✓✓~ | KEEP-partial — reframed as Challenge + e2e guards (`breadcrumb.test.ts` verified to exist); REJECTED planner authoring unit tests (fixer's lane) |

## Round 2 — verify applied / challenge skip / fresh

Prompt `tmp/codex-product-parity-plan-r2.prompt.md`; answer `tmp/codex-product-parity-plan-r2.md`.
(a) verify: 13 RESOLVED (F2,F4,F5,F6,F7,F8,F9,F11,F12,F13,F14,F15,F17,F19), 3
NOT-RESOLVED (F1,F10/F16,F18 — primary fixed, but stale contradicting text remained
in Challenges/Startup). (b) challenge to skip F3: partially upheld. (c) 5 fresh.
Kept 9, skipped 1.

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| F1r | Challenges still asserted active tile vs `--surface-inverse` (contradicts fixed w12.2) | 9 | ✓✓✓✓ | KEEP — rewrote the Challenges token bullet to `--surface-page`+`--text-primary` |
| F10r | Challenges still authorized an npm/next skip (contradicts w15 skip discipline) | 9 | ✓✓✓✓ | KEEP — removed the npm/next skip; only missing codex skips; infra failures fail with logs |
| F18r | Startup still said "UI edits under harness/src"; R7 branch prescribed "a new production table page" | 8 | ✓✓✓✓ | KEEP — softened to name no production location; R7 branch now flags scope only, not a page |
| F3-chal | w1.4 does NOT assert the guide line; feature-row selection uncovered | 7 | ✓✓✓✓ | KEEP the CORRECTION (w1.4→w7.1a; selection is w1.6); REJECT re-adding guide-line/selection assertions — prod already renders them, so they'd be BORN GREEN (R9 fail-when-absent), not part of the delta |
| n1 | promised arch regression guard not in any w13 case | 7 | ✓✓✓✓ | KEEP — removed the false promise; cite existing w2/w3 as the arch file-view guard |
| n2 | R4 CHANGE control not tied to relay persistence (could be decorative) | 7 | ✓✓✓✓ | KEEP — w13.2 now picks a seeded feature through the pill + asserts disk persistence |
| n3 | feature-description only asserted non-empty | 7 | ✓✓✓✓ | KEEP — w14 asserts equality to the served features.yaml `summaries` description (node-scoped) + muted/below title |
| n4 | line-count oracle ambiguous (off-by-one risk) | 7 | ✓✓✓✓ | KEEP — w13.1 pins `readEditor().split("\n").length` (prototype formula), not newline count |
| n5 | successful-boot teardown unassigned (leak risk) | 7 | ✓✓✓✓ | KEEP — Startup now requires the spec's `test.afterAll` to call the singleton `stop()` |

## Round 3 — verify applied / challenge skip / fresh (final, cap reached)

Prompt `tmp/codex-product-parity-plan-r3.prompt.md`; answer `tmp/codex-product-parity-plan-r3.md`.
(a) verify: F1r/F10r/F3-correction + all 5 round-2 fresh RESOLVED; F18r still
incomplete (one stale Challenges clause). (b) F3 skip HOLDS — Codex verified
`SectionTree.tsx` renders the guide line in every group + `data-selected` on every
row, so re-asserting would be born green. (c) 2 fresh. Kept 3, skipped 0. Loop ends
at the hard-lane cap of 3 rounds (not dry, but capped).

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| F18r2 | Challenges R7 bullet still said "a new production table page is in scope" | 8 | ✓✓✓✓ | KEEP — reworded to "table LISTING spec / materially larger scope"; names no production surface |
| n6 | Startup "the design change lands in the runnable prototype" reverses the baseline direction | 8 | ✓✓✓✓ | KEEP — reworded: the prototype is the UNCHANGED baseline oracle; production is made to match it; prototype never edited |
| n7 | "Architecture untouched" not enforced — a fixer could leak the product header to the shared FileView; w2/w3 assert presence, not absence | 7 | ✓✓✓✓ | KEEP — w13.1 adds a NEGATIVE guard: product `file-view-kicker`/`file-view-title` count 0 on the architecture route (green now, red if leaked) |

Verified holds: F3 skip (born-green per SectionTree.tsx). All other round-1/2
applications RESOLVED.

---

# Test-author Codex loop (w12–w15 specs + helpers)

Hard lane, ≤3 rounds. Read-only Codex critiques the NEW tests + test-side helpers;
the test-author scores every finding (composite 1–10 + 4 gates C/E/S/R) and keeps
only composite ≥7 with ALL gates passing. Artifacts: prompts
`tmp/codex-product-parity-testauthor-rN*.md`; answers `tmp/codex-product-parity-testauthor-rN.md`.

## Round 1 — fresh findings

Prompt `tmp/codex-product-parity-testauthor-r1.md`. 8 findings. Kept 4 (F1,F3,F4,F8),
KEPT-narrow 1 (F6), skipped 3 (F2,F5,F7). Codex confirmed no R7/scope/typecheck defects.

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| F1 | w15 `npm ci` can outlive the 8-min test timeout (10-min install timer) → leaked installer + wedged teardown | 7 | ✓✓✓✓ | KEEP — install timeout 6min (< 12min test timeout), track `activeInstall` + SIGKILL it in `stopPrototype`, raise w15 timeouts to 12min |
| F2 | boot readiness is HTTP-only, not per-route browser DOM-anchor + fonts (F13 letter) | 6 | ✓~✗✓ | SKIP — `capture()` already does per-route anchor+font waits before EVERY screenshot; a broken client render FAILS at capture (F10), never greens. Injecting a browser into the plain node boot helper duplicates capture() |
| F3 | install falls back to `npm install` when lockfile absent (F14 = lockfile-only) | 7 | ✓✓✓✓ | KEEP — always `npm ci`; a missing lockfile now throws diagnostically |
| F4 | w13.2 first assertion (`editor` visible) is born-green; kicker (RED) is second | 7 | ✓✓✓✓ | KEEP — reordered so the missing `file-view-kicker` is the FIRST assertion |
| F5 | w15 capture anchors (rail/sidebar/file-view) pass before the judge verdict | 5 | ✓✓~✓ | SKIP — mirrors the established w9/w11 vision-judge pattern; readiness anchors are setup and CANNOT precede the screenshots; the judge verdict is the RED (no false green — a broken boot fails per F10) |
| F6 | placeholders admissible: rail icon `not.toBeEmpty`, meta non-empty | 7 | ✓✓~✓ | KEEP-narrow — `not.toBeEmpty` wrongly rejected non-text (svg/font) icons; now accepts glyph OR svg/img. REJECTED asserting prototype-LITERAL meta text ("Workspace — Inventory Spread") — prod is REAL; that copy is unsatisfiable, and the w15 vision judge already backstops trivial placeholders |
| F7 | breadcrumb filename + feature id are fixture literals | 5 | ✓✓~✓ | SKIP — those are the test's CHOSEN route inputs / plan-sanctioned stable facts; the rendered title/description/feature-value ARE parsed from served docs (no placeholder gain) |
| F8 | `sidebar-brand-meta` distinctness tested by text only, not DOM order | 7 | ✓✓✓✓ | KEEP — also assert the meta's boundingBox sits BELOW the brand name (distinct node + vertical order) |

## Round 2 — verify applied / challenge skips / fresh

Prompt `tmp/codex-product-parity-testauthor-r2.prompt.md`. (a) VERIFY: all 5 round-1
KEEPs (F1,F3,F4,F6,F8) confirmed correctly + completely applied. (b) CHALLENGE skips:
F2,F5,F7 skips all UPHELD (justified). (c) 2 fresh. Kept 2, skipped 0. tsc passed.

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| r2-1 | w13.4 asserts cell/link anatomy only for E2E-601; other 4 rows could be empty shells; empty-`user_stories` (E2E-604/INT-901) edge unspecified | 7 | ✓✓✓✓ | KEEP — loop over EVERY registry entry: id-link + description + service cells; linked-story link when user_stories non-empty, NO linked-story link (specified empty state) when empty |
| r2-2 | w14 checks link + picker EXIST but not the left/right layout R5 mandates (a stacked/reversed layout would pass) | 7 | ✓✓✓✓ | KEEP — `assertRowLayout` compares boundingBoxes: link LEFT of picker + same-row vertical overlap, for a story/requirement/unaligned row (prototype's `.list-row__primary`/`__side` is exactly left/right → satisfiable) |

## Round 3 — verify applied / challenge skips / fresh (FINAL, cap reached)

Prompt `tmp/codex-product-parity-testauthor-r3.prompt.md`. (a) VERIFY: r2-1 + r2-2
confirmed applied (r2-1 row coverage + empty-state correct; r2-2 geometry correct).
(b) CHALLENGE skips: F2/F5/F7 skips UPHELD. (c) 2 fresh test findings + 1 scope note.
Kept 2, skipped 1 (out-of-scope). Loop ends at the hard-lane cap of 3. tsc passed.

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| r3-1 | w13.4 linked-story `toHaveText(new RegExp(escapeRe(linkedId)))` is UNANCHORED — "60.2 extra" would pass | 7 | ✓✓✓✓ | KEEP — exact `toHaveText(linkedId)` (prototype shows exactly the id) |
| r3-2 | `assertRowLayout` force-unwraps `boundingBox()` w/o a visibility gate → a hidden node throws TypeError, not an assertion failure | 7 | ✓✓✓✓ | KEEP — `await expect(left/right).toBeVisible()` before reading boxes |
| r3-3 | worktree has `src/internal/app/inventory/{documents.go,scanner_honesty_test.go}` changes vs tests-only scope | — | S✗ | SKIP — PRE-EXISTING worktree WIP from another task; verified my patch touches ZERO src/ files; reverting the user's uncommitted work is out of scope + prohibited |

Verified holds: all round-1/2 keeps applied; F2/F5/F7 skips upheld; every deterministic
case's FIRST assertion fails against current prod as a Playwright assertion (RED-first);
`scenario-table` is w13.4's first absent-prod assertion; tsc --noEmit clean.

## RED verification (test-author, post-loop)

Typecheck gate: `cd tests/harness && npx tsc --noEmit` → exit 0 (clean).
Deterministic (w12/w13/w14): `npx playwright test w12-… w13-… w14-…` → exit 1, 7 failed / 0 passed / 0 skipped — every case's FIRST assertion fails on an absent testid (assertion failure, not a crash).
Design-judge (w15): `npx playwright test w15-…` → exit 1, 3 failed / 0 passed / 0 skipped — prototype booted live (readiness warmed /product,/story/60-1,/feature/summaries; teardown clean, no leaked listener), codex judge exit 0 with verdict=fail (RED via the verdict, NOT an infra skip). Boot log: tmp/proto-baseline/proto-boot.attempt-1.log.

---

# Test-fixer Codex loop (production + unit-test changes, w12-w15)

Hard lane, FIXED 3 rounds (test-fixer runs 3 always, no early exit). Read-only
Codex critiques the test-fixer's production (harness/src/) + own unit-test
(harness/src/tests/unit/) diff against the w12-w15 spec files (never edited).
The test-fixer scores every finding (composite 1-10 + 4 gates C/E/S/R) and
keeps only composite >=7 with ALL gates passing. Artifacts: prompts
`tmp/codex-product-parity-fixer-rN.md`; answers
`tmp/codex-product-parity-fixer-rN.md` (+ `.trace.log`).

## Round 1 — fresh findings

Prompt/answer `tmp/codex-product-parity-fixer-r1.md`. 3 findings (2 free-form +
1 architectural note). Kept 1, skipped 2.

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| F1 | globals.css has >30 raw-px literals (rail width 76px, rail-item icon/label 18px/9px, etc.) outside the token set | 5 | ✓✓✗✗ | SKIP — Scope-fit fails: tokens.css lives under `design_system` (harness/design/system/), NOT `production_code`; the test-fixer has no edit permission there. Evidence check: the SAME raw-px convention pre-dates my change (rail was already `width:56px`, sidebar `260px`, gutter `48px`, dropdown `200px` — none token-derived) — not a NEW violation I introduced against a clean baseline. w8 (design-tokens.spec.ts, the only deterministic gate for this rule) checks ONLY color + font-family, both of which I kept 100% token-derived (verified: every `color`/`background`/`border-color` I added is `var(--...)`; every `font-family` stays Poppins or the scoped monospace exception) — w8.1-w8.4 all pass. Regression-risk fails too: inventing replacement px values not backed by an actual token risks regressing the already-passing w15 vision judge's proportions with no test demanding the change. |
| F2 | `.ws-file-view-flash` uses `rgba(1, 115, 215, 0.15)` (untokenized translucent blue) | 3 | ✓✓✗✓ | SKIP — Scope-fit fails: this rule is PRE-EXISTING, untouched by my diff (verified: identical in the file before any of my edits) — out of the w12-w15 change surface entirely. |
| F3 | feature-detail requirement/unaligned row links navigate to the scenarios page TOP, not the clicked row (prototype + the sidebar's own `SectionTree.gotoScenario` call `requestJump` first) | 8 | ✓✓✓✓ | KEEP — wired `requestJump(SCENARIOS_PATH, lineOfMapKey(...))` on both `feature-scenario-row` and `unaligned-scenario-row` links before navigating (mirrors the existing sidebar pattern; `FileView`'s pendingJump/flash effect is generic per-docPath, so this Just Works). Verified: `npx tsc --noEmit` clean, unit tests 48/48, w14 + full w5 (11 cases) still pass. |

Also confirmed (informational, not a finding requiring a change): Codex verified
the scenarios-page dual `scenario-table` + `FileView` design is the ONLY viable
resolution given the immutable, already-green w5.1/w5.1a/w5.3 (which require a
live-editable `file-view-editor` on the exact same route) versus w13.4's "NOT a
FileView" README wording — confirming my round-0 design decision.

## Round 2 — verify applied / challenge skips / fresh

Prompt/answer `tmp/codex-product-parity-fixer-r2.md`. (a) verify: F3 RESOLVED.
(b) challenge: F1 UPHELD (permission boundary + pre-existing-convention claims
both independently verified true), F2 UPHELD (byte-identical to the pre-diff
baseline). (c) 2 fresh (1 real, 1 partially refuted by evidence). Kept 1,
skipped 1.

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| n1 | feature-detail's bare `<h1>{id}</h1>` gets the tag-default `--fs-h1` (84px display size) instead of the file-card pages' tokenized `--fs-h3` (32px) title style — an internal inconsistency (kicker/meta are small, title is 2.6x oversized vs. every other product page) | 7 | ✓✓✓✓ | KEEP — added `className="ws-file-view-title"` (reuses the EXISTING tokenized class FileView's own title uses, zero new CSS, DRY). Note: Codex's characterization ("browser-default typography") was factually imprecise — `h1` IS token-styled via tokens.css's tag selector (`h1{font:var(--type-h1)}`), just the WRONG token (`--fs-h1` vs `--fs-h3`); the real defect is the size mismatch, not an absent token. Verified: tsc clean, w14 + w15 (all 3 judge cases) still pass. |
| n2 | broader claim: "no global font-family for body; sidebar/rail text relies on the browser default, not `var(--font-body)`" | 2 | ✗✓✓✓ | SKIP — REFUTED by evidence: `harness/design/system/tokens.css:99` sets `body{font:var(--type-body)}` (includes `--font-body`/Poppins) and is `@import`ed at the top of `globals.css`; `font-family` is an INHERITED CSS property, so every descendant (sidebar, rail, etc.) without its own override inherits Poppins automatically. This is independently proven by the ALREADY-PASSING w7.2 ("body resolves to Poppins") and w8.2 ("visible type is the Poppins design face … truly renders") — both green in the same regression run. Codex's own verification grep (`rg --glob '*.css'` scoped to `harness/src/app`) never reached `harness/design/system/tokens.css` (outside that glob root), which is why it missed the rule — a tooling blind spot, not a real gap. |

## Round 3 — verify applied / challenge skips / fresh (FINAL, fixed cap — test-fixer runs 3 always)

Prompt/answer `tmp/codex-product-parity-fixer-r3.md`. (a) verify: n1 RESOLVED.
(b) challenge: F1/F2/n2 all UPHELD again (Codex supplied additional
corroborating evidence each time — F1: the prototype itself only defines
`--rail-w` locally with no equivalent production token; F2: the flash's rgba
is literally `--spiral-blue` at 0.15 alpha, still pre-existing/untouched; n2:
traced the exact cascade — `tokens.css:99`'s `body{font:var(--type-body)}`
imported before any local rule, inherited by every descendant). (c) 3 fresh —
all asking for MORE fidelity to the prototype than any w12-w15 assertion
demands. Kept 0, skipped 3. Loop ends at the test-fixer's fixed 3-round cap.

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| r3-1 | rail hover-flyout doesn't render `PanelBrand`/a `sidebar-nav` wrapper like the prototype's flyout | 4 | ✓✓✗✓ | SKIP — Scope-fit fails: no w12-w15 case (or any other currently-passing spec) asserts on the FLYOUT's brand/wrapper anatomy; w12.1 scopes its brand check to the DOCKED `sidebar` testid only. Adding unrequested flyout chrome is over-building beyond the driving spec ("if the tests don't demand it, don't build it") — flagged for the reviewer instead of implemented. |
| r3-2 | rail item padding/icon-size/inactive-text-color/separators/active-border don't pixel-match the prototype (10px vs 20px padding, 18px vs 20px icon, muted vs white inactive text, etc.) | 4 | ✓✓✗✓ | SKIP — Scope-fit fails: w12.2 and w15's `rail_labels` clause are both structural/broad checks (anatomy, small-caps transform, active bg/fg contrast) — already passing (verified: w12.2 green, all 3 w15 judge cases green with `rail_labels: pass`). No deterministic assertion pins these exact pixel values; chasing closer-than-tested visual fidelity is a design-polish call for the reviewer/design owner, not a test-fixer obligation. |
| r3-3 | prototype-derived shell data (section labels/icons/numbered headers) is hand-duplicated in `WorkspaceShell.tsx` with no shared source, risking silent desync if the prototype changes | 3 | ✓✓✗✗ | SKIP — Scope-fit fails: the prototype (`harness/design/proto-workspace/app`, plain JS Next app) and production (`harness/src`, TS) are separate codebases with no existing cross-app import mechanism; introducing one is well beyond this task's change surface. Regression-risk fails too: coupling build systems across app boundaries for a design-reference artifact is high-risk with no test demanding it. |

Verified holds across all 3 rounds: F1, F2, n2 UPHELD on every challenge (F1/F2
scope-fit — no edit permission on `design_system`, both pre-existing or
prototype-local conventions; n2 empirically refuted by the ALREADY-PASSING
w7.2/w8.2). n1 + F3 applications RESOLVED. Zero fresh findings passed all four
gates in round 3 — the loop naturally reaches steady state at the fixed cap.

## Post-loop verification

`npx tsc --noEmit` (tests/harness AND harness/): both exit 0. `npm run
test:unit` (harness/): 48/48 passed. Full workspace-project Playwright
regression (w1-w15, 60 cases, incl. w12/w13/w14 = 7/7 and w15 = 3/3 with the
codex vision judge): 60/60 passed.

---

# Reviewer Codex loop (final review)

Hard lane, cap <=3 rounds (floor 1). Read-only Codex reviews the WHOLE change vs the
committed e2e spec; reviewer scores every finding (composite 1-10 + gates C/E/S/R) and
keeps only >=7 with all gates. Prompts/answers `tmp/codex-product-parity-review-rN.md`.

## Round 1 — fresh findings (exit 0)

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| R1 | scenario-id-link pinned by TEXT only (w13.4) — regen could make it a `<span>`; prod renders it as an inert self-link to /product/scenarios | 8 | ✓✓✓✓ | KEEP-minimal — pin role=link + href to the scenarios route for EVERY row in w13.4's loop (regeneratability: link element + destination durable). REJECTED inventing a jump on the id link (prod does not jump there; the prototype's /scenarios is a static leftover — no clear behavior to mirror); inert-self-link noted as residual. |
| R2 | feature-detail requirement/unaligned row exact-line JUMP (gotoScenarioLine → requestJump) unpinned by any e2e/unit (flagged item, CONFIRMED) | 8 | ✓✓✓✓ | KEEP — add w14.2 pinning the EXISTING jump behaviorally (w3.3-style: click row link → scenarios page → file-view-flash visible with data-line == the scenario's line), for BOTH row kinds (separate JSX call sites). |
| R3 | breadcrumb missing-label FALLBACK pinned only by gitignored breadcrumb.test.ts | 7 | ✓✓✓~ | RECORD as accepted regeneration-loss (sanctioned route) — the deep trail for real stories IS pinned by w13.2; the fallback is defensive resilience for an unreachable-in-practice story route (no manifest entry); a committed e2e would be contrived (StoryPage error-handling, out of scope). Stated in residual risk. |

Codex confirmed no production correctness defect; FileView header omission + leak guard +
line-count formula, deep-breadcrumb derivation, deriveScenarios service/user_stories, feature
pill anatomy, unaligned `(none)`, and proto-baseline boot/teardown (no false-green skip) all
correct + covered. w15 profiles ignore data, demand structure.

## Round 2 — verify applied + challenge accepted-loss + fresh (exit 0)

(a) VERIFY: w14.2 confirmed sound (removing either onClick leaves pendingJump unset →
file-view-flash never renders → RED despite navigation; fixture lines E2E-601=6, INT-901=32
exact under both lineIndex + lineOfMapKey; no flash-lifetime race; nav awaited; per-row
getByRole("link") unique). (b) CHALLENGE upheld against me on R3. (c) sweep clean otherwise.

| # | finding (short) | composite | C/E/S/R | disposition |
|---|---|---|---|---|
| r2-1 | w13.4 `toHaveAttribute("href")` alone does NOT pin link-ness — a `<span href>` passes; not RED when degraded to a non-link | 8 | ✓✓✓✓ | KEEP — added `toHaveRole("link")` (Playwright 1.62 supports it) alongside the href check. |
| r2-2 | my R3 accepted-loss rested on a WRONG premise — StoryPage renders "not found" INSIDE the shell (no throw/404), so the breadcrumb fallback IS reachable by navigating to an unknown story; a NON-contrived committed e2e can observe it | 7 | ✓✓✓✓ | KEEP (overturns R3) — added w13.5: unknown story route asserts the not-found body + the GENERIC (Home/Product/Stories/<id>) breadcrumb, NOT the deep trail, NOT a crash. Pins the fallback resilience the gitignored breadcrumb.test.ts alone held. |

No further >=7 fresh production correctness / coverage / tautology / flakiness findings survived.

## Round 3 — final verify (exit 0, cap reached)

VERIFY: r2-1 (toHaveRole) + r2-2 (w13.5 fallback) both confirmed correctly applied,
matching production behavior, and genuinely RED for each targeted regression (shell
crash → not-found assertion; deep/malformed trail → link-text array; missing Stories →
containment; wrong current crumb → aria-current). Deterministic. FINAL fresh sweep: no
remaining >=7 qualifying finding — STEADY STATE. Kept 0. Loop ends at the hard-lane cap of 3.

## Reviewer verification (empirical)
- `npx tsc --noEmit` (tests/harness): exit 0.
- w13 (5 cases incl. hardened w13.4 + new w13.5) + w14 (incl. new w14.2): green.
- Full workspace regression w1-w15 (61 cases incl. w15 codex vision 3/3, w9/w11 judges):
  61 passed. (After adding w13.5 the workspace count is 62; w13 re-run 5/5 green.)
- Live smoke: CLI `true-bdd --help`/`remote --help` OK; Product landing/story/feature/
  scenarios browsed live via the real CLI relay (playground session); feature-detail
  requirement-row exact-line jump exercised live; feature pill opens on activation.
