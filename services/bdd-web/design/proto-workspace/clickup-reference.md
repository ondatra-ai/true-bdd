# ClickUp behavior reference (live-researched)

**Standing rule:** ClickUp is the DEFAULT UX reference for the TrueBDD workspace
prototype. Any interaction not explicitly steered otherwise behaves like ClickUp,
adapted to our B&W design language (tokens.css: square corners, no drop shadows,
depth via value contrast) and our features. Before building a NEW interaction
pattern: check this file; if the pattern is missing, the orchestrator researches
it live in ClickUp (screenshots + computed styles) and appends here first.

Each entry: verified live in the user's ClickUp (Speed & Function workspace),
date, evidence.

---

## 1. Sidebar tree rows (Spaces tree) — 2026-08-02

- REST: no chevrons anywhere; icon + name per row; children of an expanded item
  indent one level with a thin vertical guide line along the child block.
- HOVER: row gets light-gray rounded hover bg; the ICON SLOT swaps to a caret —
  ▸ collapsed, ▾ expanded; trailing quick actions (…, +) fade in at row end.
- CLICK: caret toggles expand/collapse; the NAME navigates to the item's page.
  Both live on one row (split-click).
- SELECTED row keeps a persistent highlight bg.
- Evidence: tmp/proto-clickup-sidebar.png, proto-clickup-hover.png,
  proto-clickup-hover-expanded.png.

## 2. Left icon rail + section flyouts — 2026-08-02

- Narrow dark far-left rail; stacked items = small icon + tiny uppercase label;
  active section highlighted; utility items pinned at rail bottom.
- HOVER on a non-active rail item → flyout panel immediately right of the rail,
  floating OVER content (ClickUp uses rounded corners + shadow; we use heavy
  border per our no-shadow rule), showing that section's whole nav tree;
  closes on mouse-out (needs open/close delay ~150ms to avoid flicker).
- CLICK on a rail item → that section becomes the DOCKED sidebar + navigates.
- Evidence: tmp/proto-clickup-rail-home.png.

## 3. Brain chat panel — docks in-layout, NOT a popup — 2026-08-03

- Clicking Brain² docks a chat panel INTO the app layout on the right, below
  the top bar: the main content area NARROWS and visibly reflows (card grids
  rewrap). Not an overlay: no shadow, no floating; a plain vertical divider.
- Panel anatomy: own header row (new-chat control, icons), chat body, input
  pinned at the panel's bottom.
- Width: wide (~40% of window at 1920); divider likely draggable (not verified).
- Evidence: tmp/proto-clickup-brain-before.png / -after.png.

## 4. Edit-in-place (task description, ticket view) — 2026-08-03

- REST: description renders as PLAIN DOCUMENT TEXT on the page background —
  no field chrome, no border, no background tint, indistinguishable from
  read-only text.
- CLICK → EDIT MODE: caret lands in place inside the same element
  (contenteditable ql-editor). Computed styles WHILE EDITING:
  background rgba(0,0,0,0) (fully transparent — NO gray), border none,
  outline none, box-shadow none. The text does not move or restyle AT ALL.
- The only edit-mode signals: the caret itself + a floating rich-text toolbar
  appearing (cu-rich-editor-toolbar).
- Adaptation for us: editable surfaces (YAML file view, inline fields) must not
  change background color on focus; caret (+ optional subtle affordance like
  our line-flash) is the signal.
- Evidence: tmp/proto-cu-task-rest.png, proto-cu-task-edit.png, computed-style
  probe in conversation log.
