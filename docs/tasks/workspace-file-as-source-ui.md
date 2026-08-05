<!-- See docs/context/requirements-guide.md. System = architecture/infrastructure
     decisions only ("must use X"); every behavior → Product, from a role. -->
# Workspace file-as-source UI

## Goal:

Build the production TrueBDD harness workspace as a ClickUp-patterned,
file-as-source web app: fixed-schema navigation over the host project's real
YAML documents (architecture spec + product docs), edited seamlessly in place
or through a docked chat with all changes persisted through the bdd-cli, and
features as the tag aligning stories and scenarios.

(Scope was discovered via a prototyping session on 2026-08-02/03; the throwaway
prototype is preserved at `harness/design/proto-workspace/` as a design
reference only. These requirements describe the production system.)

**Terminology:** a *scenario* — one record in the scenarios registry
(`docs/scenarios.yaml`) — IS the requirement entity of the workspace; feature
references on the requirements side live on the scenario record. Where the UI
says "Requirements", it lists scenarios.

## Current behavior:

The harness workspace exists only as 17 static HTML design mockups
(`harness/design/mockups/`, exercised by the m1–m6 Playwright suite). The
sidebar is duplicated per page and flat (01—Architecture … 04—Runs), epics
exist as a grouping level, pages are read-only form-style views, there is no
chat, no file views, and no interactive app.

## Requirement

### Harness

(none)

### System

- **S1 [revealed]** The true-bdd harness must use the true-bdd CLI as its
  backend for persisting document changes — committed changes to the workspace
  YAML files (from direct edits and chat-mediated edits alike) go through
  bdd-cli.
- **S2 [suggested]** The true-bdd harness must use Next.js (App Router) for the
  workspace UI.

### Product

(Both roles have identical workspace access; each requirement names the
primary role for readability only.)

Workspace shell & navigation:

- **P1 [revealed]** A BDD System Architect should switch workspace sections —
  Home (the workspace overview), Architecture, Product, Builds — from a narrow
  left icon rail, where clicking a rail item docks that section's navigation
  panel and the active section stays visibly marked.
- **P2 [revealed]** A BDD System Architect should preview any non-active
  section's full navigation tree in a flyout panel by hovering its rail item;
  clicking an entry inside the flyout navigates there and docks that section.
- **P3 [revealed]** A BDD System Architect should find requirements/scenarios
  nested inside the Product section, and build runs under a separate Builds
  section.
- **P4 [revealed]** A BDD System Architect should toggle any sidebar group via
  a caret revealed on row hover (▸ collapsed, ▾ expanded), while clicking the
  same row's name opens that row's own page — two independent click targets on
  one row.
- **P5 [revealed]** A BDD System Architect should keep the sidebar's
  expand/collapse state while navigating between workspace pages.
- **P6 [suggested]** A BDD System Architect should see the sidebar row of the
  currently open page persistently highlighted.
- **P7 [revealed]** A BDD System Architect should find workspace interactions
  behaving like their ClickUp counterparts by default, adapted to the workspace
  design language, unless a requirement states otherwise — "like ClickUp" means
  the concrete measured behaviors recorded per-pattern in
  `harness/design/proto-workspace/clickup-reference.md`, extended there when a
  new interaction is designed.

File-as-source:

- **P8 [revealed]** A BDD System Architect should see the architecture spec as
  one single YAML-file page in a GitHub-style file view (file-path header,
  line-number gutter) rather than per-entity form pages.
- **P9 [revealed]** A BDD System Architect should edit any workspace document —
  the architecture spec, the PRD, story files, features.yaml, and the scenarios
  registry — directly in its file view as plain text, with outline entries and
  derived views updating live as the content changes.
- **P10 [revealed]** A BDD System Architect should request file changes through
  a chat dialog available on every leaf page — one workspace-wide conversation
  that follows navigation, whose edits target the current page's file and
  appear in the file view immediately; on pages without a single backing file
  (feature aggregations, Home) the chat converses but performs no file edit.
- **P10a [revealed]** A BDD System Architect's edits must autosave: changes
  persist to the host project's actual document files continuously (debounced,
  no save action — the ClickUp model) and survive page reload and harness
  restart; content that does not parse as YAML is not committed until it
  parses again.
- **P10b [suggested]** A BDD System Architect should see an indication when a
  file's content is not valid YAML, while the outline and derived views keep
  the last valid state instead of breaking.
- **P11 [revealed]** A BDD System Architect should get the chat as a panel
  docked into the layout that narrows the main content when open (never a
  popup overlay), collapsible to a thin edge tab.
- **P12 [suggested]** A BDD System Architect should resize the docked chat by
  dragging its divider.
- **P13 [revealed]** A BDD System Architect should see a fixed mandatory
  outline of the architecture file in the sidebar — Services (one entry per
  service), Terms (flat, no internal hierarchy), Docker (the compose file
  path) — whose entries update live as the file content changes, whether by
  hand or by chat.
- **P14 [revealed]** A BDD System Architect should jump to the exact line of a
  key by clicking its outline entry, including across pages (navigate first,
  then scroll).
- **P15 [revealed]** A BDD System Architect should see, for each service, its
  own tech stack and its Docker provenance — a Dockerfile path when the
  service is custom, a reference to its docker-compose section when it runs
  from an image — with the compose file path listed separately.
