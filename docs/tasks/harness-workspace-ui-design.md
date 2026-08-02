<!-- See docs/context/requirements-guide.md. System = architecture/infrastructure
     decisions only ("must use X"); every behavior → Product, from a role. -->
# Harness workspace UI design (ClickUp-style, S&F design system)

## Goal:

Design — as reviewable local HTML mockups, not yet implementation — a new
ClickUp-inspired workspace UI for the true-bdd harness: a left sidebar with
three hierarchical document sections (Architecture, Product,
Requirements/Scenarios) plus the harness's operational surfaces, skinned with
the S&F design system.

## Current behavior:

The harness UI (currently gutted in `harness/`; the e2e suite under
`tests/harness/` is the contract) is a flat three-view app: a sessions list
(`/`), a session detail page (`/sessions/<sid>`) with inventory health chips,
an epic accordion of story rows, per-story actions, and run history, and a
session-scoped run view. Story details open in a modal dialog; CLI prompts
open as native dialog modals. There is no persistent navigation frame, no
hierarchical sidebar, and no per-document pages for architecture, PRD, or the
scenario registry.

Design references reviewed via Playwright: ClickUp (two-level sidebar with
hierarchical tree Spaces → Folders → Lists, grouped list views with status
pills, full-page task detail with properties grid + subtasks + activity
panel) and Lovable (minimal light sidebar, muted section labels, rounded
floating canvas). Visual language decided: the S&F design system
(claude.ai/design project `147f5da0-fedd-4aaa-b0c2-ccb9f7d7b41e` — tokens,
Poppins type, gradient fields, core components).

## Requirement

### Harness

- [revealed] A Developer should have the S&F design system saved locally
  under `harness/design/` (tokens, components, guidelines, fonts/assets
  working offline) so future harness work can reference it without reaching
  claude.ai, with the mirror recording its source project id and sync date.
- [revealed] A Developer should be able to review the proposed workspace UI
  as static local HTML mockups — plain HTML/CSS with minimal inline JS,
  viewable without a server or build step — before any harness
  implementation happens.
- [suggested] A Developer should be able to click through the mockup pages
  along the representative navigation flows (sidebar → section → epic →
  story; scenario list → scenario; runs → run) — links between static pages
  only, no app logic, API, or real routing.
- [suggested] A Developer should find mockup pages covering at least: the
  sessions list, the workspace overview (inventory health + build actions),
  the PRD overview page, an epic page, a story detail page, the flat
  scenario list, a scenario detail page, an architecture/service page, the
  vocabulary page, the runs list, a run detail page, and all three CLI
  prompt dialog kinds (choice, numbered clarify, multiline freetext).
- [suggested] A Developer should find, within the mockup pages, the
  representative degraded and flagged states the current contract renders —
  inventory chip states and the truncation banner, the `504 cli_timeout`
  unavailable state (removed sessions are never shown as disconnected rows —
  they simply disappear), the folder-conflict and
  architecture-path-mismatch warnings, epic
  and story identity flags, an ambiguous story page (match count notice), an
  invalid story page (parse error), a run detail with a non-success outcome,
  and the lifecycle chip value vocabulary (created / applied "x/y" or
  unknown / refined not recorded) — not just happy paths.
- [suggested] A Developer should be able to determine, from a short design
  spec alongside the mockups, the layout frame (sidebar, breadcrumb, content
  canvas), the sidebar's top-level ordering (workspace overview, the three
  document sections, Runs, return to sessions), and how S&F tokens map to
  the UI parts (tree, status pills, chips, tables, dialogs); mockups target
  a 1440px-wide desktop reference viewport.

### System

- [revealed] The true-bdd harness must use the S&F design system (the
  claude.ai/design project, mirrored under `harness/design/`) as its visual
  language.

### Product

- [revealed] A BDD System Architect should land on a sessions list — each
  row showing the session's canonical folder, CLI version, and a
  test-connection control — and, by opening a connected session, enter a
  workspace whose left sidebar shows three document sections: Architecture,
  Product, and Requirements/Scenarios.
- [revealed] A BDD System Architect should be able to expand each sidebar
  section in place as a hierarchical tree — Product expands to epics and each
  epic expands to its user stories.
- [suggested] A BDD System Architect should see the Architecture section
  expand following the shape of `architecture.yaml` — one node per declared
  service opening a service page, plus a vocabulary node opening a
  vocabulary page.
- [suggested] A BDD Product Owner should see the Product section root open
  as a PRD overview page — title, summary, and personas from `prd.yaml`.
- [revealed] A BDD Product Owner should see the Requirements/Scenarios
  section as a flat list of scenario ids mirroring the structure of the
  `scenarios.yaml` registry, with each scenario's service and linked stories
  shown on its row or detail page rather than as tree levels.
- [revealed] A BDD Product Owner should open a user story as a full detail
  page in the main content area — breadcrumb, title, status and lifecycle
  chips, the as-a / i-want / so-that statement, acceptance criteria with
  their scenario steps, and access to the raw file view — replacing the
  current story modal.
- [suggested] A BDD Product Owner should open an epic as a page listing
  its stories as a ClickUp-style grouped table (status pills; created /
  applied / refined lifecycle columns; identity flags).
- [suggested] A BDD Product Owner should open a scenario as a detail page
  showing its merged given/when/then steps, its service, and links to the
  user stories it was merged from.
- [suggested] A BDD System Architect should see a breadcrumb trail above the
  content area reflecting the current tree path (session / section / epic /
  story), each crumb navigable.
- [suggested] A BDD System Architect should reach the harness's operational
  surfaces from the same workspace frame: a separate Runs sidebar section
  listing run history, with every run link staying session-scoped; per-story
  actions (Create / Refine / Apply and the fix toggle) on the story detail
  page; workspace-level build actions (build tests / build code), inventory
  health, and an inventory refresh control on the workspace overview; CLI
  prompts staying as modal dialogs over any page.

## Non-goals

- No harness production code in this task — the deliverable is the design
  (mockups + spec + local design-system mirror); implementation is a
  follow-up task.
- The design deliberately departs from the current e2e UI contract where the
  new structure requires it (story modal → full detail page; per-row actions
  → story page); migrating `tests/harness/` to the new contract belongs to
  the implementation task, not this one.
- No redesign of the CLI's terminal output or the BDD fixture harness.
