# Plan — design-conformance-tests (easy lane, tests-only)

## Goal
Prove the harness workspace UI matches its design source (`harness/design/` tokens
+ mockups) via NEW Playwright specs in `tests/harness/` (workspace `w*` project).
No production changes; specs land RED where the app deviates from the design.

## E2E cases (files → assertions)
- `w8-design-tokens.spec.ts` (R1, deterministic, NEVER skips) — render the prod
  architecture page; sweep every visible element.
  - w8.1 colors: computed `color`/`background-color`/visible-border-colors ∈ the
    rgb set resolved from `harness/design/system/tokens.css` (+ transparent).
    Fails naming the offending `selector`/`property`/`value`.
  - w8.2 typography: every visible text element's first font-family === `poppins`
    EXCEPT `file-view` descendants (monospace exception); AND
    `document.fonts.check('700 32px "Poppins"')` — the design's face truly renders.
- `w9-design-judge.spec.ts` (R2, skips with named reason when `codex` absent) —
  screenshot mockup `workspace-overview.html` + the prod architecture page at
  1440×900, submit both to `codex exec -i … --output-schema … -o verdict.json`
  with named checks (persistent sidebar/breadcrumb/canvas frame, sidebar fixed
  width, breadcrumb hairline border, canvas padding); assert `verdict === "pass"`.
  Content/data differences must not fail. RED: prod has no breadcrumb frame.

## Files to touch (all NEW)
- `tests/harness/w8-design-tokens.spec.ts`, `tests/harness/w9-design-judge.spec.ts`
- `tests/harness/helpers/design-conformance.ts` (token parse, sweeps, judge runner)
- `tests/harness/tsconfig.json` (typecheck-gate scaffolding) + typescript/@types/node devDeps
- Codex rounds: see `docs/tasks/plans/design-conformance-tests.codex.md`

## Result (RED, verified)
All 3 assertions RED (no crashes). tsc --noEmit green. Genuine deviations:
- w8.1 `chat-dock-toggle` color `rgb(0,0,0)` (not token near-black).
- w8.2 `rail-utility-item` + `chat-dock-toggle` `font-family: arial` (not Poppins).
- w9.1 codex verdict `fail`: `persistent_frame` + `breadcrumb_hairline` (no prod
  breadcrumb bar); `sidebar_fixed_width` + `canvas_padding` born green.

- 2026-08-04T11:02Z orchestrator: fixer greened w8/w9 (verified independently); reviewer escalated easy→hard cap after round 1 kept multiple, applied 7 keeps (w8.3, w8.4, w9 hardening, testid docs), live smoke PASS; final gate 5 passed confirmed by orchestrator. Fixer required 4 resume nudges (ended turns while background codex/test runs were live) — retro item.
