# proto-workspace — the file-as-source workspace prototype

Throwaway prototype (2026-08-02/03), preserved on user request instead of being
reverted. Code quality is deliberately prototype-grade (speed over quality) —
treat it as a living design reference, not a starting codebase.

## What it demonstrates

- ClickUp-patterned workspace shell: left icon rail (Home / Architecture /
  Product / Builds), hover flyout panels, docked per-section sidebar.
- Sidebar rows with split click targets: hover-revealed caret (▸/▾) toggles,
  name navigates; expand/collapse state survives navigation (persistent
  Next.js layout + native `<details>`).
- File-as-source pages: architecture.yaml and the product docs (prd.yaml,
  story files, features.yaml, scenarios.yaml) as GitHub-style file views with
  seamless edit-in-place (no chrome change on focus) and line-accurate outline
  jumps.
- Docked chat (ClickUp-Brain style: narrows content, resizable divider) with
  scripted mock LLM edits that mutate the YAML files.
- Features as tags: minimal features.yaml (id + description), `feature:`
  references on stories/scenarios, live aggregation pages, searchable
  picker with create-new, "unaligned requirements" retro-tagging bucket.

## Run it

```bash
cd services/bdd-web/design/proto-workspace/app
npm install
npm run dev -- -p 3999   # then open http://localhost:3999
```

All state is in-memory (reload resets the files to their seeds).

## Companion research

`clickup-reference.md` — ClickUp interaction patterns measured live (sidebar
tree, rail/flyouts, Brain docking, edit-in-place computed styles). ClickUp is
the default UX reference for the workspace, per that document.
