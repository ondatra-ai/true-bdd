<!-- See docs/context/requirements-guide.md. -->
# Workspace visual jank fixes

## Goal:

Eliminate the visual defects found by the instrumented walk of the workspace
(report: `tmp/visual-walk/report.md`, 2026-08-05): layout jumps on Home entry
and first load, the oversized chat dock default, the flyout sliver artifact,
and the silently-empty architecture outline.

## Current behavior:

See the report. Numbers established live against `true-bdd-harness:e2e`:
Home entry shifts the canvas 39 px (CLS 0.194, 3/3 — placeholder meta
"… — session <id>" h=20 becomes the wrapped folder path h=59 ~100 ms later);
the chat dock computes a hard `width: 768px` leaving a 310 px content pane at
1440×900 (prototype baseline `ChatDialog.js` `DEFAULT_WIDTH = 380`); the rail
flyout renders w=260 over the w=280 docked sidebar at the same x (prototype:
`.rail-flyout { left: var(--rail-w) }` + the `.mockup-sidebar` class — same
width), letting a 20 px sliver of the underlying section peek through; an
`architecture:`-wrapped architecture.yaml (with `vocabulary:` instead of
`terms:`) renders 0 outline rows with no hint of any kind (A/B: top-level
shape renders 2/2/1); Poppins font-swap nudges text ~300 ms after first paint.

## Requirement

### Product

- **P1 [revealed]** A BDD System Architect should see the Home canvas render
  without a post-paint jump: once the workspace-overview canvas is visible,
  entering the live session data (folder path in the meta line, inventory
  rows) must not move the inventory/subhead — no non-input layout shift on
  Home entry.
- **P2 [revealed]** A BDD System Architect should get the docked chat at the
  prototype's default width (380 px) when opening it, leaving the content
  pane usable at a 1440-wide viewport; the drag-resizer remains.
- **P3 [revealed]** A BDD System Architect should see the hover flyout cover
  the docked sidebar exactly (same x, same width, full height) — never a
  sliver of the underlying section peeking through its edge.
- **P4 [revealed]** A BDD System Architect should see an explicit empty
  indicator in each fixed outline group (Services / Terms / Docker) when the
  architecture file declares no entries for it — never a silently bare group
  header (the file may be valid YAML in a non-contract shape).
- **P5 [revealed]** A BDD System Architect should see the architecture page
  reach its settled layout at first paint — no non-input layout shifts (font
  swap included) while an idle page loads.

## Non-goals

- The breadcrumb trail model (Home-rooted vs "Sessions / Workspace overview"
  on home/product-file pages) is DELIBERATE — documented in the derivation's
  own contract and pinned by the w13/w16 parity suites. Not touched.
- The long-line horizontal-scroll affordance in the file editor (cosmetic;
  macOS overlay scrollbars) — out of scope.
- The chat dock's resize behavior beyond the default width.

## Established facts

- Full report + telemetry + videos: `tmp/visual-walk/` (report.md,
  layout-shifts*.json, boxes*.json, walk*.webm, GIFs). Repro scripts:
  `walk.mjs`, `walk2.mjs`, `probe.mjs` (Playwright via
  tests/harness/node_modules).
- Home jump mechanism (probe-verified): `overview-meta` first paints
  "… — session <id>" (h=20, folder unknown), then session detail lands and
  the full absolute folder path wraps to h=59, pushing `overview-inventory`
  y 215→254. The folder is ALREADY available synchronously from
  `GET /api/sessions` (the sessions list renders it) — no need to await the
  detail read to size the meta.
- Chat: production `chat-dock-panel` computed `width: 768px` (fixed);
  prototype `ChatDialog.js` has `DEFAULT_WIDTH = 380` (user-resizable via
  `chat-dock__resizer`). ClickUp reference: "~40% of window at 1920" —
  the 768 was that measurement frozen into a constant.
- Flyout: production `rail-flyout` box `{x:76,w:260}` vs sidebar
  `{x:76,w:280}` (position:fixed, z-50). Prototype `.rail-flyout` sits at
  `left: var(--rail-w)` and carries the `.mockup-sidebar` class → identical
  width (`--wk-sidebar-w`).
- Outline A/B (same build, two live sessions): top-level `services:/terms:/
  docker:` → 2 service rows, 2 term rows, 1 docker row, 2 details regions;
  `architecture:`-wrapped + `vocabulary:` → 0/0/0/0, no
  `yaml-invalid-indicator` (file is valid YAML), no empty hint. Fixture with
  the wrapped shape: `tests/harness/fixtures/p3-inventory-spread/input/docs/
  architecture/architecture.yaml`.
- Font swap: layout-shift at t≈305 ms in BOTH walks (breadcrumb "Architecture"
  78→85 px, sidebar labels 57→60 / 41→46 px, 1 px vertical nudge), CLS value
  ~0.00001 — visible as a text pop; fonts are served locally.
- Section-switch chrome is stable (0 box-moves across 2 cycles + 6 rapid
  flips) and the save cycle causes no layout movement — do not "fix" those.
- Existing suites already pin: outline rows/jumps on the happy shape (w3),
  chat dock presence/resizer (w6), rail flyout presence (w1), design parity
  (w8–w17), sessions home (w18/w19). New assertions must not contradict them.
  NOTE: any existing spec/golden that measured the 768 px chat default or the
  260 px flyout would need reconciling — grep before pinning.
