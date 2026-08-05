# Product section prototype parity — test plan

Tests-ONLY plan. Defines the e2e specs (exact assertions) + test-side scaffolding
that pin the Product section and its shell to the runnable prototype
(`harness/design/proto-workspace/app`). NO production design; the test-fixer
derives every UI change from the red specs alone (Spec-as-Source). Requirement
ids R1–R9 are the task brief (`docs/tasks/product-section-prototype-parity.md`).

## Goal

Every Product surface and the shell chrome around it in prod (`harness/src`) is
demanded by a red e2e spec — deterministic structural specs for the DOM contract
plus codex vision-judge pairs that boot the prototype from the repo as the
baseline — so that opening prod's `/product`, `/product/stories/<id>`,
`/product/features/<id>`, `/product/features`, `/product/scenarios` looks and
behaves like the prototype's `/product`, `/story/<id>`, `/feature/<id>`,
`/features`, `/scenarios(/requirements)`.

## Non-goals

- Architecture / Home / Builds / chat parity — untouched here (shell chrome that
  is shared, e.g. the rail + sidebar brand, is in scope only as R1/R2 demand).
- Changing the real data path: prod stays REAL (CLI relay via `w-workspace-happy`);
  no test hardcodes the prototype's in-memory seed content.
- New services, compose files, Dockerfile, or API routes — none are required.
- Committing baseline PNGs — R8 boots the prototype live (brief forbids committed
  PNGs).
- Touching test scripts in `harness/package.json`.

## Current state (prod, verified by reading source)

- **Rail** (`WorkspaceShell.tsx`): `rail-item-<section>` text = `Home`/`Arch`/
  `Prod`/`Bld`, no icon element, no small-caps full label; the bottom utility is a
  `rail-utility-item` button labelled `Set`/aria "Settings" — NOT a Sessions entry.
- **Sidebar** (`SectionTree.tsx` inside `Sidebar`): renders the Product tree
  (PRD / Features / Stories / Scenarios groups, guide line, per-row `data-selected`)
  but has NO brand header (no `TrueBDD` / workspace-context label) and NO underlined
  section header (`02—PRODUCT`). `+ New story` lives on the product PAGE
  (`new-story-open`), not in the sidebar Stories group.
- **FileView** (`FileView.tsx`): `file-view-path` header + gutter + editor only.
  NO kicker/title/meta page-header block, NO `N lines` counter.
- **Story page**: `FileView` + a bare `feature-picker-toggle` button whose text is
  the raw value or `Select feature…` (no `Feature:` pill, no CHANGE affordance).
  Breadcrumb (shell `content-breadcrumb`) = `Home / Product / Stories / <id>`.
- **Feature detail page**: `<h1>{id}</h1>`, `feature-description`, headings
  `Stories` / `Requirements` / `Unaligned`; story/unaligned rows = `<span>id</span>`
  + picker; **Requirements rows are plain text ids with no link and no picker**; no
  kicker; rows show no title and no link.
- **Features / Scenarios pages**: bare `FileView` (no kicker/title/meta, no lines
  counter).

## Target state (prototype, verified by reading its source + `tmp/proto-reference/`)

- **Rail** (`Rail.js`): each entry = an icon glyph + a small-caps FULL label
  (`HOME`/`ARCHITECTURE`/`PRODUCT`/`BUILDS`); active entry is an inverted tile; a
  Sessions entry (`↩`) pinned at the bottom.
- **Sidebar** (`PanelBrand.js` + `sections.js`): brand header `TrueBDD` + a
  workspace-context meta line; an underlined section header `02—PRODUCT`; a PRD file
  row highlighted when current; `Features:` / `Stories:` / `Scenarios:` groups with a
  vertical guide line listing the REAL workspace items; a `+ New story` affordance in
  the Stories group; current page's row visibly selected.
- **FileView** (`FileView.js`): page-header = kicker (`02—Product[…]`) + display
  title (`prd.yaml`) + muted meta subtitle, then a file card whose header bar shows
  the doc path + a `N lines` counter, line-number gutter, monospace body, editable
  in place.
