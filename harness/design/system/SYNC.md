# S&F design-system mirror — sync record

- source project id: 147f5da0-fedd-4aaa-b0c2-ccb9f7d7b41e
- source project: claude.ai/design "S&F Design System"
- sync date: 2026-08-01
- synced by: the DesignSync tool (per-file `get_file`)

## Layout

- `tokens.css` — the single CSS entrypoint consumers link. Normalized by the
  orchestrator from the source project's split token files (`tokens/*.css`),
  with `@font-face` re-based to `./fonts/` and gradient image URLs re-based to
  `./assets/gradients/`. Values are verbatim from the source.
- `workspace.css` — REPO-OWNED workspace density layer (`--wk-*` type scale +
  frame dimensions), promoted from the proto-workspace skin 2026-08-04. NOT
  part of the S&F sync — never overwrite it during a re-sync. Consumers link
  it immediately AFTER `tokens.css`.
- `tokens/` + `styles.css` — the source project's own split layout, as pulled
  (font URLs re-based from `../assets/fonts/` to `../fonts/` to match this
  mirror's layout).
- `fonts/` — Poppins Medium (500) + Bold (700) TTFs, verbatim binaries.
- `assets/gradients/` — see Deviations.
- `components/` — S&F primitives as pulled (`brand/`, `core/`, `forms/`,
  `layout/`; `.jsx` + `.d.ts` + `.prompt.md` + `*.card.html` per group).
- `guidelines/` — the 23 foundation specimen cards (Type / Colors / Spacing /
  Brand).
- `readme.md` — the source project's full rulebook (visual foundations, voice,
  iconography policy, component index). Read this before styling anything.
- `SKILL.md` — the source project's agent-skill front matter.
- `MIRROR-NOTES.md` — the detailed pull log and deviation rationale.

## Deviations from the source project

1. `assets/gradients/gradient-field-radial.png` and
   `assets/gradients/gradient-field-soft.png` are local 2000×1125 re-renders of
   the system's own CSS fallback gradients (`--gradient-spiral`,
   `--gradient-spiral-soft`), NOT the original 4000×2250 PDF-extracted artwork —
   the originals exceed the DesignSync per-file transfer cap (256 KiB). All
   `url()` references resolve and render offline; treat the fields as
   approximations and re-sync the real artwork when a direct export is possible.
2. Not mirrored (out of scope for the harness): `templates/`, `ui_kits/`,
   `uploads/`, `scraps/`, `thumbnail.html`, `.thumbnail`, and generated build
   artifacts (`_ds_bundle.js`, `_ds_manifest.json`, `_adherence.oxlintrc.json`).
   Note: the `components/**/*.card.html` previews reference `_ds_bundle.js` and
   CDN React, so they render only inside the source project — they are kept
   here as readable JSX usage references, not as offline-renderable pages.
   Render-time offline guarantees apply to CSS (`tokens.css`, `tokens/`,
   `styles.css`), fonts, and gradient assets.
