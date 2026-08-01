# Plan — harness-workspace-ui-design

Tests-first implementation plan for the task brief
`docs/tasks/harness-workspace-ui-design.md`. All paths are repo-relative; path
roles follow `docs/context/paths.md`.

> **This is a DESIGN-ONLY task.** The "production code" the coder writes is the
> design deliverable under `harness/design/` (a mirrored S&F design system,
> static HTML mockups, a design spec) — **not** harness app code. `harness/app/`
> stays absent. The e2e/BDD contract under `tests/harness/` (`helpers/ui.ts`,
> `helpers/README-testids.md`, every `p*`/`a*` spec, `playwright.config.ts`) is
> **not touched** — the only test change is **adding** new mockup spec file(s)
> (and, per the brief, a new mockup-only Playwright config the test-author owns).

---

## Goal

Deliver, as reviewable local artifacts (not harness implementation), a
ClickUp-inspired workspace UI design for the true-bdd harness, skinned with the
S&F design system: (a) a local, offline S&F design-system **mirror** under
`harness/design/`, (b) static self-contained **HTML mockups** of the workspace
UI that are clickable along representative flows, and (c) a short **design
spec**. New Playwright spec(s) assert the mockups meet the brief.

## Non-goals

- **No harness production code** — `harness/app/` stays absent; no Next.js pages,
  components, lib, or API routes. Implementation is a follow-up task.
- **No changes to the existing e2e/BDD contract or its specs** — `helpers/ui.ts`,
  `helpers/README-testids.md`, `playwright.config.ts`, and every `p*`/`a*` spec
  are untouched. The design deliberately departs from the current UI contract
  (story modal → full page; per-row actions → story page); migrating the
  existing suite belongs to the follow-up task.
- **No redesign of the CLI terminal output or the BDD fixture harness.**
- No app logic / API / real routing in the mockups — inter-page `<a href>` links
  only, plain HTML/CSS with minimal inline JS, viewable with no server or build.
- No Docker/Redis/Go dependency for the new specs (they must not touch the
  existing suite's heavy `globalSetup`).

---

## Current state (what exists today)

- **Harness app**: gutted. `harness/` holds only config (`package.json`,
  `next.config.ts`, `tsconfig.json`, `Dockerfile`, vitest/eslint config). There
  is **no** `harness/app/` and **no** `harness/design/`. `harness/.gitignore`
  ignores only build artifacts, so `harness/design/` will be git-tracked.
- **e2e contract** (`tests/harness/`): a self-contained Playwright package with
  two projects — `protocol` (`p[0-9]*.spec.ts`) and `ai` (`a[0-9]*.spec.ts`) —
  plus `ai-gate`. The shared `playwright.config.ts` runs a **heavyweight
  `globalSetup`** that builds the harness Docker image and starts Redis on every
  invocation; each test launches its own container. The UI it targets is the flat
  three-view v2 app (sessions list `/`, session detail `/sessions/<sid>`, run view
  `/sessions/<sid>/runs/<rid>`): a flat inventory + epic accordion + story
  **modal** (`<dialog>`), per-row actions, session-scoped runs, prompt
  **dialogs**. Selector/route contract lives in `helpers/ui.ts` +
  `helpers/README-testids.md`.
- **Fixture data** (realistic content source): `tests/harness/fixtures/` holds
  synthetic host projects. The richest single, self-consistent set is
  `p3-inventory-spread` (epic 60 with stories 60.1/60.2/60.3 engineered to
  `created:missing` / `applied 1/2` / `applied 2/2`, a `scenarios.yaml` with
  E2E-601/602/603 linked to those stories), complemented by `a1-create-no-fix`
  (`architecture.yaml` with `mcp-service` + a `vocabulary` block; `prd.yaml` with
  personas Claude User / Developer) and `a5-build-tests-fix` (scenario `INT-901`).
- **No S&F design system on disk** anywhere; it lives only in the claude.ai/design
  project `147f5da0-fedd-4aaa-b0c2-ccb9f7d7b41e`, reachable **only by the
  orchestrator** via DesignSync (subagents cannot reach it).

## Target state (what the goal requires that does not exist yet)

- `harness/design/system/` — an offline S&F mirror (tokens, components,
  guidelines, fonts/assets) with a `SYNC.md` recording the source project id and
  sync date. **Populated by the orchestrator via DesignSync before the coder
  phase** (see "Design-system mirror contract").
- `harness/design/mockups/` — 17 static HTML pages covering every required view
  and degraded/flagged state, sharing a sidebar/breadcrumb/canvas frame, linking
  the mirrored tokens, cross-linked for the representative flows, at a 1440px
  desktop reference.
- `harness/design/SPEC.md` — the layout frame, sidebar ordering, token→UI mapping,
  page inventory, degraded-state catalog, and data provenance.
- `tests/harness/` — new mockup spec(s) `m*.spec.ts` + a helper + a **separate
  mockup Playwright config** (no `globalSetup`, file:// URLs) that leaves the
  existing `protocol`/`ai` projects untouched.

---

## Folder layout (agreed contract — orchestrator + coder + test-author)

```
harness/design/
  system/                     # S&F MIRROR — orchestrator populates via DesignSync
    tokens.css                #   REQUIRED single entrypoint: :root { --sf-* } + @font-face
    fonts/                    #   Poppins (+ any S&F faces), offline
    components/               #   S&F component references/snippets (as pulled)
    guidelines/               #   S&F usage guidelines (as pulled)
    assets/                   #   gradient/image assets referenced by tokens/components
    SYNC.md                   #   source project id 147f5da0-… + sync date (ISO)
  mockups/                    # CODER writes — static HTML pages
    assets/
      mockups.css             #   layout frame + token→component skin (built on ../../system/tokens.css)
      mockups.js              #   MINIMAL inline-scale JS (sidebar expand, dialog open/close)
    sessions.html
    workspace-overview.html
    prd-overview.html
    epic.html
    story-detail.html
    story-ambiguous.html
    story-invalid.html
    scenarios.html
    scenario-detail.html
    service.html
    vocabulary.html
    runs.html
    run-detail.html            # non-success outcome (degraded)
    prompt-choice.html         # run-detail + open choice <dialog>
    prompt-clarify.html        # run-detail + open numbered-clarify <dialog>
    prompt-freetext.html       # run-detail + open multiline-freetext <dialog>
    unavailable.html           # 504 cli_timeout / disconnected workspace state
  SPEC.md                     # CODER writes — the short design spec
```

**Path-role note (coordination, not a blocker):** paths.md's "Production code"
list is app-focused (`src/`, `templates/`, `true-bdd/`, `harness/app/`) and does
not enumerate `harness/design/`. That directory is **outside `tests/`**, so the
`block_test_edits.py` hook does not block it and the coder may write it. The
orchestrator should confirm the coder's write surface for this task is
`harness/design/` (design output), and that `harness/design/system/` is
pre-populated by the orchestrator, not the coder.

---

## Design-system mirror contract (orchestrator DesignSync step — BEFORE the coder)

The orchestrator (only actor that can reach the claude.ai/design project) pulls
**and normalizes** the S&F system into `harness/design/system/` via its DesignSync
tool **before** dispatching the coder, and **validates the complete contract
below is met before dispatch**. The subagents consume the mirror; they never reach
claude.ai. **Because the coder cannot consult the source project, all decisions
that require the source — resolving split token exports into the single
entrypoint, embedding real token values, obtaining the font binaries — are the
orchestrator's responsibility, not the coder's (round 1, finding 8).** The coder
only *consumes* the mirror and *raises a blocker* (via the orchestrator) if the
contract is unmet; the coder never invents tokens or font data. The mirror MUST
satisfy this contract so the mockups and specs bind to stable paths:

1. **`harness/design/system/tokens.css` MUST exist** as the single CSS entrypoint
   the mockups link, produced/normalized **by the orchestrator**. It defines the
   S&F design tokens as `:root` custom properties (colors, gradients, spacing,
   radii, type scale) and `@font-face` rules pointing at **local** `./fonts/`
   files (no remote `http(s)` `url()` — the mirror must work offline). If
   DesignSync's raw export is split across files, the **orchestrator** re-exports
   the real S&F values into this one entrypoint.
2. **`harness/design/system/fonts/`** contains the Poppins face(s) as real font
   binaries so type renders offline (no network font fetch); every `@font-face`
   `url()` in `tokens.css` resolves to a file that exists here.
3. **`harness/design/system/SYNC.md`** records `source project id:
   147f5da0-fedd-4aaa-b0c2-ccb9f7d7b41e` and the `sync date:` (ISO 8601). This
   fulfils the brief's "mirror recording its source project id and sync date."
4. **`components/` and `guidelines/`** are non-empty (the brief lists components +
   guidelines as required mirror content); `assets/` holds any gradient/image
   files the tokens/components reference, all local.

> The specs bind only to **(a)** the *presence of a stylesheet link into
> `../system/`* and **(b)** the *applied Poppins type* (a brief-stated S&F
> invariant) — never to specific token variable names, which DesignSync owns.
> This keeps the tests stable regardless of the exact token names pulled.

---

## End-to-end test cases (LEAD — `tests/harness/`, mockup specs)

**How Playwright opens the pages (decision):** the mockup specs open the pages as
**`file://` URLs** — `page.goto(pathToFileURL(resolve(__dirname,
"../../harness/design/mockups/<page>.html")).href)`. No `webServer`, no
`baseURL`, no build step. This is deliberate: it *directly verifies the brief's
"viewable without a server or build step"* requirement, and keeps the specs free
of Docker/Redis/Go. Relative `<a href>` links and `<link rel="stylesheet">` into
`../system/` resolve against the file:// origin (confirmed to work in Chromium
under Playwright for linked CSS, `@font-face`, and anchor navigation).

**Test isolation (decision):** the specs run under a **new, separate config**
`tests/harness/playwright.mockups.config.ts` (test-author-owned) that has **no
`globalSetup`/`globalTeardown`**, one `mockups` project (`testMatch:
**/m[0-9]*.spec.ts`), no `webServer`, and a **1440×900 `viewport`** in `use` so
every mockup spec renders at the brief's 1440px desktop reference (round 2,
finding B4). Run:
`cd tests/harness && npx playwright test -c playwright.mockups.config.ts`.
Rationale (evidence-based): the existing `playwright.config.ts` `globalSetup`
builds the harness Docker image and starts Redis on *every* invocation
regardless of `--project`; a `mockups` project added to that config would inherit
that heavy setup and couple static-HTML tests to the app image. A separate config
also leaves the existing `protocol`/`ai` projects byte-for-byte untouched. (The
default config already never runs `m*` — no project's `testMatch` matches it — so
the new files are inert there too.)

