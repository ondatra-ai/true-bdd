# Codex rounds — design-scale-conformance (test-fixer)

Fixed 3 rounds, every run, no early exit (test-fixer lane rule — independent
of task lane; step 5's dry-stop does not apply). Task-blind: grounded in the
reproduce block + `w16-workspace-scale.spec.ts` (read from disk, verbatim) +
the full current production/unit-test file content per round (harness/src/ is
gitignored — no git diff exists, so each prompt pastes complete current file
contents in place of a diff). Codex is read-only; the agent scores every
finding (composite 1–10 + four gates: Correctness / Evidence / Scope-fit /
Regression-risk). Keep only composite ≥7 AND all gates pass.

Baseline (before any fix): 6 failed, 0 passed — matches the supplied
reproduce block verbatim (--wk-fs-row unresolved, oversized breadcrumb,
duplicated PRD entry, light chevron chat toggle, hairline file-card border +
~140px empty tail + clipped save-state, single-level breadcrumb); no drift.

Fix applied before round 1: import `design/system/workspace.css` (the
promoted `--wk-*` density scale) into `harness/src/app/globals.css` and
consume it across sidebar rows/group labels/brand-meta/section-header,
breadcrumb bar, file-view kicker/meta/path/line-count; merge the PRD
sidebar-group + its sole child row into one element (fixes the w16.3 text
duplication while still satisfying `w5-product-features.spec.ts`'s
pre-existing "4 sidebar groups incl. PRD" contract); add the muted
scenario-row service annotation; restyle the chat-dock-toggle as a dark
inverted vertical tab; give the file card a strong (dark) 2px border on all
four sides and switch `.ws-file-view-body` from a fixed viewport-relative
height to `height:auto` + the SAME value as a `max-height` cap (hugs short
content, still clamps + internally scrolls long content — this + `rows=`
on the editor textarea, restoring `overflow:auto` after a false-start
`hidden`, is what kept `w2.2`/`w3.3` green); flatten the breadcrumb DOM
(`Fragment` per crumb instead of a wrapping `<span>`) so every separator has
an eligible sibling on both sides; add a deep 3-level breadcrumb trail for
the bare `/product` (PRD) route. New unit test:
`harness/src/tests/unit/breadcrumb.test.ts` (the bare-PRD-route case).
Confirmed green pre-round-1: literal reproduce command → `6 passed`; full
`--project=workspace` suite (68 tests) → **68 passed** (0 regressions,
including two intermediate regressions caught and fixed mid-flight: w15's
vision judge on unconverted sidebar-group-name font size, and w2.2/w3.3 on
the file-card height cap); `tsc --noEmit` → exit 0; `npm run test:unit` →
49/49.

## Round 1 of 3

- Prompt: `tmp/codex-w16-r1-prompt.md`
- Answer: `tmp/codex-w16-r1.md` (trace: `tmp/codex-w16-r1.trace.log`)
- Mode: read-only. Jobs asked: fresh findings only (round 1).

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| 1 | Bare-`/product` breadcrumb added a redundant "Product" crumb pointing at the SAME href as the current "prd.yaml" crumb — a self-referential parent link; the spec's own comment states the baseline as the 3-level "Sessions / Workspace overview / prd.yaml" trail, not 4. | 8 | ✓ (verified against the exact w16.6 comment text) | ✓ | ✓ — directly the trail w16.6 asserts | ✓ — no other spec references a "Product" crumb on this route (grepped) | **KEEP** — dropped the extra crumb; `breadcrumbCrumbs` now returns exactly Sessions/Workspace overview/prd.yaml for the bare route; unit test updated. |
| 2 | `[data-testid="chat-dock-toggle"]` is reused by BOTH the collapsed edge tab AND the open panel's header "Close chat" button, but the vertical-tab CSS (writing-mode, padding, inverted colors) applied unconditionally — the open-state control would render as a squeezed vertical strip inside the horizontal `chat-dock-header` row. No current test asserts the open-state shape, so this was uncaught. | 7 | ✓ (verified: one selector, two structurally different render branches) | ✓ | ✓ — fixing a defect in code this task's own diff introduced | ✓ — scoped via `[aria-label="Open chat"]`, additive/narrowing only | **KEEP** — scoped the vertical-tab rule to `[data-testid="chat-dock-toggle"][aria-label="Open chat"]`; the open-state "Close chat" button now falls back to the plain `button` reset (matches the adjacent `+ New` control). |
| 3 | `.ws-chip` / `.ws-feature-pill` / `.ws-feature-pill-change` still read the editorial `--type-caption`/`--fs-caption` (14px) instead of the promoted `--wk-*` scale, even though `workspace.css`'s own doc comment explicitly names "chips" as part of workspace CHROME. Not asserted by any w16 case (not caught by w15's fuzzy judge either). | 7 | ✓ (verified: `workspace.css` header comment; grepped both classes) | ✓ | ✓ — same class of fix already applied everywhere else in this diff; no test asserts a conflicting exact px | ✓ — no test asserts these at 14px (grepped) | **KEEP** — all three switched to `--wk-fs-caption`. |
| 4 | `--wk-rail-w` (76px) exists in the promoted scale but the rail (`[data-testid="rail"]`) and the flyout's `left` offset are still hardcoded `76px` literals — the exact "coincidental number, not proven token consumption" weakness w16.1 exists to catch, just on an element w16 doesn't happen to assert. | 7 | ✓ | ✓ | ✓ — same principle as w16.1's own token-resolution checks; zero visual change (76px === 76px) | ✓ — value-identical swap | **KEEP** — both switched to `var(--wk-rail-w)`. |