- **Story page** (`story/[id]/page.js`): kicker `02—Product / <id>`; title
  `<id> — <title>`; a `Feature: <name>` pill with a `change`/CHANGE control opening
  the picker (assign writes the story's `feature:` through the relay); breadcrumb
  `Sessions / Workspace overview / Product / <file>.yaml`.
- **Feature detail** (`feature/[id]/page.js`): kicker
  `02—Product / Features / <NAME>`; title = feature id; description subtitle;
  sections **User stories**, **Requirements**, **Unaligned requirements** —
  card-row lists where each row = the item title as a LINK + its `Feature:` pill
  control; unaligned rows show `Feature: (none)`.
- **Features page** (`features/page.js`): FileView card, kicker
  `02—Product / Features`, title `features.yaml`.
- **Scenarios page**: the LIVE prototype surface the sidebar opens is
  `/requirements` — a FileView of `scenarios.yaml` (kicker `02—Product / Scenarios`,
  title `scenarios.yaml`). The `/scenarios` route in the prototype is a leftover
  STATIC table (SCENARIO/DESCRIPTION/SERVICE/LINKED STORY) — see Challenges.

### Key current→target delta (one line)

Prod's Product pages are behaviorally complete but visually bare: they lack the
FileView kicker/title/meta + `N lines` counter, the branded sidebar (brand header,
underlined section header, in-sidebar `+ New story`), the icon+small-caps rail with
a bottom Sessions entry, the story `Feature:` pill + CHANGE + deep breadcrumb, and
the feature-detail linked-title card rows with per-row pickers and the
`User stories / Requirements / Unaligned requirements` section naming.

## End-to-end test cases

New spec files in `tests/harness/` (workspace `w*` project, filename-routed). Each
case is authored RED against current prod; the cited "fails now because" is the
R9 anchor. Deterministic specs read the prototype's ACTUAL DOM contract (above);
the judge specs boot the prototype live. Additive testids are named per case and
land in `helpers/ui.ts` (`WTID`) + `helpers/README-testids.md` (test-author owned).

### w12 — shell parity (`w12-shell-product-parity.spec.ts`) — R1, R2

- **w12.1 branded sidebar (R1).** Nav to `wsRoutes.product(sid)`.
  - `sidebarSection(product)` contains a `sidebar-brand` whose text contains
    `TrueBDD` AND a distinct workspace-context meta label element (non-empty).
  - `sidebarSection(product)` contains a `sidebar-section-header` whose text
    matches `/02\s*[—-]\s*PRODUCT/i` and whose computed
    `border-bottom-style` / `text-decoration` shows an underline (a non-`none`
    bottom border or underline — the prototype underlines the section header).
  - The `PRD` row (`prd-row`) has `data-selected="true"` on `/product`.
  - `sidebarGroup(product,"Stories")` contains the `new-story-open` affordance
    (scoped inside the sidebar), and clicking it still opens `new-story-form`
    (keeps w5 green — the testid is reachable via `page.getByTestId`).
  - Fails now because: no `sidebar-brand` / `sidebar-section-header` in prod, and
    `new-story-open` is on the product page, not in the sidebar.

- **w12.2 rail parity (R2).** Nav to `wsRoutes.product(sid)`.
  - For each of home/architecture/product/builds: `railItem(page,section)` contains
    an icon element (`rail-item-icon`, non-empty glyph) AND a label element
    (`rail-item-label`) whose text (accessible name) equals the FULL word
    `Home`/`Architecture`/`Product`/`Builds` (NOT the truncated `Arch`/`Prod`/`Bld`),
    with the label's computed `text-transform` = `uppercase` and its `letter-spacing`
    resolving from `--ls-label` — the small-caps presentation the prototype uses via
    CSS, not literal uppercase DOM text (F4: a title-case span with no transform must
    NOT pass). Icon sits above/before the label (assert vertical/reading order via
    boundingBox).
  - The active entry (`product`, `aria-current="page"`) is an INVERTED tile on the
    dark rail: its computed `background-color` resolves from `--surface-page` and its
    `color` from `--text-primary` (via `toRgb`), and BOTH contrast with (a) an
    inactive entry (e.g. `architecture`, which is transparent/inverse-fg on the dark
    rail) AND (b) the rail's own `--surface-inverse` background (F1 — the tile is the
    inverse of the dark rail, not `--surface-inverse` itself).
  - The bottom `rail-utilities` contains a Sessions entry: a `rail-utility-item`
    whose accessible name matches `/sessions/i` and which navigates to the sessions
    route (`/` or `/sessions`), positioned in the lower rail region (reuse w7.1a's
    boundingBox check).
  - Fails now because: prod rail items have no icon/label sub-elements and use
    truncated labels with no uppercase transform; the bottom utility is `Settings`,
    not Sessions.