**Selectors (decision):** the mockup specs assert primarily via **ARIA
roles/landmarks and visible text** (`getByRole("navigation")`, headings,
`getByRole("link", {name})`, `getByRole("dialog")`) plus a **small set of
mockup-only `data-testid`s** documented *inside the new helper*
(`helpers/mockups.ts`) — e.g. `mockup-sidebar`, `mockup-breadcrumb`,
`sidebar-section-<name>`, `inventory-chip`, `lifecycle-chip`. These are **not**
added to `helpers/README-testids.md` or `helpers/ui.ts` (that contract is
frozen). Every assertion below is phrased to **fail if the behavior/content were
absent** (missing page, missing region, broken token link, dead nav link).

Proposed spec files (all under `tests/harness/`, `m*` → the new `mockups`
project):

### `m1-mockup-pages.spec.ts` — page coverage, sidebar frame, token consumption

- **m1.1 Every page in the authoritative inventory exists and renders its
  identifying content.** Data-driven over the **single canonical page inventory**
  exported from `helpers/mockups.ts` — **all 17 HTML files** (round 2, finding B6),
  which the existence, token (m1.6), sidebar (m1.2), and resource checks all reuse
  so no page can be silently dropped:
  `sessions`, `workspace-overview`, `prd-overview`, `epic`, `story-detail`,
  `story-ambiguous`, `story-invalid`, `scenarios`, `scenario-detail`, `service`,
  `vocabulary`, `runs`, `run-detail`, `prompt-choice`, `prompt-clarify`,
  `prompt-freetext`, `unavailable`. For each, `goto(file://…)` succeeds and the
  page shows its signature content; the degraded pages
  (`story-ambiguous`/`story-invalid`/`unavailable`) get their deep content
  assertions in m3, but their **existence** is checked here so the loop covers the
  whole inventory. Signature content, e.g.:
  - `sessions.html`: a sessions list with ≥1 `session-row`; the row shows a
    canonical folder path, a CLI version string, and a **test-connection**
    control (`getByRole("button", {name: /test connection/i})`).
  - `workspace-overview.html`: heading "Workspace overview"; **inventory health**
    chips; **build actions** (`build tests`, `build code` controls) and an
    **inventory refresh** control.
  - `prd-overview.html`: PRD title "MCP Google Docs Editor…", a summary, and both
    personas ("Claude User", "Developer/Maintainer") from `prd.yaml`.
  - `epic.html`: epic title "Inventory Spread Epic"; a **grouped story table**
    with rows for 60.1/60.2/60.3.
  - `story-detail.html`: story "60.2", the as-a/i-want/so-that statement, ≥1 AC.
  - `scenarios.html`: a **flat** list containing scenario ids "E2E-601",
    "E2E-602", "E2E-603" (and "INT-901"), each row showing its service.
  - `scenario-detail.html`: "E2E-601", service "mcp-service", merged
    given/when/then text, a link to story 60.2.
  - `service.html`: "mcp-service", its language/framework, its test config.
  - `vocabulary.html`: the vocabulary content ("ask Claude", forbidden qualifiers
    like "properly"/"correctly"/"seamless", forbidden action "handle").
  - `runs.html`: a runs list with ≥1 `run-row` carrying a command + outcome.
  - `run-detail.html`: run state + a **non-success** outcome badge + output tail.
  - `prompt-choice.html` / `prompt-clarify.html` / `prompt-freetext.html`: an
    **open** `role="dialog"` overlay (see m4).
  Assertion strength: each signature string is drawn from a named fixture file, so
  a placeholder/empty page fails. If a page file is missing, `goto` errors → fail.