Kept: #1, #2, #3, #4 (all four). Skipped: none.
Round 1 not dry (4 findings, all applied). Continuing to round 2 (fixed
3-round rule — no early exit).

Post-round-1 verification: `tsc --noEmit` exit 0; `npm run test:unit`
49/49; full `--project=workspace` suite re-run → **68 passed, 0 failed**
(5.4m) — no regressions from the four round-1 fixes (incl. w13.2's deep
story breadcrumb and w5-product-features' PRD-group contract, both adjacent
to files this round touched).

## Round 2 of 3

- Prompt: `tmp/codex-w16-r2-prompt.md`
- Answer: `tmp/codex-w16-r2.md` (trace: `tmp/codex-w16-r2.trace.log`)
- Mode: read-only. Jobs asked: (a) verify round-1's 4 keeps, (b) challenge
  round-1 skips (none existed), (c) fresh findings.

| Job | Result |
|---|---|
| (a) verify #1 (bare-`/product` 3-crumb trail) | **RESOLVED** — cited `breadcrumb.ts:110`, `WorkspaceShell.tsx:196/201/205`, `globals.css:94/120`, `breadcrumb.test.ts:63`. |
| (a) verify #2 (chat-toggle scoped to `[aria-label="Open chat"]`) | **RESOLVED** — cited `globals.css:776/779`, `WorkspaceShell.tsx:260/286`. |
| (a) verify #3 (`.ws-chip`/`.ws-feature-pill`/`.ws-feature-pill-change` → `--wk-fs-caption`) | **RESOLVED** — cited `globals.css:526/716/736`. |
| (a) verify #4 (rail width/flyout offset → `var(--wk-rail-w)`) | **RESOLVED** — cited `globals.css:141/208`. |
| (b) challenge skips | None to challenge (round 1 had zero skips). |
| (c) fresh findings | **None** — "no new defect or regression grounded in w16, its reproduce block, or the current changed-file surface" (Codex's own sandbox could not run Vitest directly due to an EPERM on its SSR temp dir — an environment restriction it flagged itself, not a claim about the code; the agent's own `npm run test:unit` run, outside that sandbox, already confirmed 49/49 green both before and after this round). |

Kept: none new (no fresh findings). Skipped: none. Round 2 is DRY per the
loop's own definition (all applications verified, no skip survived, no
fresh finding passed the gates) — but the test-fixer runs a FIXED 3 rounds
with no early exit, so round 3 runs anyway.
No code changes this round (nothing to apply). Verification unchanged:
`tsc --noEmit` exit 0; `npm run test:unit` 49/49; reproduce command 6/6;
full workspace suite 68/68 (all already current from round 1's post-fix run;
re-confirmed identical since no files changed this round).

## Round 3 of 3 (final, fixed — runs regardless of round 2 being dry)

- Prompt: `tmp/codex-w16-r3-prompt.md`
- Answer: `tmp/codex-w16-r3.md` (trace: `tmp/codex-w16-r3.trace.log`)
- Mode: read-only. Jobs asked: (a) re-verify the same 4 findings independently,
  (b) challenge (none to challenge), (c) one more genuinely independent
  fresh-findings pass, explicitly asked to try hard before concluding empty
  (walk w16.5's border-candidate algorithm and w16.3's PRD-dedup algorithm by
  hand again, check for CSS cascade/specificity fragility).

| Job | Result |
|---|---|
| (a) re-verify #1 (bare-`/product` 3-crumb trail) | **RESOLVED** — cited `breadcrumb.ts:110/114`. |
| (a) re-verify #2 (chat-toggle scoped) | **RESOLVED** — cited `globals.css:776`, `WorkspaceShell.tsx:262/286`. |
| (a) re-verify #3 (chip/pill density scale) | **RESOLVED** — cited `globals.css:526/716/736`. |
| (a) re-verify #4 (rail token consumption) | **RESOLVED** — cited `globals.css:141/208`. |
| (b) challenge skips | None to challenge (zero skips across rounds 1-2). |
| (c) fresh findings | **None** — independently traced w16.5's exact border-candidate set (`.ws-file-card` wraps both `file-view-header` and `file-view-gutter`'s parent, so its single `border: 2px solid var(--border-strong)` shorthand covers all 4 sides regardless of the header's own hairline bottom border) and w16.3's PRD dedup algorithm (the merged link is the only element whose own text is exactly "PRD" with a parent whose own text differs) BY HAND, independent of rounds 1-2, and confirmed no CSS cascade/specificity fragility across the `.ws-row`/`.ws-sidebar-group-name`/`[data-selected]` rule set (later attribute-selector rules only ADD selected-state properties, never reverse the border/font-size ones). |

Kept: none new. Skipped: none. Round 3 is DRY — three consecutive
independent passes (round 1's own fresh pass plus two full re-verifications)
converge on the same 4 findings, all applied, nothing further. Loop closed
per the test-fixer's fixed-3-round rule (all 3 rounds run; no early exit was
taken even though rounds 2 and 3 were individually dry).

## Final verification (after round 3, no further code changes)