- **P16 [revealed]** A BDD System Architect should declare optional endpoints
  (method, path, summary) on custom services, while supporting services
  (databases etc.) carry connection info instead.
- **P17 [revealed]** A BDD System Architect should enter edit mode on any
  editable surface with no visual change beyond the caret — no background
  tint, border shift, or text movement (ClickUp edit-in-place).

Product docs & features:

- **P18 [revealed]** A BDD Product Owner should see the product docs as file
  pages too — the PRD, one file per story, and the scenarios registry — in the
  same GitHub-style file view as the architecture spec.
- **P18a [revealed]** A BDD Product Owner should navigate Product via a fixed
  outline — PRD, Features, Stories (flat), Scenarios — with entries derived
  live from the files; story and scenario entries jump to the exact line in
  their file page (same mechanics as Architecture), the Features section
  header opens features.yaml's file page, and each feature entry opens that
  feature's aggregation page.
- **P19 [revealed]** A BDD Product Owner should work without epics: stories
  are flat, standalone files with no epic grouping anywhere in the workspace.
- **P20 [revealed]** A BDD Product Owner should align work via features: each
  feature record in features.yaml has exactly two fields — `id` (a slug, also
  the display name) and `description` — while stories and scenarios each carry
  a `feature: <id>` reference; nothing else lives in features.yaml.
- **P21 [revealed]** A BDD Product Owner should see each feature aggregated on
  its own page — description plus the user stories and requirements
  referencing it — derived live from the references and updating when any
  reference changes (by hand, picker, or chat).
- **P22 [revealed]** A BDD Product Owner should create a story only with a
  feature: picking an existing feature from a searchable list or creating a
  new one inline (which appends the minimal id+description stub to
  features.yaml).
- **P23 [revealed]** A BDD Product Owner should change the feature of any
  story or scenario from the UI via the same searchable picker, with the
  change written into the underlying YAML file.
- **P24 [revealed]** A BDD Product Owner should see scenarios not yet aligned
  to any feature in a visible "unaligned" bucket and retro-tag them from
  there.
- **P25 [suggested]** A BDD Product Owner should see stories and scenarios
  whose feature reference matches no feature in features.yaml treated as
  unaligned — a dangling reference surfaces visibly instead of silently
  breaking (covers feature deletion/rename fallout).

## Open questions (explicitly NOT requirements)

- Whether docker-compose.yml should be generatable from the architecture spec —
  the user is explicitly undecided.
- The sub-hierarchy of each service in the outline — "we'll identify later";
  the outline shows one entry per service for now.
- Whether prd.yaml's stories index stays in sync automatically when stories are
  created.
- Conflict policy when a chat edit lands while direct typing is in flight
  (autosave makes the window small but nonzero).
- Feature rename propagation (does renaming a slug rewrite references, or do
  they surface as unaligned per P25?).
- Error/empty states beyond invalid YAML (missing/unreadable files, CLI write
  failures) — production needs them; shapes not yet designed.
- Builds section content: this task treats Builds as navigation-level only
  (hosts the existing runs pages); its own file-as-source model is a future
  task.

## Established facts

- ClickUp patterns live-measured in the user's workspace, recorded in
  `harness/design/proto-workspace/clickup-reference.md`: (1) sidebar tree — no carets at rest, icon
  slot swaps to ▸/▾ on hover, caret toggles / name navigates, indent + thin
  guide line, selected row highlighted; (2) icon rail — hover opens a flyout
  panel with the section tree, click docks it; (3) Brain chat — docks
  IN-LAYOUT, main content narrows and reflows (~40% width at 1920), plain
  divider, no shadow; (4) edit-in-place — computed background stays
  rgba(0,0,0,0) focused and unfocused, no border/outline/shadow change, only
  caret + floating toolbar.
- `harness/design/mockups/` — 17 static pages; sidebar markup duplicated
  verbatim per page; design system in `assets/tokens.css` + `mockups.css`:
  square corners, NO drop shadows ("depth comes from value contrast"),
  Poppins-only including code — file views need a scoped monospace exception.
- Next.js proof: components in a persistent route-group layout survive
  client-side navigation — native `<details>` kept open/closed state across
  pages with zero state code; `next/link` needs `scroll={false}` wherever a
  custom scroll jump runs post-navigation (default scroll reset races it).
- App-shell scrolling is required: cap the frame at 100vh and scroll the
  content pane. With a scrolling body, right-edge-docked elements sit under
  the body scrollbar and become unclickable — invisible in screenshots, found
  only by click-testing.
- CSS gotchas hit during the prototype: `overflow-x: hidden` forces computed
  `overflow-y: auto` (CSS Overflow spec); `input:not([type])` has specificity
  (0,1,1) and beats a plain class selector.
- Real host-project file shapes to mirror:
  `tests/bdd-cli/fixtures/us-create-happy-path/input/docs/prd/prd.yaml`
  (title/personas/stories shape); story files as `story:` maps with
  `acceptance_criteria`; scenarios registry keyed by scenario id.
- Line-accurate outline jumps: compute the target line from the live YAML text
  per key on every change; clamp at scroll extremes.
- Validated interaction mechanics: `event.stopPropagation()` on the name link
  inside `<summary>` gives split click targets on native details rows.
