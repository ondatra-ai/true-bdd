# Workspace overview design parity

## Goal

The production workspace-overview page (`/sessions/<sid>/home`) must match its
design mockup (`harness/design/mockups/workspace-overview.html`) in structure
and content surfaces — verified by e2e tests (deterministic + codex vision
judge), iterated red → green until the visual comparison holds.

## Constraints

- The existing shell IA (icon rail + secondary sidebar) is BINDING per the
  w1–w7 contract and stays; parity applies to the breadcrumb trail and the
  canvas content, not the navigation chrome. Judge checks must tolerate the
  rail (as the existing w9 architecture-pair checks already do).
- All data comes from the existing browser API surface (`SessionDetail`:
  status + inventory; run-dispatch routes) — no new engine endpoints.
- Existing w1–w9 and a10 specs must stay green.

## Requirements

- **R1 — session metadata.** Under the "Workspace overview" title the page
  must show the session's folder path and session id (as the mockup does
  beneath its title).
- **R2 — build actions.** The overview canvas must offer the run-dispatch
  actions the mockup shows — Build Tests, Build Code, and Refresh (inventory)
  — wired to the existing dispatch/read routes, disabled per the existing
  own-active-run rule.
- **R3 — inventory health.** The overview must render an inventory-health
  list from `SessionDetail.inventory`: each entry with its path, kind label,
  and a status chip (present / present-empty / missing / not-a-directory /
  ambiguous — the statuses the mockup's chips show), styled from the design
  system.
- **R4 — degraded-state banners.** When the session detail reports them, the
  overview must show the mockup's flagged states (e.g. inventory truncated,
  sibling folder conflict) as bordered banners; absent states render nothing.
- **R5 — breadcrumb trail.** The overview breadcrumb must be a two-level
  trail — a "Sessions" link (to `/`) followed by the current "Workspace
  overview" crumb — matching the mockup's `Sessions / Workspace overview`.
- **R6 — judge pair.** The design-judge suite must gain a workspace-overview
  pair: mockup `workspace-overview.html` vs prod `/home`, screenshot at the
  same viewport, codex vision verdict with named checks covering the canvas
  parity surfaces (title+metadata block, actions row, inventory-health list
  with chips, breadcrumb trail) — content values ignored, structure asserted.
  Red while any surface is missing; the permanent gate thereafter.
- **R7 — red first.** Every new spec must fail as an assertion against the
  current stub Home page before any production change.