### w13 — product file pages parity (`w13-product-file-pages-parity.spec.ts`) — R3, R4, R6, R7

- **w13.1 product landing file card (R3).** Nav `wsRoutes.product(sid)`.
  - `file-view-kicker` text matches `/02\s*[—-]\s*PRODUCT/i`, structurally ABOVE the
    title.
  - `file-view-title` text = `prd.yaml`, structurally ABOVE the meta.
  - `file-view-meta` (the muted subtitle) is non-empty AND its computed `color`
    resolves from `--text-muted` (via `toRgb`), and it sits BELOW the title
    (boundingBox order) — F6: a plain body paragraph must not pass.
  - File-card anatomy (F5): an additive `file-view-header` bar element CONTAINS both
    `file-view-path` (= `docs/prd/prd.yaml`) and `file-view-line-count` (text matches
    `/^\d+\s+lines$/`, number equals EXACTLY `readEditor(editor).split("\n").length` —
    the prototype's formula, which counts the trailing empty segment after a final
    newline; NOT a newline-character count, which is off-by-one — round-2 fresh #4).
    Below that bar, `file-view-gutter` and `file-view-editor` are siblings inside a
    card body, with the gutter's line count equal to the editor's (monospace of
    editor+gutter is already pinned by w7.2 — not re-asserted here).
  - Architecture-untouched guard (round-3 fresh #2 — enforces the non-goal
    observably): navigate `wsRoutes.architecture(sid)` and assert its file view does
    NOT render the product page-header — `page.getByTestId(WTID.fileViewKicker)` and
    `WTID.fileViewTitle` have count 0 on the architecture route. Current prod has no
    kicker anywhere, so this is green now; it fails the moment a fixer adds the
    product kicker/title to the SHARED FileView unconditionally (leaking it onto
    Architecture), which w2/w3 alone would not catch. Lives inside this otherwise-red
    case, so R9 holds for w13.1 overall.
  - Edit round-trip preserved (regression guard, F17): append a UNIQUE YAML comment
    (`\n# <runToken>\n`, mirroring `seedTokenInDoc`) via `writeEditor`; assert the
    editor shows it, `save-state` reaches `data-save-state="saved"`, and
    `waitForDocOnDisk` bytes both `include` the token and still `parse` as YAML. The
    browser→relay→CLI receipt oracle for PRD editing is already proven by w5.4a — this
    case only guards that the new header block doesn't break the edit path.
  - Fails now because: prod FileView has no kicker/title/meta, no `file-view-header`
    bar, and no lines counter.

- **w13.2 story page kicker/title/pill/breadcrumb (R4).** Nav
  `wsRoutes.story(sid,"60.1")`.
  - `file-view-kicker` matches `/02\s*[—-]\s*PRODUCT\s*\/\s*60\.1/i`.
  - `file-view-title` text = `60.1 — Summary Length Preference` (id + story title,
    read from the served buffer, not hardcoded expectation only — assert it equals
    `<id> — <parsed title>`).
  - The `Feature:` pill has DISTINCT anatomy (F9 — a single unstyled text button must
    not pass): the collapsed `feature-picker-toggle` contains three additive child
    elements — `feature-pill-label` (text `Feature:`), `feature-pill-value` (text =
    the current value `summaries`), and `feature-pill-change` (text matches
    `/change/i`) — asserted as three separately-located elements. Only ACTIVATING the
    control reveals the searchable picker: `feature-picker-input` is absent/hidden
    before the click and visible after.
  - The pill (bearing CHANGE) is OPERABLE, not decorative (round-2 fresh #2): after
    seeding a unique feature via `seedFeatureOnDisk`, picking it through this pill
    (`pickFeatureIn(page, seededId)`) writes the story's `feature:` to disk
    (`waitForDocOnDisk` + node-scoped parse of `60.1-…yaml`). This ties the redesigned
    pill to the real relay path; w5.6 covers generic-toggle persistence for 60.2.
  - `content-breadcrumb` renders the deep trail: an ordered crumb sequence whose
    labels are `Sessions`, `Workspace overview`, `Product`, and a final
    `aria-current="page"` crumb equal to the story FILE name
    (`60.1-summary-length-preference.yaml`). Assert each crumb text in order.
  - Fails now because: no kicker/title, the picker is a bare `Select feature…`
    button, and the breadcrumb leads `Home / Product / Stories / 60.1`.

- **w13.3 features page file card (R6).** Nav `wsRoutes.features(sid)`.
  - Same file-card anatomy contract as w13.1 (F5): `file-view-kicker` matches
    `/02\s*[—-]\s*PRODUCT\s*\/\s*FEATURES/i`; `file-view-title` = `features.yaml`;
    `file-view-meta` muted + below title; `file-view-header` bar contains
    `file-view-path` (= `docs/prd/features.yaml`) + `file-view-line-count`
    (= buffer line count); gutter/editor siblings in the card body.
  - Fails now because: bare FileView (same missing header/counter as R3).

- **w13.4 scenarios page file card (R7 — see Challenges for the conflict).** Nav
  `wsRoutes.scenarios(sid)`.
  - PRIMARY (asserted): same file-card anatomy contract as w13.1 — `file-view-kicker`
    matches `/02\s*[—-]\s*PRODUCT\s*\/\s*SCENARIOS/i`; `file-view-title` =
    `scenarios.yaml`; `file-view-header` bar with `file-view-path` (=
    `docs/scenarios.yaml`) + `file-view-line-count`; gutter/editor siblings. This is
    the surface the LIVE prototype sidebar opens (`/requirements`) and the one the
    distilled brief P18 mandates (scenarios registry AS a file view).
  - This assertion is CONDITIONAL on the R7 resolution (Challenges): the brief's prose
    + the reference capture point at a distinct static TABLE listing. If the
    orchestrator picks the table reading, w13.4 is replaced by a table LISTING spec
    (one row per real scenario, scenario-id link, description, service/linked-story
    cells) whose target surface is materially larger than a file view — a scope the
    orchestrator must confirm BEFORE the fixer runs, since a task-blind fixer follows
    whichever red spec ships. (The plan does not prescribe that surface's production
    shape — only the observable listing the spec would assert.)
  - Fails now because: bare FileView (missing header/counter) under the primary
    reading.

### w14 — feature detail parity (`w14-feature-detail-parity.spec.ts`) — R5

- **w14.1 feature aggregation anatomy (R5).** Seed a unique feature id via
  `seedFeatureOnDisk` for the reassign check; nav `wsRoutes.feature(sid,"summaries")`.
  - `feature-page-kicker` matches `/02\s*[—-]\s*PRODUCT\s*\/\s*FEATURES\s*\/\s*SUMMARIES/i`.
  - The page title heading text = `summaries` (the feature id); `feature-description`
    text EQUALS the `summaries` record's `description` parsed (node-scoped) from the
    SERVED `features.yaml` buffer — not merely non-empty (round-2 fresh #3: arbitrary
    static text must not pass) — and is the muted subtitle positioned below the title.
  - Three section headings present with EXACT text `User stories`, `Requirements`,
    `Unaligned requirements` (asserts the rename from `Stories`/`Unaligned`).
  - In `feature-stories-list`: the `feature-story-row[data-story-id="60.1"]` contains
    a LINK (role=link) whose text includes `60.1` and the story title, whose href
    resolves to `wsRoutes.story(sid,"60.1")`, AND a `feature-picker` scoped to the row.
  - In `feature-scenarios-list`: the `feature-scenario-row[data-scenario-id="E2E-601"]`
    contains, as sibling left/right elements, a LINK whose text includes `E2E-601` and
    its description and whose href resolves to the scenarios page, AND a `feature-picker`
    scoped to the row (currently prod Requirements rows are plain text with NEITHER).
  - In `unaligned-bucket`: `unaligned-scenario-row[data-scenario-id="INT-901"]`
    contains, as sibling left/right elements (F8), a LINK whose text includes
    `INT-901` and its description with an href to the scenarios page, AND a
    `feature-picker-toggle` pill whose `feature-pill-value` shows `(none)`.
  - Reassignment still works via BOTH row kinds (F7 — exercises the NEW Requirements-
    row picker prod currently lacks): `pickFeatureIn(storyRow60_1, seededId)` and
    `pickFeatureIn(reqRowE2E601, seededId)` each write the underlying node's
    `feature:` on disk (`waitForDocOnDisk` + node-scoped parse) and re-bucket the row
    out of `summaries` live. (General live re-bucketing is already proven by
    w5.4/w5.7; this case proves the new anatomy AND that the added Requirements-row
    picker persists.)
  - Fails now because: headings are `Stories`/`Requirements`/`Unaligned`; story rows
    have no title link; Requirements rows have no link and no picker; unaligned rows
    show no linked title and no `(none)` pill; no kicker.

### w15 — product design-judge pairs (`w15-product-design-judge.spec.ts`) — R8

Reuses `runDesignJudge` (helpers/design-conformance.ts) with NEW named-check
profiles, and a NEW `bootPrototype()` helper (§Startup scaffolding) that serves the
prototype live on a free port; baseline screenshots come from that server, never a
committed PNG.

Skip discipline (F10/F16 — critical): the ONLY authorized skip is `codex` absent —
`test.skip(!codexOnPath())` with a named reason, matching w9/w11. EVERY OTHER
failure (npm install, Next boot, readiness, screenshot, navigation, judge exit) is a
TEST FAILURE with captured logs + cleanup diagnostics, NOT a skip and NOT a hang. A
run with codex present but a broken prototype boot MUST go red (three failed cases),
never three greenish skips — otherwise a broken baseline hides the judge and R8/R9
are defeated. A module-level lazy singleton boots the prototype ONCE (shared across
the three cases in the single-worker suite) and is torn down robustly (§Startup
scaffolding); a rejected boot fails all three cases rather than silently skipping.

Each case: set 1440×900 viewport; navigate the EXACT baseline route on the
prototype server AND wait for that route's own DOM anchors before capture (F13 —
Next dev compiles routes lazily, so `/product` readiness does NOT imply `/story/60-1`
or `/feature/summaries` compiled): FileView for the landing/story, the feature strip
for the story, the three section lists for the feature page — plus
`document.fonts.ready`. Screenshot the prototype route (image 1, baseline) and the
prod route (image 2). Run the judge; `expect(exitCode).toBe(0)`;
`auditVerdict(verdict, CHECKS)` → `problems` empty; `failedChecks` empty. Unlike
w11, the rubric does NOT tolerate the rail/sidebar difference — both baseline and
prod use the rail+sidebar, so nav chrome is compared directly. Rubric judges
STRUCTURE only; ignores all data/content values.

- **w15.1 product landing pair.** baseline `${proto}/product` vs
  `wsRoutes.product(sid)`. Checks: `sidebar_structure` (brand header + underlined
  section header + PRD row + Features/Stories/Scenarios groups), `rail_labels`
  (icon + small-caps full labels + bottom Sessions), `kicker_title_block` (kicker
  over a display title over a muted subtitle), `file_card` (header bar with path +
  lines counter, gutter, monospace body).
- **w15.2 story page pair.** baseline `${proto}/story/60-1` vs
  `wsRoutes.story(sid,"60.1")`. Checks: `sidebar_structure`, `rail_labels`,
  `kicker_title_block`, `file_card`, `feature_pill` (a `Feature: <name>` pill with a
  change control above the file card).
- **w15.3 feature detail pair.** baseline `${proto}/feature/summaries` vs
  `wsRoutes.feature(sid,"summaries")`. Checks: `sidebar_structure`, `rail_labels`,
  `kicker_title_block`, `section_lists` (three named sections of card rows, each row
  = a linked title on the left and a Feature pill control on the right).
- Fails now because: current prod is missing the brand header, icon rail, kicker/
  title block, lines counter, the `Feature:` pill, and the linked-title section
  rows — the judge names each missing surface and returns `verdict:"fail"`.

## Startup scaffolding

No NEW startup scaffolding (services, compose files, Dockerfile, service directories,
API routes) is required for this task — the parity specs assert against surfaces the
existing harness already boots, so there is nothing to leave EMPTY for the fixer. The
plan therefore prescribes NO production edits and names no production location (no
files-to-touch, no implementation choices — Spec-as-Source); the task-blind fixer
derives every PRODUCTION change from the red specs alone. The runnable prototype is
the UNCHANGED design-truth baseline (the oracle): production is made to match it; the
prototype is never edited, and no static mockup is authored.

The only NEW files are TEST-side (test-author owned, fully implemented test code —
not empty production stubs), listed here for completeness:

- `tests/harness/w12-…`, `w13-…`, `w14-…`, `w15-…spec.ts` — the specs above.
- `tests/harness/helpers/ui.ts` — additive `WTID` entries + locator helpers for the
  new testids: `railItemIcon`/`railItemLabel`, `sidebarBrand`,
  `sidebarSectionHeader`, `fileViewHeader`, `fileViewKicker`/`fileViewTitle`/
  `fileViewMeta`/`fileViewLineCount`, `featurePillLabel`/`featurePillValue`/
  `featurePillChange`, `featurePageKicker`. Existing entries are untouched.
- `tests/harness/helpers/README-testids.md` — documents the additive testids +
  their `data-*` vocabularies (binding contract; additive only).
- `tests/harness/helpers/design-conformance.ts` — additive `PRODUCT_LANDING_PROFILE`,
  `STORY_PAGE_PROFILE`, `FEATURE_DETAIL_PROFILE` (check-name sets + rubrics) reusing
  `buildJudgeSchema`/`auditVerdict`/`runDesignJudge`; existing profiles untouched.
- `tests/harness/helpers/proto-baseline.ts` — the `bootPrototype()` boot mechanism,
  specified concretely (Codex r1 F11/F12/F14/F15):
  - **Install (F14):** run ONCE (module singleton, never per Playwright worker) in
    `harness/design/proto-workspace/app`, only when `node_modules` is absent/invalid;
    prefer `npm ci` (the dir ships `package-lock.json`) over `npm install` for
    determinism; give install its own bounded timeout + a captured log; an install
    error FAILS (never skips).
  - **Launch (F11):** do NOT run `npm run dev`/`start` (both hardcode `-p 3999`).
    Spawn the local Next binary directly (`node_modules/.bin/next dev -p <port>`) with
    ONE explicit allocated port, OR do a cached `next build` then
    `next start -p <port>`. Retain the child handle immediately after spawn. Read the
    actual listening URL from the process's stdout rather than assuming the port.
  - **Port + readiness (F12/F13):** allocate via `allocatePort()`; on `EADDRINUSE`
    retry with a freshly allocated port, bounded (mirror ServerController's
    `BIND_RETRIES`); monitor the child's `exit` during readiness (an early exit fails
    fast with stdout/stderr attached); readiness is per-baseline-route (navigate each
    exact route + its DOM anchor + fonts), not a single-route poll.
  - **Teardown (F15 + round-2 fresh #5):** idempotent; kill the retained child in a
    `finally` on EVERY readiness/setup failure (not only `afterAll`); graceful
    terminate then bounded forced kill; safe if the singleton init rejected before
    returning a handle. On the SUCCESSFUL path, the spec file's `test.afterAll` MUST
    call the singleton's `stop()` (single-worker suite) so a healthy boot is never
    leaked — the failure-path `finally` alone does not cover a clean run.
  - **Skip vs fail:** the helper NEVER skips — it returns a live `{ baseURL, stop() }`
    or throws with diagnostics; only the SPEC skips, and only for missing `codex`.

## Codex rounds

See `docs/tasks/plans/product-section-prototype-parity.codex.md` (kept beside this
plan; the orchestrator/test-author read it).

## Challenges

- **R7 scenarios ambiguity — THREE conflicting sources; orchestrator MUST resolve
  before the fixer runs.** (1) Brief R7 prose + the reference
  `tmp/proto-reference/scenarios.png` = a STATIC TABLE (columns SCENARIO / DESCRIPTION
  / SERVICE / LINKED STORY — service + linked-story links, NOT feature tags/pills;
  the "feature tags" phrasing is a loose paraphrase of the SERVICE column). (2) The
  prototype's LIVE scenarios surface (what the sidebar Scenarios rows actually open)
  is `/requirements` — a FileView of `scenarios.yaml`; the literal `/scenarios` route
  is a leftover `dangerouslySetInnerHTML` static dump not wired into live nav. (3) The
  distilled brief P18 (the "real product" per the prototype README) mandates the
  scenarios registry AS a GitHub-style file view. Prod's `/product/scenarios` is
  already a FileView. The plan's PRIMARY red spec (w13.4) asserts the FileView parity
  — the reading backed by 2 of 3 sources, the live prototype, and prod's current
  surface, and self-consistent with R3/R6. Because a task-blind fixer follows
  whichever red spec ships, if the orchestrator/user instead wants the static-table
  listing, w13.4 must be swapped for a table LISTING spec — a materially larger scope
  than a file view — that the orchestrator must confirm; the plan neither prescribes
  nor names the production shape of that surface, only the observable listing a spec
  would assert. This is a decision that CANNOT be silently made by the planner. No
  feature-pill assertions are invented for the scenarios listing: no real prototype
  scenarios surface has them (feature pills on scenario rows live on the feature-detail
  page,
  R5).
- **R1 sub-clause coverage (traceability; F3 skip rationale, corrected in round 2).**
  w12.1 adds the genuinely-new R1 surfaces (brand header, underlined section header,
  in-sidebar `+ New story`). R1's guide line, real-item population, and current-row
  selection are ALREADY satisfied by CURRENT PROD and partly pinned: w7.1a (guide-line
  visibility, Stories group), w5.1a (fixture-derived group counts + real rows), w1.6
  (selection moves story→scenario). Round-2 correction: w1.4 asserts caret/collapse/
  nav, NOT the guide line — the guide line is w7.1a's. These surfaces are NOT part of
  the current→target delta: prod's `SectionTree` already renders a `sidebar-guide-line`
  in every group and `data-selected` on every row/feature-row, so a new "red"
  assertion for them would be BORN GREEN (violating R9's fail-when-absent rule). The
  plan therefore does not re-assert them; adding feature-row-selection or per-group
  guide-line assertions would test already-shipped behavior, not the parity delta.
- **Shared-surface regression is guarded two ways (F18 + round-2 #1 + round-3 #2).**
  (a) POSITIVE: the Architecture file page's existing contract (`file-view-path`/
  gutter/editor + editing) is already protected by w2 (file view) + w3 (outline jumps)
  on `wsRoutes.architecture` — a regression there goes red without this plan adding a
  redundant guard. (b) NEGATIVE: w2/w3 assert PRESENCE, not the ABSENCE of product
  chrome, so a fixer could add the product kicker/title to the SHARED FileView
  unconditionally and leak it onto Architecture while w2/w3 stay green — so w13.1 adds
  an observable negative guard (product `file-view-kicker`/`file-view-title` count 0 on
  the architecture route), enforcing the "Architecture untouched" non-goal without
  prescribing HOW the header is product-scoped. (Round-2 correction: the earlier "w13
  includes a guard" wording promised a step no w13 case defined; the guard now lives
  concretely in w13.1.)
- **Breadcrumb is a shared derivation.** R4's deep story-page trail
  (`Sessions / Workspace overview / Product / <file>.yaml`) is a real production-
  behavior change with a unit-test implication (`breadcrumb.test.ts` exists —
  regenerated by the fixer, its lane, not the planner's). The e2e guard: w13.2 pins
  the story trail, and existing w10.7 pins the /home two-level trail; no e2e asserts
  the arch/builds trail, so those stay whatever the fixer's regenerated unit contract
  holds. The plan does NOT prescribe the derivation — it only asserts the observable
  story + home trails.
- **`+ New story` relocation.** R1 wants it in the sidebar; w5 drives it via
  `page.getByTestId(newStoryOpen)` (page-wide, not position-scoped — verified). w12.1
  asserts the affordance is INSIDE the sidebar Stories group while w5 keeps finding it
  page-wide, so both are satisfiable by the same reachable `new-story-open`/
  `new-story-form` testids without the plan dictating placement mechanics.
- **Prototype boot cost (R8 infra).** First-run `npm ci` in the prototype is
  minutes-long and network-dependent; `next` build/compile adds more. Mitigations:
  install/boot ONCE via a module-level singleton shared across w15's three cases;
  generous per-route readiness (see §Startup scaffolding); a free port via
  `allocatePort()`. The prototype's in-memory seed matches the fixture shape, so
  structural parity holds even though the judge ignores data values. Skip discipline
  is settled in the w15 preamble + §Startup scaffolding: the ONLY skip is missing
  `codex`; npm/Next/readiness/screenshot failures are TEST FAILURES with captured
  logs, never skips (do not re-introduce an npm/next skip here — that was the
  round-2 F10 correction).
- **Rail active-tile token check.** The active tile is the INVERSE of the dark rail:
  assert its background resolves from `--surface-page` and its foreground from
  `--text-primary` (verified proto-extra.css:180), not a literal, mirroring w7.1's
  token discipline. (The rail itself is `--surface-inverse`; the active tile must NOT
  be asserted against that token — that was the round-1 F1 correction.)

## Workflow log

- Studied prototype source (Rail/PanelBrand/sections/ProductFiles/FileView/
  FeaturePicker + product/story/feature/features/requirements/scenarios routes),
  prod equivalents (WorkspaceShell/SectionTree/FileView/FeaturePicker + product
  pages), the reference captures, and the e2e conventions (w1/w5/w7/w9/w11,
  helpers/design-conformance + workspace-env + ui + README-testids).
- Confirmed green-keeping: no spec asserts non-home breadcrumb text, rail-item text,
  or picker-toggle text; `newStoryOpen`/`newStoryForm` are page-located.
- Drafted the four specs (w12–w15) + additive testids + the prototype-boot helper.
- Codex critique loop: see the ledger.

- 2026-08-04T14:35Z orchestrator DECISION (R7 challenge): the parity target for the scenarios page is the TABLE anatomy shown by the live prototype /scenarios route and tmp/proto-reference/scenarios.png — title "Requirements / Scenarios", flat-registry subtitle, table columns SCENARIO(link)/DESCRIPTION/SERVICE/LINKED STORY(link). The /requirements FileView is a separate surface, out of scope. The plan's w13.4 FileView-parity case is OVERRIDDEN to assert the table; test-author implements accordingly.

- 2026-08-04T16:20Z orchestrator: fixer green 10/10 + full workspace regression 60/60 (verified independently 10/10, fresh image). Fixer-FLAGGED unpinned behavior for reviewer regeneratability audit: feature-detail scenario/unaligned row exact-line-jump wiring (gotoScenarioLine via requestJump/lineOfMapKey) — no w14 assertion or unit test covers the jump. Also note: fixer fixed pre-existing red w1.1b (home-landing testid, demanded by existing w1 spec) — legitimate, verify.