- **m1.2 Every workspace page carries the shared sidebar with the required
  structure and top-level ordering.** For each workspace page (all except
  `sessions.html`), assert one `mockup-sidebar` landmark containing, **in order**:
  a **workspace overview** entry, the **three document sections** (Architecture,
  Product, Requirements/Scenarios), a **Runs** section, and a **return to
  sessions** link. Assert order by reading the sidebar's link/heading text
  sequence and comparing to the expected array (fails if a section is missing or
  misordered).
- **m1.3 The Architecture section follows `architecture.yaml`: a service node +
  a vocabulary node.** In the sidebar's Architecture section assert a node/link
  for "mcp-service" (→ `service.html`) and a "Vocabulary" node (→
  `vocabulary.html`). Fails if either node is absent.
- **m1.4 The Product section is epics→stories with a PRD-overview root.** In the
  Product section assert the root links to `prd-overview.html`, an epic node
  "Inventory Spread Epic" (→ `epic.html`) expands to story nodes 60.1/60.2/60.3
  (→ `story-detail*.html`). Fails if the tree is flat or a level is missing.
- **m1.5 The Requirements/Scenarios section is a FLAT scenario-id list.** Assert
  the section lists scenario ids directly (E2E-601/602/603, INT-901) with **no
  intervening epic/story tree levels** — i.e. the scenario links are direct
  children of the section, and each row/link surfaces the scenario's service
  and/or linked story. Fails if scenarios are nested under epics.
- **m1.6 Every page consumes the mirrored S&F tokens (robust, no false-pass).**
  Loop over **every** page in the inventory (not a "representative sample" —
  round 1, finding 6) and assert:
  - **(a) exact mirror entrypoint linked** — a `<link rel="stylesheet">` whose
    resolved `href` is exactly `…/harness/design/system/tokens.css` (not merely
    "something under `../system/`" — round 1, finding 5). DOM assertion.
  - **(b) the token font binary actually decoded offline** — on a small subset of
    pages (the check is heavier), use the robust oracle (round 2, finding A5 —
    `FontFaceSet.check()` alone is NOT a reliable "loaded" proof): call
    `await document.fonts.load('16px "Poppins"', "Ag")` to force the load, then
    require at least one entry in `document.fonts` whose family is Poppins has
    `FontFace.status === "loaded"`, then `await document.fonts.ready`. This fails
    if the local font file is missing/broken even though
    `getComputedStyle(...).fontFamily` would still report "Poppins" (the
    false-pass). Also assert body `fontFamily` matches `/Poppins/i` on every page
    as the cheap applied-type check.
  - **(c) a token value is actually applied (serialization-safe — round 2,
    finding B1)** — self-referential and stable without hardcoding DesignSync-owned
    names: pick **one canonical longhand** (`color`). Read a documented semantic
    custom property at runtime
    (`getComputedStyle(document.documentElement).getPropertyValue('--<documented
    token>')`), assert non-empty; assign it to a throwaway probe element
    (`probe.style.color = "var(--<token>)"`) and read `getComputedStyle(probe).color`
    so the browser normalizes it to a canonical `rgb(...)`; assert a known UI
    element's computed `color` **equals** that normalized value (both serialized
    identically by the browser — no hex-vs-rgb or shorthand pitfalls). Proves the
    element consumes the token, not a hardcoded literal. The coder documents the
    semantic property + element used for this check in `SPEC.md`.
  - **(d) fully self-contained + offline (listeners attached BEFORE `goto`; reject
    all remote requests — round 2 finding B2 + round 3 finding B7)** — the shared
    page-opening helper attaches `page.on("request")`, `page.on("pageerror")`, and
    `page.on("requestfailed")` collectors **before** every `page.goto` (requests
    emitted before attachment are otherwise lost), navigates, awaits
    fonts/resources, then asserts: (i) **every** request URL uses the `file:`
    scheme (or `data:`) — **no `http(s):` request at all**, whether it would
    succeed or fail — so the page is provably offline/self-contained, not merely
    "no failed loads" (an online CDN reference would otherwise pass); and (ii) the
    `pageerror`/`requestfailed` arrays are empty. This is the render-time offline
    proof; m5.6 adds the static scan.
- **m1.7 The story detail page carries the full document the brief requires
  (round 1, finding 2).** On `story-detail.html` (story 60.2) assert, beyond the
  as-a/i-want/so-that statement (m1.1): a **status pill**; at least one AC with
  its **Given / When / Then scenario steps** rendered underneath (assert the
  specific step text, not just an AC heading); the three **story-page action**
  controls **Create**, **Refine**, **Apply**
  (`getByRole("button", {name: /create|refine|apply/i})` — all three); and a
  labelled **fix toggle** (`getByRole("checkbox")` or `"switch"`). Each is a
  distinct assertion, so a page missing any one of them fails.