- Literal reproduce command (`npx playwright test w16-workspace-scale
  --project=workspace --reporter=line`) → **6 passed, 0 failed** (unchanged
  since round 1's fix).
- Full `--project=workspace` suite (68 tests) → **68 passed, 0 failed**
  (confirmed post round 1; unchanged through rounds 2-3, no files touched).
- `harness && npm run typecheck` → exit 0.
- `harness && npm run test:unit` → 49/49 passed.

---

# Reviewer Codex rounds (final review) — hard lane, cap ≤3, floor 1

Read-only Codex loop run by the reviewer over the WHOLE change (production + the
committed e2e spec). Reviewer scores every finding (composite 1–10 + four gates:
Correctness / Evidence / Scope-fit / Regression-risk); keep only composite ≥7 AND
all gates. Prompts: `tmp/codex-review-r{1,2,3}-prompt.md` (r1: `tmp/codex-review-r1.md`);
answers: `tmp/codex-review-r{1,2,3}.md` (+ `.trace.log`).

## Round 1 of 3 — 9 findings

| # | Finding (short) | Composite | Corr | Evid | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| 1 | w16.6 accepts any ≥2-link/≥2-sep trail — the rejected self-referential 4-crumb `…/Product/prd.yaml` still passes; exact 3-crumb property lived only in the gitignored unit test | 8 | ✓ | ✓ | ✓ | ✓ | **KEEP** — added exact-crumb-texts + no-self-ref-link assertions to w16.6 |
| 2 | w16.5 save-state check is `if(count>0)` — vacuously passes if save-state vanishes | 7 | ✓ | ✓ | ✓ | ✓ | **KEEP** — `toHaveCount(1)`+visible+box, unconditional |
| 3 | Token checks prove :root resolution + a matching value, but not per-component CONSUMPTION (a coincidental hardcoded px passes) | 7 | ✓ | ✓ | ✓ | ✓ | **KEEP** — added live-mutation proof (`--wk-fs-row`, `--wk-sidebar-w`) in w16.1 (pattern from w7) |
| 4 | w16.2 breadcrumb height lower-bounded only — an oversized 70/100px bar stays green | 7 | ✓ | ✓ | ✓ | ✓ | **KEEP** — added `≤ --wk-breadcrumb-h + 12` upper bound |
| 5 | File-card border oracle pools 4 sides across ancestors — 4 disconnected edges could pass | 6 | ✓ | ✓ | ✗ (permissiveness is DELIBERATE/documented — prototype splits the outline; w15 file_card backstops the visual outline) | ~ | **SKIP** — scope-fit; documented in residual risk |
| 6 | Open-state `Close chat` (reused testid) horizontal shape has no durable e2e — re-broadening the CSS selector stays green | 8 | ✓ | ✓ | ✓ | ✓ | **KEEP** — w16.4 now opens chat, asserts Close-chat is `writing-mode: horizontal-*` inside chat-dock-header |
| 7 | `.ws-chip`/`.ws-feature-pill` `--wk-fs-caption` not pinned by any e2e | 6 | ✓ | ✓ | ✗ (chip/pill live on arch/story pages OUTSIDE the w16/w15 surface; cosmetic 14→12px; token consumption already proven in w16.1) | ~ | **SKIP** — accepted regeneration-loss; documented in residual risk |
| 8 | Rail width/flyout offset consume `--wk-rail-w` but no durable gate pins it | 7 | ✓ | ✓ | ✓ | ✓ | **KEEP** — added `--wk-rail-w` to resolved set + rail width==token in w16.1 |
| 9 | Generic breadcrumb accumulation + malformed-percent decode pinned only by unit suite | 5 | ✓ | ✓ | ✗ (PRE-EXISTING behaviors, not this task's diff; prefixless branch unreachable) | — | **SKIP** — out of scope; noted |

Kept: #1,#2,#3,#4,#6,#8 (6). Skipped: #5,#7,#9 (3). Applied to
`tests/harness/w16-workspace-scale.spec.ts`; re-ran w16 → 6/6 green. Not dry → round 2.

## Round 2 of 3 — verify applications / challenge skips / fresh (6 items)

| # | Item | Decision |
|---|---|---|
| 1 | w16.6 exact-trail selector `a,[aria-current]` misses a non-linkable `<span>` crumb — an extra "Product" span would pass | **KEEP** (composite 8) — reselect nav direct-children minus `breadcrumb-sep`; a plain-span crumb is now counted |
| 4 | w16.5 "fully INSIDE viewport" only checks the bottom edge | **KEEP** (7) — added x≥0, y≥0, right≤vw |
| 5 | w16.1 rail width is a static value check, not consumption-proven like sidebar | **KEEP** (7) — added `--wk-rail-w` live-mutation |
| 2 | Re-raise #5 (geometric-continuity border oracle) | **SKIP** — contrived scenario, flake-prone geometry, w15 backstop unchanged |
| 3 | Re-raise #7 (chip/pill caption) | **SKIP** — accepted regeneration-loss (scope + proportionality; widening non-deterministic w15 judge risks destabilizing it) |
| 6 | #9 holds (pre-existing) | agreed — no action |

Kept: 3 refinements to my own round-1 edits. Applied; re-ran w16 → 6/6 green + tsc clean. Not dry → round 3.

## Round 3 of 3 (final) — verify all + re-challenge skips + fresh

Codex answer: **"No findings."** All applied hardenings verified correct/complete
and RED-on-regression; no new grounds against the three skips; no fresh findings.
Round 3 DRY. Loop closed at the hard-lane cap (3/3). w16 → 6/6 green throughout.

---

# round 2 (residual fidelity) — test-fixer, w16.7 + w17

New reproduce block: `npx playwright test w16-workspace-scale
w17-visual-pixel-parity --project=workspace --reporter=line`. Baseline: 2 failed
/ 6 passed — w16.1–w16.6 green from the prior round; w16.7 (new residual-fidelity
case) and w17.1 (new deterministic pixel-parity gate against committed golden
crops) red. Task-blind, grounded in the reproduce block + both spec files (read
from disk, verbatim) + the complete current `harness/src/app/globals.css` /
`WorkspaceShell.tsx` / `SectionTree.tsx` content per round (gitignored, no git
diff exists).

## Fixes applied before the critique loop (w16.7 → green, w17.1 partial)

Iterative, test-run-verified (not guessed) fixes to `globals.css` +
`WorkspaceShell.tsx` + `SectionTree.tsx`, each confirmed via a literal Docker
reproduce-command re-run: app-shell inherits `--wk-fs-body` by default (dense
chrome, not editorial-unless-overridden); rail icon/label consume
`--wk-rail-icon-fs`/`--wk-rail-label-fs`, bold label, `--space-3` vertical
padding, hairline inter-item borders, strong rail border, `--text-inverse`
(not muted) rest color; sidebar/PRD/section-header full-bleed `--surface-subtle`
bands via a `.ws-row--full-bleed` compound-selector escape from the section's
own ambient gutter; group labels get a trailing colon + underline (shared
`SidebarGroup`, so Architecture's Services:/Terms:/Docker: inherit it too —
**flagged for the reviewer**: only Product's Features:/Stories:/Scenarios: are
w16.7-gated; Architecture's is a side effect of the shared component, pinned
only by this session's manual verification, not a driving spec); `+ New story`
muted/regular weight; file-card border switched from `--border-width-strong`
to `--border-width`; header-bar separator switched from hairline to strong;
gutter auto-width with `--space-2` all-around padding (was a hardcoded 48px);
editor `--space-2`/`--space-3` padding; chat-toggle `line-height: var(--lh-base)`
pinned (Poppins Bold's UA "normal" line-height is ~1.5, inflating the
vertical-tab's cross-axis width by 4px past the golden's tolerance).

Then, hunting w17 pixel-diff via row-by-row/column raw-pixel analysis of the
actual/golden/diff PNGs (not guessed), found and fixed 8 literal-px-instead-
of-design-token bugs by diffing every relevant production rule against its
prototype source (`harness/design/proto-workspace/app/components/{PanelBrand,
DockedPanel,sections}.js` + `public/vendor/mockups.css` + `public/proto-
extra.css`): sidebar brand-meta `margin-top: 2px` → `var(--space-1)`;
section-header `padding-bottom: var(--space-1)` → `var(--space-2)`; flat-PRD
group `margin-bottom: 2px` → `var(--space-2)`; group-body missing
`padding-bottom: var(--space-2)`; group-header/PRD/leaf-row horizontal insets
corrected to the prototype's `--space-4`/`--space-5` nested-tree indents;
page-header `margin-bottom: var(--space-3)` → `var(--space-4)`; kicker
`margin-bottom: var(--space-1)` → `var(--space-2)`; breadcrumb padding
`var(--space-2) var(--space-3)` → `0 var(--space-4)` (prototype has no vertical
padding, relies on `align-items: center` + `min-height`). Result: rail,
breadcrumb, chat-toggle now PASS w17.1; sidebar 8.88%→3.94% (still > 1% cap);
file-card 4.64%→4.54% (still > 1% cap).

Regression check after all of the above: full `--project=workspace` suite (70
tests) → **69 passed, 0 regressions** (only w17.1 red); `tsc --noEmit` exit 0;
`npm run test:unit` 49/49.

## Blocker consultation (distinct from the 3-round loop below)

The remaining sidebar (3.94→then 5.xx before the fix below) and file-card
(4.5x%) w17.1 residuals resisted further principled CSS-token fixes — one
focused, read-only Codex call was made (not the scored critique loop) per the
blocker protocol.

- Prompt: `tmp/codex-w17-blocker-prompt.md`
- Answer: `tmp/codex-w17-blocker.md` (trace: `tmp/codex-w17-blocker.trace.log`)

Evidence supplied: row-by-row raw-pixel diff-count scripts over the actual/
golden/diff PNGs, proving (a) file-card's header TEXT renders byte-identical
for all 36 rows once the top-border row is set aside — the residual is a
1-2-row screenshot-clip/rounding artifact (production's crop starts exactly at
the border pixel; golden's has a leading white row with the border at row 1) —
not a token/color mismatch; (b) sidebar's story row "60.2 — Summary For Shared
Docs" wraps to ONE line in production but TWO in the golden crop, cascading a
growing Y-drift through every row below it.

**Verdict — file-card: STOP.** "No internal border, spacing, font, or markup
change can insert a row outside an element's screenshot bounds while
preserving the remaining element pixels. This is cross-app fractional
clipping/rounding noise from independently laid-out locator screenshots, not a
reasonably closable production CSS gap." Left red; test untouched.

**Verdict — sidebar: CODE-FIX.** Widen the reserved right-padding gutter on
`[data-testid="feature-row"], [data-testid="story-row"],
[data-testid="scenario-row"]` (the prototype's nested `.sidebar-epic
.sidebar-tree a` indentation reserves more horizontal space than production's
ambient `.ws-row` padding accounted for) to
`calc(var(--space-2) + var(--space-4) + var(--border-width))`. **Applied.**
Verified: literal reproduce command re-run in Docker → sidebar 4.37%→3.94%
(the wrap now matches the golden — confirmed by re-reading the actual/golden
PNGs directly, "60.2" now wraps identically on both sides, and every
downstream row lands within 1-7px of the golden, down from tens of px); full
`--project=workspace` suite re-run → 69/70 unchanged (only w17.1 red, 0
regressions).

## Round 1 of 3 — fresh findings

- Prompt: `tmp/codex-w16w17-r1-prompt.md` (a first attempt with the
  instructions ONLY at the top of a ~2700-line stdin paste got a clarifying
  question back instead of a review — `model_reasoning_effort=low` apparently
  didn't retain the top instructions across that much pasted material; a
  second attempt REPEATING the concrete asks in a footer immediately before
  the `-o` write got a real review. Re-run, not counted as a wasted round.)
- Answer: `tmp/codex-w16w17-r1.md` (trace: `tmp/codex-w16w17-r1.trace.log`)

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| 1 | `w17-visual-pixel-parity.spec.ts:186-189` — the mask loop's `masked = Math.max(masked, applyMask(...))` takes the MAX of each mask's own area instead of the union/sum across `meta.masks`, inflating the `compared` denominator when a component (e.g. file-card) has 2+ non-overlapping masks — makes the ratio artificially LENIENT, not strict. | 7 | ✓ (confirmed by reading the exact lines) | ✓ | ✗ — inside `tests/harness/w17-visual-pixel-parity.spec.ts`, hard-blocked by `.claude/hooks/block_test_edits.py`; test-fixer cannot edit it | n/a | **SKIP — scope-fit fails.** Escalate to the test-author via the orchestrator (this write-up). Also: fixing it would only raise the reported ratios further, not resolve either current failure. |
| 2 | `WorkspaceShell.tsx:276` — `chat-dock-new` ("+ New") button's `onClick` is an intentional no-op per its own comment; no spec asserts its behavior. | 4 | ✓ | ✓ | ✗ — pre-existing code from an earlier session, not part of this round's w16.7/w17 diff, not demanded by any spec | ✓ | **SKIP — scope-fit fails.** Out of round scope; not a regression or gap in THIS diff. |

Kept: none. Skipped: #1, #2 (both scope-fit). Round 1 surfaced findings but
applied zero (both legitimately out of the test-fixer's editable surface or
scope). Continuing to round 2 (fixed 3-round rule — no early exit).

## Round 2 of 3 — verify / challenge skips / fresh

- Prompt: `tmp/codex-w16w17-r2-prompt.md`
- Answer: `tmp/codex-w16w17-r2.md` (trace: `tmp/codex-w16w17-r2.trace.log`)

| Job | Result |
|---|---|
| Challenge skip #1 (mask union bug) | Re-confirmed valid — still inside `tests/`, still would only raise ratios. |
| Challenge skip #2 (`chat-dock-new` no-op) | Re-confirmed valid — repo-wide grep found only visibility/token-sweep assertions (w7, w8), nothing behavioral. |
| Fresh finding #3 | `.ws-sidebar-caret` uses `display:none` at rest / `display:inline-block` on `:hover` (removed from flex flow entirely at rest), vs. the prototype's `.tree-caret` which is ALWAYS laid out (`display:inline-block; visibility:hidden`, toggling only `visibility`) — proposed reserving the slot (width 16px→14px, drop `.ws-sidebar-group-header`'s `gap`) to stop group labels sitting further left than the golden. |

Finding #3 scoring: Composite 7 (plausible, well-evidenced reasoning, citing
the exact prototype rule). **Applied, then empirically MEASURED, then
REVERTED — regression-risk gate fails in practice.** The agent applied the
exact proposed CSS, rebuilt the harness image, and re-ran the literal
reproduce command in Docker: sidebar's w17 ratio got WORSE, not better
(3.94%→4.39%). Reverted; re-run confirmed back to 3.94%. Recorded as a
disproven finding, not merely "skipped," so round 3 doesn't re-litigate the
identical proposal without new evidence.

Kept: none (0 of 1 fresh findings survived empirical verification; both prior
skips re-confirmed, not re-opened). Round 2 not dry (one finding was tried).
Continuing to round 3 (fixed 3-round rule).

## Round 3 of 3 (final) — verify all + re-challenge skips + fresh

- Prompt: `tmp/codex-w16w17-r3-prompt.md`
- Answer: `tmp/codex-w16w17-r3.md` (trace: `tmp/codex-w16w17-r3.trace.log`)

| Job | Result |
|---|---|
| Re-verify skip #1 (mask union bug) | Re-confirmed independently — same conclusion, still a test-author escalation. |
| Re-verify skip #2 (`chat-dock-new`) | Re-confirmed — still no driving spec. |
| Verify round-2's revert | Confirmed: `.ws-sidebar-caret` in the pasted current file is `display:none`/`display:inline-block`, `.ws-sidebar-group-header` still has `gap: var(--space-1)` — the disproven combination is NOT present; revert was clean. |
| Fresh finding #4 | The story/feature/scenario-row `padding-right` (currently `calc(var(--space-2) + var(--space-4) + var(--border-width))` = 41px) "overshoots the prototype by 9px" — proposed `calc(var(--space-2) + var(--space-3))` = 30px instead, reasoning from `.sidebar-tree a { padding-right: var(--space-3) }`. |

Finding #4 scoring: Composite 5 (the cited arithmetic contains an internal
error — Codex's own write-up computes "16px + 32px + 1px = 49px" for a
formula that is actually `10+30+1=41px` given this repo's real token values,
i.e. it doesn't add up even on its own terms) — **Correctness gate: borderline
fail.** Applied anyway to let the numbers settle the question empirically:
rebuilt, re-ran the literal reproduce command in Docker → sidebar 3.94%→3.95%
(statistically flat, within run-to-run noise, not an improvement). Reverted to
the blocker-consultation's already-validated 41px value.
**Decision: SKIP — Regression-risk gate fails** (no measured improvement to
justify the change; the existing value is the empirically better of the two).

Kept: none (0 of 1 fresh findings survived empirical verification). Both prior
skips + round 2's revert independently re-verified, none re-opened. Round 3
is the final fixed round — loop closed at 3/3 (test-fixer's fixed-3-round
rule; two of the three rounds were individually "not dry" in that a candidate
fix was tried, but neither candidate survived empirical measurement).

## Final verification (after round 3)

- Literal reproduce command (`npx playwright test w16-workspace-scale
  w17-visual-pixel-parity --project=workspace --reporter=line`) →
  **7/7 w16 passed**; **w17.1 still red** (sidebar 3.94%, file-card 4.54%,
  both against a 1% cap — rail/breadcrumb/chat-toggle sub-checks pass).
- Full `--project=workspace` suite (70 tests) → **69 passed, 1 failed**
  (w17.1 only) — 0 regressions across the whole session.
- `harness && npm run typecheck` → exit 0.
- `harness && npm run test:unit` → 49/49 passed.
- `harness && npm run lint` → pre-existing failures confined to
  `harness/design/proto-workspace/` (the vendored prototype deliverable, not
  touched this session, not part of `harness_code_root`); zero lint errors in
  the three files this session edited (`eslint` run scoped to just those
  three files → clean).

## Residual risk / open items for the reviewer

1. `tests/harness/w17-visual-pixel-parity.spec.ts:186-189`'s mask-union bug
   (finding #1 above) — a genuine test-authoring defect, escalate to the
   test-author. Not applied here (off-limits).
2. w17.1 sidebar (3.94%) and file-card (4.54%) remain red against the 1% cap.
   Both are backed by a Codex STOP/evidenced-residual verdict (file-card) or
   an evidenced-but-incompletely-closed CODE-FIX (sidebar, improved
   5.80%→3.94%, ~32% reduction, but not fully closed) after 8+ token-bug
   fixes, one blocker consultation, and 3 further critique rounds (2 more
   candidate fixes tried and empirically disproven). The believed remaining
   cause (per the blocker consultation + this loop) is cross-app sub-pixel
   rendering/screenshot-clip noise between the prototype's `next dev` server
   and production's `next build` output, not a remaining design-token
   mismatch — but this belief is not independently proven beyond what's
   documented above.
3. Group labels (Features:/Stories:/Scenarios: trailing-colon + underline)
   apply via the shared `SidebarGroup` component, so Architecture's
   Services:/Terms:/Docker: labels get the same treatment as a side effect —
   only the Product three are w16.7-gated. Route to durable e2e coverage if
   desired.

---

# round 3 (box-model parity) — test-fixer, w16.7 new numeric targets + w17

New baseline handed to this round: w16.1–w16.6 green; **w16.7 FAILING** on
newly added box-model numeric assertions since the prior round (tree-row
`padding: --space-1 --space-3 --space-1 --space-4`, file-view-gutter
`--space-2` on all 4 edges, PRD row `font-size: --wk-fs-label`); **w17.1
FAILING on 2/5 components** (sidebar 3.96%, file-card 1.68%, cap 1.0%).
Confirmed via the literal reproduce command before any edit — matches the
supplied baseline verbatim, no drift. Task-blind, grounded in the reproduce
block + both spec files (read from disk, verbatim) + the complete current
`harness/src/app/globals.css` / `WorkspaceShell.tsx` content (gitignored, no
git diff exists).

## Fixes applied before the critique loop

Iterative, Docker-rebuild-and-rerun-verified after EACH change (never
guessed): tree-row box model — `[data-testid="feature-row"|"story-row"|
"scenario-row"]` `padding: --space-1 --space-3 --space-1 --space-4`
(replacing an earlier session's wider ad-hoc `padding-right` that had been
compensating for a wrapping mismatch); PRD row `font-size:
var(--wk-fs-label)`. w16.7 → green (7/7); w17.1 sidebar got WORSE at first
(3.96%→4.91%) because the narrower padding-right reopened the story-title
wrap mismatch the prior round's blocker fix had closed.

Root-caused and fixed via direct pixel-analysis of the golden/actual PNGs
(python/PIL, leftmost-dark-pixel-per-row scans) cross-referenced against the
live prototype's OWN computed geometry (booted locally via `next dev`,
inspected with a throwaway Playwright script) rather than guessed:
- Section header (`02—PRODUCT`) restructured from a single `<div>` with a
  full-bleed block `border-bottom` to a flex row (`WorkspaceShell.tsx`) with
  a reserved 14px caret slot + a `flex:1 1 auto` text span carrying its OWN
  scoped `border-bottom` — matches the prototype's real DOM (`.tree-caret` +
  `.tree-name-link` inside a flex `summary`), not a full-width bar at the
  wrong y. `text-decoration-line: underline` kept on the outer element only,
  to satisfy w12.1's own-computed-style "underlined" check without
  double-painting (propagation blocked on the inner span).
- `.ws-sidebar-group-header` (Features:/Stories:/Scenarios:) switched from
  an ambient-relative inset (30px total) to its own full-bleed
  `padding: --space-1 --space-3 --space-1 --space-4` (44px caret-inclusive
  inset, matching the golden's measured inset exactly).
- `.ws-sidebar-caret` switched `display:none`/`inline-block` →
  `visibility:hidden`/`visible` (ALWAYS laid out, matching the prototype's
  `.tree-caret`), plus `font-size:inherit; line-height:var(--lh-base)` —
  `<button>` doesn't inherit `line-height` from the UA stylesheet, so left
  unset it was the tallest flex child in the `align-items:center` header
  row, inflating every group header's height. Single biggest fix:
  4.12%→3.27%.
- `[data-testid="feature-row"|"story-row"|"scenario-row"]`: `margin-left:0`
  (cancel `.ws-row`'s inherited `-10px` pull, tuned for single-level rows)
  + `margin-right: calc(var(--space-3)*-1)` (escape the ambient
  `.ws-sidebar-section`'s own right padding, recovering the ~20px of
  text-wrap width the narrower w16.7-mandated `padding-right` had cost,
  without violating the exact `--space-3` value w16.7 asserts).
- `.ws-row[data-testid="prd-row"]`: `line-height: 1`. w16.7 asserts PRD's
  `font-size ≈ --wk-fs-label` (15px) via a comment attributing this to the
  prototype's `.sidebar-flat-link` class — but grepping the LIVE prototype
  shows that class is never applied to the PRD row (it styles an unrelated
  "workspace-overview" flat link elsewhere); PRD's real prototype rendering
  is 13px. w16.7 is off-limits and DOES assert 15px, so production
  necessarily renders PRD taller than golden — `line-height:1` (w16.7 never
  asserts PRD's line-height) minimized the resulting cascade. Single biggest
  win of the session: 3.27%→1.76%.
- `.ws-new-story-open`: `line-height: var(--lh-base)`. 1.76%→1.62%.

Verified throughout: `w16.7` stayed green; full `--project=workspace` suite
(70 tests) → **69/70, only w17.1 red, 0 regressions**; `tsc --noEmit` exit 0;
`npm run test:unit` 49/49.

## Blocker consultation (distinct from the 3-round critique below)

- Prompt: `tmp/codex-w17-blocker2-prompt.md`
- Answer: `tmp/codex-w17-blocker2.md` (trace: `tmp/codex-w17-blocker2.trace.log`)

Asked Codex to independently verify (via its own commands/image analysis,
read-only) whether the residual sidebar (1.62%) and file-card (1.68%, this
session never touched file-card CSS) gaps were further closeable in
production CSS.

**Verdict — sidebar: CODE-FIX.** Found `.ws-new-story-open` only set
`padding-block`, leaving its LEFT/RIGHT inset at Chromium's UA button
default — its text sat ~14px left of every other leaf row's own inset.
Proposed `padding: var(--space-1) var(--space-3)` (not `--space-4`, since
this button — unlike the leaf-row testids — isn't nested inside
`.ws-sidebar-group-body`'s own extra indent). **Applied.** Verified: sidebar
1.62%→1.58%. Codex separately characterized the REMAINING residual as
"genuinely alternating 1px [row-position] offsets... look like fractional
text/layout rounding and should not be chased with per-row transforms" —
confirmed independently via my own row-by-row pixel scan: X-alignment is
within 0-1px at EVERY row now; Y-alignment alternates ±1px non-uniformly
(e.g. `inventory-view` 1px earlier than golden while `Scenarios:` is 1px
later in the SAME crop) — no single global registration offset (the test's
own ±2px search) or production CSS change can zero out an alternating,
per-region-varying sub-pixel drift between two independently-built Next.js
apps (prototype `next dev` vs production Dockerized `next build`
standalone).

**Verdict — file-card: STOP** (re-confirmed, unchanged from the prior
round's own blocker consultation). Independent inspection: best-registered
crops have identical top-border placement, structurally aligned header/
background rows; the only differing boundary row is a gutter-background-vs-
border-color pixel at the header/editor seam; the golden's crop includes a
partially-clipped extra line at its bottom edge that production's
independently-rounded locator screenshot does not. "No internal file-card
CSS rule can add pixels outside the locator's rounded screenshot bounds
while preserving its correctly aligned border, header, gutter width, and
line rhythm." Left untouched; test unchanged.

## Round 1 of 3 — fresh findings

- Prompt: `tmp/codex-w16w17-boxmodel-r1-prompt.md`
- Answer: `tmp/codex-w16w17-boxmodel-r1.md` (trace:
  `tmp/codex-w16w17-boxmodel-r1.trace.log` — Codex ran real verification
  commands: grepped the prototype's `NewStoryForm`/`sidebar-tree`/
  `sidebar-epic` structure, cross-checked `workspace.css` token values)

**"No fresh findings."** Kept: none. Skipped: none. Round 1 DRY but the
fixed-3-round rule runs regardless — continuing to round 2.

## Round 2 of 3 — verify round 1 / challenge skips (none) / fresh

- Prompt: `tmp/codex-w16w17-boxmodel-r2-prompt.md`
- Answer: `tmp/codex-w16w17-boxmodel-r2.md`

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| 1 | `.ws-sidebar-group-name[data-selected="true"] { display:block; padding:...; margin-inline:...; }` was DEAD for its stated target (selected PRD row — a higher-specificity compound rule, 0,0,3,0 > 0,0,2,0, always won regardless of source order) but ACTIVELY HARMFUL for a selected Features:/Stories:/Scenarios: group-header link (reachable via e.g. `/product/features`), corrupting `.ws-sidebar-group-header`'s own flex box model | 8 | ✓ (independently re-verified the specificity math myself before applying) | ✓ (exact line cites) | ✓ (interacts directly with this round's own group-header restructuring) | ✓ (deletion only; targeted re-run w1+w5+w12 = 19/19, full suite unaffected) | **KEEP** — deleted the dead/harmful rule |

Kept: #1. Skipped: none. Verified: w16.7 still 7/7; w17.1 unchanged (sidebar
1.58%, file-card 1.68% — expected, the bare `/product` fixture never
exercises a selected group header); targeted w1+w5+w12 regression (19
tests) → 19/19. Round 2 not dry — continuing to round 3 (fixed rule).

## Round 3 of 3 (final) — re-verify round 2 / re-challenge round 1 / fresh

- Prompt: `tmp/codex-w16w17-boxmodel-r3-prompt.md`
- Answer: `tmp/codex-w16w17-boxmodel-r3.md`

| # | Finding (short) | Composite | Correct | Evidence | Scope | Regr | Decision |
|---|---|---|---|---|---|---|---|
| 1 | A selected Features:/Stories:/Scenarios: group header still doesn't get the prototype's `summary:has(> .tree-name-link.is-active) { background: var(--surface-subtle) }` full-row paint — only the inline `.ws-sidebar-group-name`'s own background | 6 | ✓ | ✓ (cites `proto-extra.css:24`) | ✗ — PRE-EXISTING gap (not introduced or regressed by this round's diff; the `data-selected` background rule itself was never touched), not asserted by any current w16/w17 case (bare `/product` fixture only ever selects PRD), zero ratio impact | n/a | **SKIP — scope-fit fails.** Flagged for the reviewer/durable e2e coverage below, not fixed here. |
| 2 | Two MORE now-dead `[data-selected="true"]` rules survived round 2's cleanup: a `color`/`font-weight`-only rule fully shadowed by an identical-specificity, later rule with the same values; and a compound rule whose every declaration duplicated `.ws-row.ws-row--full-bleed` (the only element that can match both selectors is PRD, which already carries plain `.ws-row`) | 7 | ✓ (independently re-verified: same-values shadow + PRD-only-match duplication, both confirmed by reading the selectors) | ✓ (exact line cites) | ✓ (cleanup of the exact rule cluster round 2 just touched; corrects that round's own comment) | ✓ (dead code only, zero computed-output change — confirmed byte-identical w17.1 ratios after) | **KEEP** — deleted both; corrected the round-2 explanatory comment to describe the full cluster |

Kept: #2. Skipped: #1 (scope-fit — pre-existing, out of round). Round 3 is
the final fixed round — loop closed at 3/3. Re-verified: w16.7 7/7; w17.1
ratios byte-identical (sidebar 1.58%, file-card 1.68% — dead-code removal
has no computed-output effect); targeted w1+w5+w12 → 19/19; `tsc --noEmit`
exit 0; `npm run test:unit` 49/49; `eslint` on the two touched files → 0
errors, 0 warnings on the `.tsx` (globals.css isn't ESLint-scoped).

## Final verification (after round 3)

- Literal reproduce command (`npx playwright test w16-workspace-scale
  w17-visual-pixel-parity --project=workspace --reporter=line`) →
  **w16.1–w16.7: 7/7 passed**; **w17.1: still red** (sidebar 1.58%,
  file-card 1.68%, both against the 1.0% cap — rail/breadcrumb/chat-toggle
  sub-checks pass).
- Full `--project=workspace` suite (70 tests) → **69 passed, 1 failed**
  (w17.1 only) — 0 regressions across the whole session (confirmed twice,
  once mid-session and once as the final post-round-3 check).
- `harness && npm run typecheck` → exit 0.
- `harness && npm run test:unit` → 49/49 passed.
- `harness && npx eslint` on the two files this session touched
  (`globals.css`, `WorkspaceShell.tsx`) → 0 errors/warnings on the `.tsx`
  (CSS isn't ESLint-scoped in this repo).

## Residual risk / open items for the reviewer

1. `tests/harness/w17-visual-pixel-parity.spec.ts`'s mask-union bug (flagged
   two rounds ago) has since been FIXED by the test-author — confirmed this
   session by reading the current file (`applyMasks` now unions via a flags
   array, not `Math.max`); no longer an open item.
2. w17.1 **sidebar (1.58%)** and **file-card (1.68%)** remain red against
   the 1.0% cap, both backed by this session's blocker-protocol Codex
   consultation:
   - file-card: STOP (re-confirmed identical to the prior round's own STOP
     — a screenshot-clip/crop-boundary rounding artifact between two
     independently-rounded locator screenshots, not a token/CSS gap).
   - sidebar: one further CODE-FIX found and applied (`.ws-new-story-open`
     padding), then the remainder characterized as irreducible alternating
     ≤1px cross-app sub-pixel layout rounding — reduced from a 3.96% session
     baseline (peaking at 4.91% mid-session before the full fix set landed)
     to 1.58%, a ~60% reduction, via 8 further token-driven CSS fixes this
     round (on top of the ~9 already applied in the prior round), but not
     fully closed.
3. `w16.7`'s own explanatory comment for the PRD row's `font-size` (`//
   ...prototype "`.sidebar-flat-link { font-size: var(--wk-fs-label) }`"`)
   misattributes a CSS class that in the CURRENT live prototype styles an
   UNRELATED "workspace-overview" flat link, not the PRD row (verified by
   grepping `harness/design/proto-workspace/app/components/sections.js` and
   `.../public/vendor/mockups.css`) — the PRD row's real prototype rendering
   is 13px (`--wk-fs-row`), not 15px. w16.7 is off-limits to the test-fixer
   and DOES assert the 15px value, so production now legitimately renders
   PRD 2px taller than the golden capture, which was the single largest
   contributor to the w17.1 sidebar residual before the `line-height:1`
   mitigation. Escalate to the test-author: either the assertion or its
   citing comment needs correcting.
4. A selected Features:/Stories:/Scenarios: sidebar group header doesn't
   get the prototype's `summary:has(> .tree-name-link.is-active)` full-row
   `--surface-subtle` background (round 3, finding #1 above) — pre-existing,
   not this round's regression, not asserted by any current w16/w17 case
   (their fixtures only ever select PRD). Route to durable e2e coverage if
   desired.
