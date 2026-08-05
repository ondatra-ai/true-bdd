# Product section prototype parity

## Goal

The production workspace (`harness/src`) must match the file-as-source
prototype — the design truth — for the Product section and the shell it sits
in: opening prod's Product pages must look and behave like
`harness/design/proto-workspace` (`npm run dev -- -p 3999` → `/product`,
`/story/<id>`, `/feature/<id>`, `/features`, `/scenarios`). Tests first: every
surface below is demanded by a red e2e spec (deterministic + codex vision
judge) before any production code changes.

## Design truth & references

- **The prototype app IS the baseline**: `harness/design/proto-workspace/app`
  (runnable Next.js; in-memory seed data). Its README and
  `docs/tasks/workspace-file-as-source-ui.md` (the distilled brief) describe
  intent. The static mockups' visual skin (tokens) still applies via
  paths.yaml → design_system.
- Reference captures (viewport 1440×900) under `tmp/proto-reference/`:
  `product.png`, `story-60-1.png`, `feature-summaries.png`, `features.png`,
  `scenarios.png`, `workspace-overview.png`, plus `product-snapshot.md`
  (accessibility tree).

## Constraints

- Prod stays REAL: all data flows through the existing CLI relay
  (SessionDetail, doc read/write) — never hardcode the prototype's seed
  content. Tests drive the `w-workspace-happy` fixture.
- The existing w1–w11 + a10 suites and the binding testid contract stay green;
  extend testids additively (README-testids.md documents new ones).
- Styling only from the design system (tokens/components; paths.yaml →
  design_system). No ad-hoc values.
- The judge's baseline screenshots must come from the prototype app in the
  repo (the suite may boot it on a free port); never from committed PNGs.

## Requirements

- **R1 — branded sidebar (shell-wide).** The secondary sidebar must match the
  prototype: "TrueBDD" brand header with a workspace context label; per
  section, an underlined section header (e.g. `02—PRODUCT`); the section's
  file row (e.g. `PRD`) highlighted when current; subsection groups
  `Features:`, `Stories:`, `Scenarios:` listing the REAL workspace items
  (from the session's docs) with a vertical guide line; a `+ New story`
  affordance; the current page's row visibly selected.
- **R2 — rail parity.** Rail entries render an icon plus a small-caps label
  (HOME / ARCHITECTURE / PRODUCT / BUILDS), active entry as an inverted tile,
  with the sessions entry pinned at the bottom — per the prototype's rail.
- **R3 — product landing (`…/product`).** Renders `docs/prd/prd.yaml` as the
  prototype does: kicker line (`02—PRODUCT`), display title (`prd.yaml`),
  muted subtitle, then a GitHub-style file card — header bar with the doc
  path and a `N lines` counter, line-number gutter, monospace body, editable
  in place (existing edit round-trip preserved).
- **R4 — story page.** Same file-card anatomy for a story file; kicker
  `02—PRODUCT / <id>`; title `<id> — <story title>`; a `Feature: <name>`
  pill with a CHANGE control (opens the feature picker; assigning updates the
  story's `feature:` field through the relay); breadcrumb is the deep trail
  `Sessions / Workspace overview / Product / <file>.yaml`.
- **R5 — feature detail page.** Kicker `02—PRODUCT / FEATURES / <NAME>`,
  title = feature id, description subtitle; sections **User stories**,
  **Requirements**, and **Unaligned requirements** — card-row lists where
  each row shows the item title (link) and its `Feature:` pill control;
  unaligned rows show `Feature: (none)`. Content aggregated live from the
  session's stories/scenarios `feature:` tags.
- **R6 — features page.** Renders `features.yaml` with the same file-card
  anatomy (as the prototype's `/features`).
- **R7 — scenarios page.** Matches the prototype's `/scenarios`
  (requirements/scenarios listing with per-item feature tags and links).
- **R8 — vision judge on product pages.** The design-judge suite gains
  prototype-baseline pairs for at least: product landing, story page, feature
  detail. Named checks per page (sidebar_structure, rail_labels,
  kicker_title_block, file_card, feature_pill / section_lists as
  applicable); data values ignored, structure asserted; judge boots the
  prototype from the repo for baseline screenshots; skips cleanly without
  `codex`.
- **R9 — red first.** Every spec fails as an assertion against current prod
  before the fixer runs.