- **m1.8 The layout frame holds at the 1440px desktop reference (round 2, finding
  B4).** With the config's 1440-wide viewport, on a representative workspace page
  (`workspace-overview.html`) assert the three frame regions are all visible — the
  `mockup-sidebar`, the `mockup-breadcrumb`, and the content canvas — and that the
  page does not overflow horizontally at 1440 (`document.documentElement.scrollWidth
  <= clientWidth`, allowing a 1px rounding tolerance). Fails if the frame collapses
  or overflows at the reference width.

### `m2-mockup-navigation.spec.ts` — clickable flows + breadcrumbs

- **m2.1 Sidebar → section → epic → story flow.** Start on
  `workspace-overview.html`; click the Product epic node, land on `epic.html`
  (assert epic title). From `epic.html` click the 60.2 story row/link, land on
  `story-detail.html` (assert "60.2"). Each hop is a real `click()` that changes
  the page URL/heading — a dead or wrong `href` fails the landing assertion.
- **m2.2 Scenario list → scenario detail → linked story flow (link destination
  verified — round 3, finding B8).** From `scenarios.html` click the "E2E-601"
  row/link → land on `scenario-detail.html` (assert "E2E-601" + "mcp-service").
  Then **click the linked-story link** (E2E-601's originating story is 60.2 in the
  fixture) and assert it **navigates to `story-detail.html`** with the 60.2
  signature content — so an empty / fragment-only / wrong `href` fails (asserting
  the link merely *exists* would not). The brief requires scenario detail to link
  to the stories it was merged from.
- **m2.3 Runs → run detail flow (session-scoping surfaced, not in the file path
  — round 1, finding 11).** From `runs.html` click a run row → land on
  `run-detail.html` (assert the run state/outcome). The `<a href>` target is the
  flat `run-detail.html` (which resolves over `file://`); a real
  `sessions/<sid>/runs/<rid>` path could not resolve to a flat file without a
  server. Session-scoping is asserted **in presentation**: the run row on
  `runs.html` and the `run-detail.html` header both **visibly carry the owning
  session id** (via `data-session-id` / visible text), and the row href may
  carry a `?session=<sid>` query. Assert the same session id appears on the list
  row and the detail page. Fails if the run detail is not tied to a session.
