# TrueBDD harness workspace UI — design spec

Status: **design only** (mockups + spec + local design-system mirror). No
harness production code (`harness/app/`) is part of this task — see the task
brief `docs/tasks/harness-workspace-ui-design.md` and plan
`docs/tasks/plans/harness-workspace-ui-design.md`.

This document is the short design spec the mockups implement. It covers: the
layout frame, the sidebar top-level ordering, the token→UI mapping, the full
17-page inventory, the degraded-state catalog, the 1440px desktop reference,
and data provenance.

---

## 1. Layout frame — sidebar / breadcrumb / canvas

Every workspace page (every mockup page except `sessions.html`, the
pre-workspace list) renders three persistent regions, ClickUp's information
architecture rendered in the S&F Swiss/editorial skin:

```
┌───────────────┬──────────────────────────────────────────┐
│               │  breadcrumb (48px, hairline bottom border) │
│   sidebar     ├──────────────────────────────────────────┤
│   280px       │                                            │
│   fixed       │   canvas (content, 40px inner padding)     │
│               │                                            │
└───────────────┴──────────────────────────────────────────┘
```

- **Sidebar** (`data-testid="mockup-sidebar"`) — persistent left navigation,
  `280px` fixed width, hairline right border. Holds the workspace-overview
  link, the three document sections, the Runs section, and the return-to-
  sessions link.
- **Breadcrumb** (`data-testid="mockup-breadcrumb"`) — a horizontal trail
  above the canvas reflecting the current tree path (session / section /
  epic / story). The last crumb names the current page and is not a link
  (`aria-current="page"`); every earlier crumb is a real `<a href>` resolving
  to an existing mockup page.
- **Canvas** (`data-testid="mockup-canvas"`) — the main content region.
  Grouped tables (status pills, lifecycle columns, identity flags), full-page
  detail views, and degraded-state banners all render here.

