# S&F design-system mirror — provenance

- **Source**: claude.ai/design project "S&F Design System",
  projectId `147f5da0-fedd-4aaa-b0c2-ccb9f7d7b41e`.
- **Synced**: 2026-08-01, via the DesignSync tool (per-file `get_file`).
- **Scope mirrored**: `styles.css`, `tokens/` (7 files), `readme.md`, `SKILL.md`,
  `guidelines/` (23 specimen files), `components/` (brand / core / forms /
  layout — 44 files), `assets/fonts/` (Poppins Medium + Bold TTF, verbatim).

## Deviations from the source project

1. **`assets/gradients/gradient-field-radial.png` and
   `assets/gradients/gradient-field-soft.png` are NOT the original artwork.**
   The originals (4000×2250, extracted from the brand PDF) exceed the
   DesignSync per-file transfer cap (256 KiB) and could not be mirrored
   verbatim. The files here are local 2000×1125 re-renders of the design
   system's own CSS fallback definitions (`--gradient-spiral`,
   `--gradient-spiral-soft` in `tokens/effects.css`, blurred per
   `--blur-field`). They keep every `url(...)` reference and the
   `GradientField` component working offline, but treat them as
   approximations — re-sync the real artwork if a direct export becomes
   available.
2. **Not mirrored (deliberately out of scope)**: `templates/` (presentation /
   one-pager), `ui_kits/website/`, `uploads/` (source PDFs + duplicate font
   files), `scraps/`, `thumbnail.html`, `.thumbnail`, and the generated build
   artifacts `_ds_bundle.js`, `_ds_manifest.json`, `_adherence.oxlintrc.json`.
   The harness needs the rules, tokens, primitives, and assets — not the
   marketing-site kit or deck templates. The `*.card.html` preview files
   reference `_ds_bundle.js` and CDN React, so they render only inside the
   claude.ai project; they are kept as readable JSX usage references.