- **m2.4 Breadcrumb trails are present and navigable on every workspace content
  page (round 1 finding 4 + round 2 finding A4).** Data-driven over **every
  workspace content page** — i.e. all pages except the root `sessions.html`:
  `workspace-overview`, `prd-overview`, `epic`, `story-detail`, `story-ambiguous`,
  `story-invalid`, `scenarios`, `scenario-detail`, `service`, `vocabulary`,
  `runs`, `run-detail`, and the three `prompt-*` pages (which inherit the
  underlying `run-detail` breadcrumb), and `unavailable`. For each assert a
  `mockup-breadcrumb` whose ordered trail reflects the page's tree path, whose
  **last crumb** names the current page (not a link), and whose **every
  non-current crumb is a link** with a resolvable `href`. Then click a
  representative non-current crumb (e.g. the epic crumb from `story-detail.html`)
  and assert it lands on its target. Fails if crumbs are plain text or point
  wrong. (The breadcrumb-bearing set is exported from `helpers/mockups.ts`
  alongside the page inventory so it can't drift.)
- **m2.5 Sidebar "return to sessions" leaves the workspace.** Click the
  return-to-sessions control from a workspace page → land on `sessions.html`.
- **m2.6 Story detail exposes the raw story file (real content, not an inert
  affordance — round 1, finding 3).** On `story-detail.html` activate the
  raw-file control (link/tab/`<details>`), assert its revealed/selected state
  changes, and assert the revealed raw view contains **specific signature lines
  from the actual 60.2 story file** (e.g. the exact `i_want:` text and an AC
  step string drawn from the fixture) — not merely that a control exists. (Kept
  as strong-specific-content rather than the byte-exact oracle the real app uses,
  since a static mockup need not paste the file verbatim.)
- **m2.7 Sidebar sections expand/collapse in place (round 1, finding 1).** The
  brief requires each section to "expand … in place" and Product epics to expand
  to stories. Author the tree with native disclosure (`<details>/<summary>`, no
  custom JS required) so it is testable with JS off. Assert: a section starts
  collapsed (descendants hidden), clicking its toggle expands it (`open`
  attribute / `aria-expanded` flips and the child nodes become visible), and the
  Product epic node further expands to reveal its story nodes 60.1/60.2/60.3.
  Fails if the tree is statically flat with no expand/collapse.

### `m3-mockup-degraded-states.spec.ts` — degraded + flagged states

- **m3.1 Inventory chip state vocabulary — the full frozen set (round 1,
  finding 10).** A design deliverable should define the *whole* chip vocabulary,
  so `workspace-overview.html` (or a documented "inventory states" catalog block
  on it) renders and the test asserts **every** status the frozen contract
  enumerates in `helpers/README-testids.md`: `present`, `missing`, `invalid`,
  `not_a_dir`, `present_empty`, `ambiguous`, `unknown` — each as a visibly
  distinguished chip (e.g. a `data-status="<status>"` chip per value). Keep the
  page's primary inventory summary realistic. Fails if any status is missing.
- **m3.2 Inventory truncation banner.** On `workspace-overview.html` (or a
  documented degraded variant) assert a visible truncation banner conveying the
  inventory read was degraded to fit the budget. Fails if absent.
- **m3.3 504 cli_timeout unavailable state + removed-session absence (round 1,
  finding 12).** `unavailable.html` shows an explicit **unavailable** state (not a
  fake data view), referencing the timeout. Separately, on `sessions.html` assert
  the *positive* content first — ≥1 **connected** session row carrying folder +
  version + test-connection — and then the *specific* absence: a session id the
  test data documents as "removed" (e.g. `sess-removed-01`) does **not** appear as
  a row, **and** there are no "disconnected"/"unreachable" row markers. (A bare
  placeholder page would fail the positive half.) This encodes "removed sessions
  simply disappear."
- **m3.4 Folder-conflict + architecture-path-mismatch warnings.** On
  `workspace-overview.html` assert a **folder-conflict** warning banner and an
  **architecture path-mismatch** warning banner are both present as design
  elements. Fails if either is missing.
- **m3.5 Epic AND story identity flags — both categories (round 1, finding 9).**
  The brief names "epic and story identity flags", so `epic.html` must render
  **both**: at least one **epic-scoped** identity flag (e.g. id-mismatch /
  duplicate-number / noncanonical-filename, on the epic header) **and** at least
  one **story-scoped** identity flag (e.g. id-mismatch / no-ACs / deprecated-format
  / empty-internal-id, on a story row). Assert each is attached to its
  corresponding header/row. Fails if only one category is present or the table is
  all-clean.
- **m3.6 Ambiguous story page with match count.** `story-ambiguous.html` shows an
  ambiguity notice including a **match count** (e.g. "2 matching files"). Fails
  if no count is shown.
- **m3.7 Invalid story page with parse error.** `story-invalid.html` shows a
  **parse-error** banner and access to the raw (unparseable) file. Fails if no
  error surface.
- **m3.8 Non-success run detail.** `run-detail.html` shows a **non-success**
  outcome badge (e.g. `not_fixed` / `error(<detail>)` / `max_attempts`) — assert
  the badge text is a non-`ok` outcome. Fails if it shows `ok`.
- **m3.9 Lifecycle chip vocabulary.** On `story-detail.html` (and/or `epic.html`)
  assert the lifecycle chips cover the vocabulary: a **created** chip, an
  **applied** chip reading "x/y" (e.g. "1/2") **or** "unknown", and a **refined**
  chip reading "not recorded". Fails if any lifecycle dimension is missing.

### `m4-mockup-prompts.spec.ts` — three CLI prompt dialog kinds over a page

Each prompt page = a run-detail canvas with an **open** native `<dialog>`
overlay (`role="dialog"`), demonstrating "CLI prompts stay as modal dialogs over
any page". The dialogs render open with zero JS (`<dialog open>`), so the specs
assert structure statically (no reliance on scripted open).

- **m4.1 Choice dialog.** `prompt-choice.html`: an open dialog whose title marks a
  **choice** prompt and which offers **Apply / Refine / Exit** controls
  (`getByRole("button", {name: /apply|refine|exit/i})` — assert all three).
- **m4.2 Numbered clarify dialog.** `prompt-clarify.html`: an open dialog marking a
  **clarify** prompt with **numbered options** (≥2 options carrying a visible
  1-based index) and a single-line answer input. Fails if options are unnumbered
  or the input is a multiline textarea.
- **m4.3 Multiline freetext dialog.** `prompt-freetext.html`: an open dialog
  marking a **freetext** prompt with a **multiline `<textarea>`** (assert the
  control is a `textarea`, distinguishing it from the clarify single-line input)
  and a submit control.
- **m4.4 The dialog overlays the run page.** For each of the three, assert the
  underlying run-detail content is present in the DOM beneath the open dialog
  (dialog is an overlay, not a separate bare page).

### `m5-design-mirror.spec.ts` — the S&F mirror is a real, offline deliverable (round 1, finding 7)

The mirror is a first-class brief deliverable but otherwise has no test. This
spec uses Node `fs` (no browser). **Path resolution (round 2, finding B3):** the
run command executes from `tests/harness/`, so the spec derives its roots from its
own location — `systemRoot = resolve(__dirname, "../../harness/design/system")`,
`designRoot = resolve(__dirname, "../../harness/design")` — never from
`process.cwd()`.

- **m5.1 Contract files present + non-empty.** `system/tokens.css`, `system/SYNC.md`,
  and the `system/fonts/`, `system/components/`, `system/guidelines/` directories
  all exist and are non-empty. Fails if any required mirror artifact is missing
  (the brief lists tokens, components, guidelines, fonts).
- **m5.2 Provenance recorded.** `SYNC.md` contains the **exact** source project id
  `147f5da0-fedd-4aaa-b0c2-ccb9f7d7b41e` and a valid **ISO-8601 sync date**.
  Fails if either is absent/malformed.
- **m5.3 Works offline (no remote deps — recursive over the whole mirror; round 2,
  finding A7).** Every **CSS** file anywhere under `system/` (`tokens.css` and any
  it `@import`s, plus component CSS) contains **no** `url(http…)` / `@import
  url(http…)` — every `url()` is a local path. (Markdown *prose* provenance links
  in `SYNC.md`/`guidelines/` are exempt — they may cite the source project URL and
  are never fetched at render time.) Fails if a render-time asset reaches the
  network.
- **m5.4 Referenced local fonts/assets exist (recursive).** For every CSS file
  under `system/`, each `@font-face` `url()` and other local `url()` resolves to a
  file that exists on disk under `system/`; if any CSS references `assets/…`, that
  file must exist. Fails if a referenced font/asset is missing (which would break
  offline rendering — the root cause behind the m1.6 font false-pass).
- **m5.5 The design spec exists and covers its required topics (round 2, finding
  B5).** `harness/design/SPEC.md` exists and is non-empty and mentions each of the
  seven brief-mandated topics (keyword presence, not exact heading syntax, to stay
  robust): layout frame (sidebar/breadcrumb/canvas), sidebar top-level ordering,
  token→UI mapping, page inventory, degraded-state catalog, the 1440px reference,
  and data provenance. Fails if `SPEC.md` is missing or omits a topic.
- **m5.6 The mockups reference nothing remote (static self-contained scan —
  round 3, finding B7).** Recursively scan every mockup **HTML / CSS / JS** file
  under `harness/design/mockups/` for `http://` / `https://` references (in
  `href`/`src` attributes and CSS `url()`/`@import`) and assert there are **none**
  — the mockups must be viewable offline with no server/build (brief). This is the
  static complement to the m1.6(d) render-time proof; between them a CDN font,
  script, stylesheet, or image can neither be referenced nor loaded. (Provenance
  Markdown is not scanned — it is never fetched at render time.)

**Red-state expectation:** before the coder creates `harness/design/mockups/`,
every `goto(file://…)` errors or renders an empty document, so all `m*` cases
fail (proper RED). They go green only once the mockups exist with the required
content, structure, links, and token wiring.

---

## Startup scaffolding (files created empty / behavior-free)

This design task has **almost no** service scaffolding — no Docker/compose edits,
no new service dirs, `harness/app/` stays absent. The only non-content files are
**test infrastructure** the test-author owns (they carry test logic, not
production behavior, and are allowed under `tests/`):

- **`tests/harness/playwright.mockups.config.ts`** — a new, minimal Playwright
  config: `testDir: "."`, one `mockups` project (`testMatch: **/m[0-9]*.spec.ts`),
  `workers: 1`, **no `globalSetup`/`globalTeardown`**, no `webServer`. Exists so
  the mockup specs run in isolation from the app-image build. Behavior-free w.r.t.
  production (it starts nothing).
- **`tests/harness/helpers/mockups.ts`** — a small helper exporting: the **single
  canonical `PAGES` inventory** (all 17 file stems, reused by m1.1/m1.6/m1.2/m5 so
  no page drifts — round 2, finding B6), the **`BREADCRUMB_PAGES`** subset (m2.4),
  a `mockupUrl(name)` → `file://` resolver
  (`pathToFileURL(resolve(__dirname, "../../harness/design/mockups", name)).href`),
  a `systemRoot`/`designRoot` resolver for the `fs`-based m5 (derived from
  `__dirname`, not `process.cwd()` — finding B3), the shared page-opening helper
  that attaches failed-resource listeners **before** `goto` (finding B2), and the
  mockup-only testid constants. No production behavior; a pure test helper. It does
  **not** import or extend `helpers/ui.ts`.

Note: the mockup **HTML files, `mockups.css`, `mockups.js`, `SPEC.md`, and the
`system/` mirror entrypoint are production deliverables (coder/orchestrator)**,
not scaffolding — the test-author must **not** stub them (stubbing would defeat
the RED state). No empty placeholders are created for them.

---

## Implementation (production changes that make the tests pass)

Because `harness/app/` stays absent, "production" = the design deliverable under
`harness/design/`. Layered by artifact:

### A. Design-system mirror (orchestrator OWNS end-to-end — round 1, finding 8)

- **Orchestrator (pre-coder), the ONLY actor with source access:** run DesignSync
  to pull the S&F project `147f5da0-fedd-4aaa-b0c2-ccb9f7d7b41e` into
  `harness/design/system/`, and **normalize + validate** it to the mirror contract
  before dispatching the coder: a single `tokens.css` entrypoint defining the S&F
  `:root` custom properties + `@font-face` from local `./fonts/` (re-exported from
  the source if DesignSync's export is split — the coder cannot do this, having no
  source access), non-empty `components/` + `guidelines/`, all `url()`s local, and
  `SYNC.md` with the exact project id + ISO sync date. The `m5` spec is the
  machine check of this contract.
- **Coder:** *consume only* — link `tokens.css`, use its tokens/fonts/assets. If
  the mirror fails the contract (missing entrypoint, split tokens, absent font
  binary), the coder does **not** invent tokens or fonts; it raises a blocker via
  the orchestrator (Challenges).

### B. Shared skin + minimal script (coder)

- `harness/design/mockups/assets/mockups.css` — the layout frame (left sidebar,
  breadcrumb bar, content canvas; 1440px reference width) and the token→component
  skin: it consumes `../../system/tokens.css` variables to style the tree, status
  pills, lifecycle/inventory chips, tables, and dialogs (per the SPEC token-map).
  Body type resolves to the Poppins token (satisfies m1.6b).
- `harness/design/mockups/assets/mockups.js` — MINIMAL only: sidebar
  expand/collapse and dialog open/close niceties. Pages must render correctly with
  JS disabled (dialogs use `<dialog open>`; nav is `<a href>`), so tests never
  depend on it.

### C. Mockup pages (coder) — realistic, fixture-drawn content

Author the 17 pages in the Folder layout, each with the shared sidebar/breadcrumb
frame, the `../system/tokens.css` link, mockup-only testids/landmarks the specs
key on, and cross-links for the flows. Content provenance (so the design "reads
true"):

- **Architecture** (`service.html`, `vocabulary.html`, sidebar Architecture tree)
  ← `a1-create-no-fix/.../architecture.yaml`: service `mcp-service`
  (path `services/mcp`, go, `net/http`, jest integration + `McpClient` helper) and
  the vocabulary block (`ask Claude`, forbidden qualifiers, forbidden action
  `handle`).
- **Product** (`prd-overview.html`, `epic.html`, `story-detail*.html`, sidebar
  Product tree) ← `prd.yaml` (title/summary/personas) + `p3-inventory-spread`
  epic 60 (stories 60.1/60.2/60.3, ACs, lifecycle states missing / 1/2 / 2/2).
- **Requirements/Scenarios** (`scenarios.html`, `scenario-detail.html`, sidebar
  flat list) ← `p3-inventory-spread/scenarios.yaml` (E2E-601/602/603 + service +
  linked stories; merged given/when/then) and `a5` `INT-901`.
- **Ops surfaces**: `runs.html` (session-scoped run rows), `run-detail.html`
  (non-success), workspace-overview build actions + inventory health + refresh,
  story-page actions (Create/Refine/Apply + fix toggle), Runs sidebar section.
- **Degraded/flagged**: chip-state matrix + truncation banner + folder-conflict +
  path-mismatch on `workspace-overview.html`; identity flags on `epic.html`;
  `story-ambiguous.html` (match count); `story-invalid.html` (parse error);
  `unavailable.html` (504 cli_timeout); non-success `run-detail.html`; lifecycle
  chip vocabulary on `story-detail.html`.
- **Prompt dialogs**: `prompt-choice.html` / `prompt-clarify.html` /
  `prompt-freetext.html` — run-detail canvas + an open `<dialog>` of each kind.

### D. Design spec (coder)

- `harness/design/SPEC.md` — the layout frame (sidebar / breadcrumb / canvas),
  the sidebar top-level ordering (workspace overview → Architecture → Product →
  Requirements/Scenarios → Runs → return to sessions), the token→UI mapping table
  (tree, status pills, chips, tables, dialogs → S&F tokens), the page inventory,
  the degraded-state catalog, the 1440px reference, and the data provenance.

### E. Tests (test-author, per §"End-to-end test cases")

- `tests/harness/playwright.mockups.config.ts`, `tests/harness/helpers/mockups.ts`,
  and `m1`–`m5` specs (m5 is the `fs`-based mirror-integrity spec). No edits to
  `playwright.config.ts`, `helpers/ui.ts`, `helpers/README-testids.md`, or any
  `p*`/`a*` spec.
- **Diff-scope check (round 1, findings 15/16 — RESOLVED, retained as a guard):**
  the orchestrator/reviewer confirms the final diff adds only files under
  `tests/harness/` (new `m*` specs + config + helper) and `harness/design/`, and
  leaves `harness/app/` absent and `playwright.config.ts` / `helpers/ui.ts` /
  `helpers/README-testids.md` / every `p*`/`a*` spec byte-for-byte unchanged.

---

## Codex rounds (ledger)

_Filled in during the Codex critique loop (≤3 rounds). Each round sends the full
task + full plan + all prior findings; the planner scores every finding
(composite 1–10 + four gates; keep iff ≥7 AND all gates pass)._

### Round 1

- Prompt: `./tmp/codex-uidesign-r1.md` (content archived in
  `./tmp/codex-uidesign-r1.run.log`). Response: same path, overwritten with the
  answer by the wrapper (label collision — harmless; findings archived in the
  ledger below).
- Codex returned 12 substantive findings (1–12) + 4 RESOLVED confirmations
  (13 file:// suitable, 14 separate-config isolation, 15 helper-naming no
  collision, 16 design-only scope). **All 12 substantive findings kept** (each
  composite ≥7, all four gates pass — they harden a brand-new spec set, so
  regression risk is nil and evidence is file-cited). Three were kept with a
  scope-preserving adjustment.

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Verdict + note |
|---|---|---|---|---|---|---|---|
| 1 | Sidebar expand/collapse untested | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → new **m2.7** via native `<details>` (no custom JS) |
| 2 | Story-detail omits status/steps/actions/fix | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → new **m1.7** |
| 3 | Raw-file test passes on inert control | 7 | ✅ | ✅ | ✅ | ✅ | KEEP (adjusted) → **m2.6** asserts specific real file content, not byte-exact (byte-exact over-fits a mockup) |
| 4 | Breadcrumb tested on one page only | 7 | ✅ | ✅ | ✅ | ✅ | KEEP → **m2.4** data-driven over all deep pages |
| 5 | Token check false-passes on broken font/link | 8 | ✅ | ✅ | ✅ | ✅ | KEEP (adjusted) → **m1.6** exact tokens.css URL + `document.fonts.check` + self-referential computed-vs-resolved-var (no hardcoded DesignSync var names) |
| 6 | Token check only "representative sample" | 7 | ✅ | ✅ | ✅ | ✅ | KEEP → **m1.6** loops every page |
| 7 | Mirror completeness/provenance untested | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → new **m5-design-mirror.spec.ts** |
| 8 | Handoff: mirror "ready before dispatch" vs coder normalizing | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → mirror contract + Impl A: orchestrator OWNS normalize/validate; coder consumes only |
| 9 | Identity flags: either epic OR story | 7 | ✅ | ✅ | ✅ | ✅ | KEEP → **m3.5** requires BOTH categories |
| 10 | Inventory chips: 3 states vs frozen 7 | 7 | ✅ | ✅ | ✅ | ✅ | KEEP → **m3.1** asserts all 7 frozen statuses |
| 11 | Session-scoped href path impossible over file:// | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → **m2.3** scoping surfaced in presentation, href stays flat |
| 12 | "Removed sessions disappear" negative too weak | 7 | ✅ | ✅ | ✅ | ✅ | KEEP → **m3.3** positive connected rows + specific removed-id absence |
| 13–16 | RESOLVED (file://, isolation, naming, scope) | — | — | — | — | — | No change needed; 13's pageerror/fonts hardening folded into m1.6(d); 15/16 diff-guard added to Impl E |

Kept: 12. Skipped: 0 (the four RESOLVED items are confirmations, not findings).

### Round 2

- Prompt: `./tmp/codex-uidesign-r2.md`. Response: `./tmp/codex-uidesign.md`.
- Part A (re-check of round-1 fixes): **RESOLVED** — 1, 2, 3, 6, 8, 9, 10, 11, 12,
  13, 14, 15, 16. **NOT-RESOLVED (partial)** — 4 (breadcrumb set still omitted
  some pages), 5 (`document.fonts.check` not a reliable loaded-oracle), 7 (m5
  only checked the entrypoint, not the whole mirror). Part B: 6 new findings.
- **9 open items kept, 0 skipped** (all composite ≥7, all gates pass — corrections
  to a not-yet-written spec set; A4 and A7 kept with scope-preserving adjustments).

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Verdict + note |
|---|---|---|---|---|---|---|---|
| A4 | Breadcrumb set omits prd/scenarios/runs/prompt/unavailable | 7 | ✅ | ✅ | ✅ | ✅ | KEEP (adj) → **m2.4** over all workspace content pages (sessions excluded; prompts inherit run-detail) |
| A5 | `document.fonts.check` not a reliable font-loaded oracle | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → **m1.6(b)** uses `fonts.load()` + `FontFace.status==="loaded"` |
| A7 | m5 checked only the entrypoint, not the whole offline mirror | 7 | ✅ | ✅ | ✅ | ✅ | KEEP (adj) → **m5.3/m5.4** recurse all mirror CSS; assets/ refs must exist; markdown provenance links exempted |
| B1 | Computed-token equality flaky across CSS serialization | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → **m1.6(c)** one longhand `color`, normalized via a probe element |
| B2 | Failed-resource listeners attached too late | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → helper attaches listeners **before** `goto` (m1.6(d)) |
| B3 | m5 cwd/path-resolution ambiguity | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → m5 derives roots from `__dirname` |
| B4 | 1440px reference viewport untested | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → config `viewport:1440×900` + **m1.8** frame/overflow assertion |
| B5 | `SPEC.md` contents untested | 7 | ✅ | ✅ | ✅ | ✅ | KEEP → **m5.5** SPEC.md non-empty + seven required topics |
| B6 | Page-count language inconsistent (17 vs "~18"/"12+3") | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → canonical 17-page `PAGES` inventory in helpers; m1.1 covers all 17; counts fixed |

Kept: 9. Skipped: 0.

### Round 3 (final — 3-round cap reached)

- Prompt: `./tmp/codex-uidesign-r3.md`. Response: `./tmp/codex-uidesign3.md`.
- Part A: **all 9 round-2 items RESOLVED** against the current plan (A4, A5, A7,
  B1–B6). Part B: 2 new blocking findings, **both kept** (composite 8, all gates
  pass — core offline/self-contained + scenario-link brief requirements; no
  regression).

| # | Finding | Composite | Correct | Evidence | Scope | Regress | Verdict + note |
|---|---|---|---|---|---|---|---|
| B7 | Remote deps that *load OK* slip past (m1.6(d) only caught failures; m5 only scanned mirror CSS) | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → **m1.6(d)** asserts every request is `file:`/`data:` (no `http(s)` at all); new **m5.6** static scans mockup HTML/CSS/JS for remote refs |
| B8 | Scenario→story link destination untested (a wrong/empty href passes) | 8 | ✅ | ✅ | ✅ | ✅ | KEEP → **m2.2** clicks the linked story and asserts arrival on `story-detail.html` (60.2) |

Kept: 2. Skipped: 0.

**Loop closed at the 3-round cap.** Net: **23 findings kept, 0 skipped** across the
three rounds (round 1: 12; round 2: 9; round 3: 2), plus the round-1 RESOLVED
confirmations. The composite stayed ≥7 with all gates green on every keep.

---

## Challenges

_Filled by the orchestrator if a blocker arises._

---

## Workflow log

- 2026-08-01 19:40 — Baselines captured (`./tmp/implement-task/harness-workspace-ui-design/`):
  production manifest 228 files (src/templates/true-bdd; harness/app absent),
  off-limits manifest 250 files (tests/ minus node_modules + ephemeral
  test-results), package-scripts snapshot, change-surface copy (2.7M), HEAD
  ba52d00 (clean except the untracked task brief). Note: ephemeral Playwright
  output dirs (`test-results/`) excluded from manifests as generated artifacts.
- 2026-08-01 19:45–20:15 — Orchestrator DesignSync pull (parallel to planning):
  S&F project 147f5da0… mirrored file-by-file. Both Poppins TTFs verbatim; both
  gradient PNGs exceed the 256 KiB per-file transfer cap → local CSS-fallback
  re-renders substituted and documented (SYNC.md + MIRROR-NOTES.md deviations).
- 2026-08-01 20:30 — Phase 1.1 planner (Opus) done: plan written + 3 Codex
  rounds, 23 findings kept / 0 skipped.
- 2026-08-01 20:35 — Mirror installed + normalized to the plan's contract at
  `harness/design/system/` (84 files): single `tokens.css` entrypoint
  (fonts → `./fonts/`, gradients → `./assets/gradients/`), SYNC.md provenance,
  as-pulled `tokens/`+`styles.css` kept (font URLs re-based). Contract
  validation script: all checks OK (files present, provenance exact, zero
  remote CSS url(), every local url() resolves).
- 2026-08-01 21:25 — Phase 1.2 test-author (Opus) done: 7 new files
  (playwright.mockups.config.ts, helpers/mockups.ts, m1–m5 specs; 154 cases).
  RED confirmed: 150 failed (148 ERR_FILE_NOT_FOUND navigations + m5.5/m5.6 fs
  assertions), 4 passed (m5.1–m5.4 mirror checks — expected, mirror is the
  orchestrator deliverable). Isolation proven both directions (mockups config
  lists only m*; default config lists only p*/a*). Codex loop: 14 kept /
  2 skipped / 1 already-applied over 3 rounds.
- 2026-08-01 21:30 — Orchestrator scope check on the test-author: production
  manifest diff CLEAN (src/templates/true-bdd untouched, harness/app absent);
  off-limits diff shows zero modified existing files, only the 7 new test
  files. Package scripts unchanged. Phase 2 "before" snapshots taken; coder
  (Sonnet) dispatched with the reproduce block verbatim.
- 2026-08-01 22:15 — Phase 2 coder (Sonnet) done: 21 files under
  harness/design/ (17 pages + mockups.css/js + SPEC.md), suite 154/154 green,
  Codex loop 7 kept-fixed / 1 skipped (row-hover tint; Codex agreed).
  Orchestrator verification: off-limits diff CLEAN, package scripts CLEAN,
  production dirs CLEAN, mirror untouched; suite re-run by the orchestrator —
  154/154 green, isolation 63/154 confirmed.
- 2026-08-01 22:40 — Phase 3 review run INLINE by the orchestrator (the Fable
  reviewer agent fails to spawn in this repo — known issue). Change-surface
  diff: production dirs byte-identical; adds only 7 test files + 104
  harness/design files. Live smoke via Playwright browser over a local HTTP
  server (MCP blocks file:): sessions → workspace-overview → story-detail →
  prompt-clarify all render on-brand; CLI smoke N/A (no CLI surface changed).
  Codex review round 1: 14 findings → 11 KEPT (section breadcrumbs + m2.8,
  runs session-coherence, m2.7b Architecture expand, m1.9 epic-table
  semantics, m1.7 AC-scoping + named fix toggle, m6 no-JS spec, scrim
  dismissal fix, m3.6/m3.7 false-pass holes, m1.8 all-pages clip oracle,
  code/kbd/samp Poppins fix) and 3 SKIPPED with justification (representative-
  destination idiom — documented in SPEC §9; strict m5.6 scan is deliberate;
  gradient tokens render as proven + verbatim mirror component; component
  cards' CDN refs documented in SYNC.md — offline binds to fonts/assets).
  After keeps: suite 180/180 green (26 new/hardened cases), frozen files
  still byte-for-byte untouched, isolation 63/180. Codex round 2 dispatched
  to verify dispositions.
- 2026-08-01 23:10 — Peter asked why Phase 3 runs inline; orchestrator
  test-spawned the implement-task-reviewer agent (spawn now WORKS — the old
  failure is stale), but Peter said "don't do that" mid-launch; agent stopped
  before making any change; review continued inline per his instruction.
- 2026-08-01 23:20 — Final Codex review round 2 (./tmp/codex-review-final-r2.md
  → ./tmp/codex-review-final-answer2.md): 8/10 applied items RESOLVED, all 4
  skips AGREED (with W3C citation on the custom-property url() point), frozen
  files verified byte-for-byte against HEAD blobs, isolation verified, NO new
  findings. Two round-1 items were completed rather than fully resolved:
  F4 → new m2.7c toggles the Requirements and Runs sections; F5 → m1.9 now
  pins thead th count = 6 and exactly 3 tbody story rows each with pill+chip.
  Suite after completions: 182/182 green; default config still 63 (no m*
  leakage). Loop CLOSED at round 2: zero new findings and the two remaining
  items are mechanical assertions the green suite itself now proves — a third
  round would be pure confirmation (adaptive exit).
- 2026-08-01 23:25 — Final manifests: production CLEAN, harness/app absent,
  package scripts CLEAN, off-limits 0 modified / 8 added (the task's own
  specs + config + helper). Live smoke evidence: tmp/design-research/smoke-*.png.
  CLI smoke skipped-with-note (no CLI surface in this task). DONE.