`sessions.html` is the one page outside the workspace frame — it has no
sidebar/breadcrumb, only a top bar and the sessions list, per the brief
("land on a sessions list … by opening a connected session, enter a
workspace").

## 2. Sidebar top-level ordering

Top to bottom, every workspace page:

1. **Workspace overview** (`workspace-overview.html`) — flat link.
2. **Architecture** (`sidebar-section-architecture`) — expandable, service
   node(s) + a Vocabulary node.
3. **Product** (`sidebar-section-product`) — expandable, PRD-overview root +
   epic → story tree.
4. **Requirements / Scenarios** (`sidebar-section-requirements`) —
   expandable, a FLAT sibling list of scenario rows (no epic/story nesting).
5. **Runs** (`sidebar-section-runs`) — expandable, links into `runs.html`.
6. **Return to sessions** — flat link back to `sessions.html`, pinned to the
   bottom of the sidebar.

All four document/runs sections are authored as native `<details>/<summary>`
— no custom JS required to expand/collapse. The Product section's epic node
is itself a nested `<details>`, so "sidebar → section → epic → story" is
three levels of native disclosure.

**Default open/closed state:** on `workspace-overview.html` every section
(and the Product epic node) renders **closed by default**, so the click-to-
expand affordance itself is demonstrable there. On every other workspace
page, the sections render **open by default** — the sidebar shows the
current tree context on arrival (e.g. `story-detail.html` opens with
Product → Inventory Spread Epic already expanded) rather than making a
reader re-expand the whole tree on every page load. Both states use the same
native `<details>` markup; only the `open` attribute differs.

## 3. Token → UI mapping

The mockups link `../system/tokens.css` (the S&F mirror entrypoint) BEFORE
`assets/mockups.css`, which only ever reads S&F custom properties via
`var(--token)` — colours, type, radius, borders are never hardcoded hex. A
small set of LOCAL, non-token custom properties (`--wk-fs-*`, `--wk-sidebar-w`,
`--wk-breadcrumb-h`) hold workspace-density values (13–16px row/table/chip
text) that the S&F editorial type scale (`--fs-body:20px` → `--fs-h1:84px`)
does not define — those values are reserved for headings (`<h1>` uses
`--fs-h3` at workspace scale) per the density note in the task brief.

| UI part | S&F token(s) | Notes |
|---|---|---|
| Sidebar tree (sections, epic/story nodes) | `--text-body`, `--text-muted`, `--border-hairline`, `--ls-label`, hover → `--surface-inverse`/`--text-inverse` | Uppercase section labels (`01—Architecture`) via `text-transform` + `--ls-label`, never authored in caps. |
| Breadcrumb | `--text-muted` (links), `--text-primary` + `--fw-bold` (current), `--link-color-hover` on hover | |
| Status pill | `--surface-inverse` / `--text-inverse`, `--radius-none` | Solid inverted chip — the story/epic "Planned" status. |
| Inventory chip (`data-status`) | `--status-success` (present), `--status-warning` (present_empty / ambiguous), `--status-error` (missing / invalid / not_a_dir), `--text-muted` (unknown) | Colour used for STATUS only, per the monochrome-by-default rule. |
| Lifecycle chip (`data-kind`) | `--surface-inverse` (created), `--status-warning` (applied partial), `--status-success` (applied complete, `data-complete="true"`), `--text-muted` (refined) | |
| Identity flag | `--status-warning`, `--radius-none` | Epic-scoped (`mockup-epic-flag`) and story-scoped (`mockup-story-flag`). |
| Data table | `--border-hairline` (`border-collapse: collapse` — adjacent cells share one hairline), `--surface-subtle` on row hover | |
| Banner (truncation / folder-conflict / path-mismatch / parse-error) | `--status-warning` / `--status-error` border + title colour | |
| Dialog (`prompt-dialog`) | `--border-strong`/`--border-width-strong`, `--surface-page`, `--surface-inverse` (scrim, fully opaque — no translucency) | Renders via `<dialog open>` — no JS required to be visible; CSS positions it as a centred modal over an opaque inverse-surface field (value contrast, not a translucent overlay). |
| Gradient band | `--gradient-spiral-soft` (the CSS-fallback radial stops, never the `url()` image token — see §8) | Used exactly ONCE, on `sessions.html`'s top bar. |
| Buttons | `--border-strong`, uppercase + `--ls-button`, hover inverts to `--surface-inverse`/`--text-inverse`, `:active` → `translateY(1px)`, `:disabled` → 35% opacity | |

**Token probe (m1.6c):** every page carries one `.mockup-brand` element
("TrueBDD") with `data-token-probe="--text-primary"` and `color:
var(--text-primary)` — on workspace pages it is the sidebar's brand mark; on
`sessions.html` (no sidebar) it is the top-bar wordmark
(`data-token-probe="--text-inverse"` there, since the top bar sits on the
gradient/dark band — both are real, declared S&F tokens). This is a plain
`<span>`, never wrapped in an `<a>`, so no other CSS rule's specificity can
shadow the `color: var(--token)` declaration; the live-mutation check
(setting `--text-primary`/`--text-inverse` on `:root` and reading the
element's recomputed `color`) proves the element consumes the token rather
than a hardcoded literal.

## 4. Page inventory (17 pages)

> **Baseline moved (2026-08-04):** the static mockup set this section describes
> was retired and deleted; the runnable prototype
> (`harness/design/proto-workspace/`) superseded it as the per-screen design
> baseline (paths.yaml → design_system). The inventory below is retained as the
> historical catalog of screens and their intent; the prototype's routes are the
> living equivalents.

Historically under `harness/design/mockups/`:

| Filename | Purpose |
|---|---|
| `sessions.html` | Pre-workspace sessions list — folder, CLI version, test-connection control per row. |
| `workspace-overview.html` | Session workspace root — inventory health, build actions, refresh, degraded-state catalog. |
| `prd-overview.html` | Product section root — PRD title/summary/personas. |
| `epic.html` | Epic detail — grouped story table, epic + story identity flags. |
| `story-detail.html` | Story 60.2 full detail — statement, status/lifecycle chips, ACs with steps, actions, fix toggle, raw file. |
| `story-ambiguous.html` | Degraded: ambiguous story lookup (match count). |
| `story-invalid.html` | Degraded: story parse error + raw file. |
| `scenarios.html` | Requirements/Scenarios root — flat scenario table. |
| `scenario-detail.html` | Scenario E2E-601 detail — merged steps, linked story. |
| `service.html` | Architecture service detail — mcp-service. |
| `vocabulary.html` | Architecture vocabulary — actions, forbidden qualifiers/actions. |
| `runs.html` | Session-scoped run history. |
| `run-detail.html` | Run detail — non-success outcome, output tail. |
| `prompt-choice.html` | Run detail + open choice `<dialog>` (Apply/Refine/Exit). |
| `prompt-clarify.html` | Run detail + open numbered-clarify `<dialog>`. |
| `prompt-freetext.html` | Run detail + open multiline-freetext `<dialog>`. |
| `unavailable.html` | 504 `cli_timeout` unavailable state. |

## 5. Degraded & flagged states catalog

- **Inventory chip vocabulary** (`workspace-overview.html`) — the full frozen
  seven-status set: `present`, `missing`, `invalid`, `not_a_dir`,
  `present_empty`, `ambiguous`, `unknown`, both in a realistic inventory list
  and in a dedicated status legend.
- **Truncation banner** (`workspace-overview.html`) — the inventory read was
  truncated/degraded to fit the reply budget.
- **Folder-conflict banner** (`workspace-overview.html`) — a sibling owner
  active in the same project folder.
- **Architecture path-mismatch banner** (`workspace-overview.html`) —
  configured architecture path differs from the canonical one.
- **Epic identity flag** (`epic.html`, on the epic header) — e.g.
  noncanonical filename.
- **Story identity flag** (`epic.html`, on a story row) — e.g. id mismatch.
- **Ambiguous story page** (`story-ambiguous.html`) — match-count notice ("2
  matching files").
- **Invalid story page** (`story-invalid.html`) — parse-error banner + raw
  (unparseable) file.
- **504 `cli_timeout` unavailable state** (`unavailable.html`) — explicit
  unavailable view, not a fake data view.
- **Removed session absence** (`sessions.html`) — connected rows render
  normally; a removed session id never appears as a row, and there is no
  disconnected/unreachable marker (sessions are gone on disconnect, per the
  v2 API contract in `helpers/README-testids.md`).
- **Non-success run outcome** (`run-detail.html`) — `not_fixed`.
- **Lifecycle chip vocabulary** (`story-detail.html`, also `epic.html`) —
  created / applied `"x/y"` or `unknown` / refined `"not recorded"`.

## 6. Desktop reference

Mockups target a **1440px-wide desktop reference viewport**
(`playwright.mockups.config.ts` pins `1440×900`). The sidebar is a fixed
280px column; the canvas fills the remaining ~1160px. No page overflows
horizontally at this width (`overflow-x: hidden` on the canvas, wrapping
tables/`<pre>` blocks).

## 7. Data provenance

Content is fixture-drawn, not invented, so the design "reads true":

- **Architecture** (`service.html`, `vocabulary.html`, sidebar Architecture
  tree) ← `tests/harness/fixtures/a1-create-no-fix/input/docs/architecture/architecture.yaml`
  — service `mcp-service` (path `services/mcp`, Go, `net/http`, Jest
  integration tests at `tests/jest.config.ts`, `McpClient` helper) and the
  vocabulary block (`ask Claude`, forbidden qualifiers `properly` /
  `correctly` / `seamless`, forbidden action `handle`).
- **Product** (`prd-overview.html`, `epic.html`, `story-detail.html`, sidebar
  Product tree) ← `tests/harness/fixtures/a1-create-no-fix/input/docs/prd/prd.yaml`
  (title, summary, both personas — Claude User / Developer/Maintainer) +
  `tests/harness/fixtures/p3-inventory-spread/input/docs/prd/epics/epic-60-inventory-spread.yaml`
  and `.../docs/prd/stories/60.2-summary-shared-docs.yaml` (epic 60,
  stories 60.1/60.2/60.3, lifecycle states missing / applied 1/2 / applied
  2/2).
- **Requirements/Scenarios** (`scenarios.html`, `scenario-detail.html`,
  sidebar flat list) ←
  `tests/harness/fixtures/p3-inventory-spread/input/docs/scenarios.yaml`
  (E2E-601/602/603, service + linked stories, merged given/when/then) and
  `tests/harness/fixtures/a5-build-tests-fix/input/docs/scenarios.yaml`
  (INT-901).
- **Ops surfaces** (`runs.html`, `run-detail.html`, prompt dialogs) — session
  ids, commands, and outcomes follow the run vocabulary documented in
  `tests/harness/helpers/README-testids.md` (`state`/`outcome` enums).

## 8. S&F design-system mirror

`harness/design/system/` is the offline S&F mirror (tokens, fonts,
components, guidelines, assets) pulled from claude.ai/design project
`147f5da0-fedd-4aaa-b0c2-ccb9f7d7b41e`; see `system/SYNC.md` for the sync
date and `system/readme.md` for the full rulebook this design follows
(monochrome ramp, Poppins-only type, zero radius, no shadows, hairline
borders, uppercase-via-CSS, hover-inverts, at-most-one blurred radial
gradient per page, no icons/emoji beyond `×` and `—`).

The mockups use the **CSS-fallback** gradient (`--gradient-spiral-soft`,
built from colour stops) rather than the `--gradient-image-*` `url()`
tokens: a relative `url()` inside an S&F custom property resolves against
the stylesheet where the `var()` is *used* (`assets/mockups.css`), not where
the token is *declared* (`system/tokens.css`) — using the image token from
`mockups.css` would resolve to a path that does not exist. The CSS-fallback
stops avoid this entirely and are explicitly sanctioned by the guideline
("Prefer the real artwork … over the CSS fallbacks" — implying the fallback
is a legitimate, if secondary, choice) while keeping every mockup page fully
offline and self-contained.

## 9. Mockup conventions & known idioms (review round)

- **Representative destinations.** Exactly one page models each entity type
  (one story detail, one scenario detail, one run detail). Sibling records
  (stories 60.1/60.3, scenarios E2E-602/603/INT-901, other runs) link to that
  representative page rather than to per-record variants — a deliberate
  static-mockup idiom, not an information-architecture statement. The real
  implementation routes each record to its own URL.
- **Breadcrumb section level.** Product-tree pages (epic, story pages) carry a
  `Product` crumb → `prd-overview.html`; Requirements pages carry a
  `Requirements / Scenarios` crumb → `scenarios.html`. Architecture leaf pages
  (`service.html`, `vocabulary.html`) have no section crumb because the
  Architecture section has no root page in this 17-page iteration — the
  section is conveyed by the numbered eyebrow (`01—Architecture / …`); an
  architecture overview page is a candidate for the next iteration.
- **Runs are session-scoped.** Every run row on `runs.html` and the
  `run-detail.html` header carry the same owning session
  (`sess-alpha-7f3`) — the workspace never mixes another session's runs.
- **Dialog scrim.** The prompt dialogs render open via `<dialog open>`, which
  has no native `::backdrop`; an opaque sibling `.mockup-scrim` supplies it.
  The Exit/close nicety in `mockups.js` hides the scrim together with the
  dialog so dismissal restores the underlying run page.
- **Code spans.** `code`/`kbd`/`samp` inherit the Poppins body token — the
  system has no monospace face, and the browser's monospace default is not
  part of the S&F type ramp.
