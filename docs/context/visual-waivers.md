# Visual waivers — cross-run memory for the visual-sweep skill

Findings the user accepted as non-goals, plus surfaces already verified clean.
The visual-sweep skill dedupes every new finding against this file before
flagging it; nothing listed here may be re-flagged or "fixed". Entries are
added only with explicit user approval (the skill proposes, the user decides).

Fingerprints follow the visual-sweep format: `<element-signature>/<symptom>`.

## Waived — accepted behavior, do not flag

| Fingerprint | What it looks like | Why it's waived |
|---|---|---|
| `[content-breadcrumb]/churn` | Breadcrumb trail model differs across sections (2-crumb "Home /" vs 3-crumb "Sessions /"); file card sits at different heights per section | Deliberate model, pinned as-is by w13/w16 (workspace-visual-jank non-goal, 2026-08-05) |
| `[file-view-editor]/sliver` | Long editor lines clip at the card edge with no visible scroll affordance (macOS overlay scrollbars) | Cosmetic; accepted as non-goal (visual-walk finding 7, 2026-08-05) |
| `[chat-dock-panel]/squeeze` | Chat dock behavior when resized beyond its 380px default | Only the default width is specified (w20.2); resize behavior is out of scope (workspace-visual-jank non-goal) |

## Verified clean — not suspects, but still fair game if a probe fires

| Surface | Verified by | Date |
|---|---|---|
| Sidebar hover geometry (group headers, service/term/docker rows, brand) | hover-probe: 0 jiggles, 0 layout shifts, 0 mutations across all targets | 2026-08-05 |
| Section-switch chrome stability (rail / sidebar / breadcrumb / content-pane boxes) | walk2: 0 box moves across 2 full cycles + 6 rapid flips | 2026-08-05 |
| Save cycle (idle → saving → saved on edit and revert) | walk: no accompanying layout movement | 2026-08-05 |
| Sidebar collapse/expand | walk: symmetric input-driven shifts only | 2026-08-05 |
