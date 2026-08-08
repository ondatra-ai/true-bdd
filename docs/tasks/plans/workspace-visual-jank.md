# Plan — workspace-visual-jank (easy lane, tests-only)

## Goal
Pin the 5 workspace visual defects from `tmp/visual-walk/report.md` as e2e assertions,
left RED against the current build. ONE new spec + one contract testid; no production edits.

## Test file to create
- `tests/harness/w20-visual-stability.spec.ts` (workspace project, 1440×900).
- Contract addition: `sidebar-group-empty` testid → `helpers/ui.ts` (WTID) + `helpers/README-testids.md`.
- No startup scaffolding needed — the harness image/CLI/workspace UI already boot.

## Cases (exact assertions; each RED for the stated reason)
- **w20.1 (P1)** Injected `PerformanceObserver('layout-shift')`; goto `/home`; wait meta→realpath.
  Assert zero non-input shifts whose source is an `overview-*` node with prevY≠currY. RED: inventory 215→254 (Δ39).
- **w20.2 (P2)** Open chat at 1440×900; `chat-dock-resizer` visible; `|panelWidth−380|≤2`; contentPane>panel. RED: 768.
- **w20.3 (P3)** Hover Product flyout over docked sidebar; `|Δx|≤1`; `|Δwidth|≤1`; height≥sidebar−1. RED: flyout 260 vs sidebar 280.
- **w20.4 (P4)** Seed `architecture:`-wrapped+`vocabulary:` arch.yaml on disk; 0 rows, no invalid indicator;
  each fixed group (Services/Terms/Docker) shows a non-empty `sidebar-group-empty`. RED: testid absent.
- **w20.4b (P4 guard)** Happy fixture: populated groups render NO `sidebar-group-empty` (count 0). GREEN now, must stay green.
- **w20.5 (P5)** Injected observer; goto `/architecture`; assert zero non-input shifts with t≤3000ms. RED: font-swap ~305ms.

## Codex rounds
See `docs/tasks/plans/workspace-visual-jank.codex.md` (cap 1).

## Challenges

- **w7.1a wide-chat conflict (fixer blocker, Codex verdict: STOP — test must
  change).** w7.1a asserted panel width > 1920×0.3 (576px, from the ClickUp
  "~40% of window" measurement); w20.2 pins the prototype's committed
  DEFAULT_WIDTH=380. Contradictory defaults — no code satisfies both. User
  delegated the call ("decide yourself"); orchestrator decision: **380 wins**
  (the prototype is the committed design baseline per paths.yaml
  design_system; w7.1a predates it; 380 keeps content usable at laptop
  widths — the original finding). Test-author reworked w7.1a's width
  assertion to 380±2; all other w7 assertions kept.
- **Fixer declined the 1-round override** (its agent rules treat
  prompt-delivered process overrides as unverifiable) and ran its fixed 3
  rounds (0 keeps). The user's cap was honored by the author (1 round) and
  will be honored by the reviewer (1 round, result-focused).
- **Unasserted defensive behavior flagged by the fixer:** home/page.tsx
  resolveFolder() swallows relayHub() errors to null — routed to the
  reviewer for durable-coverage-or-removal judgment.

## Workflow log

- 2026-08-05 ~17:40 orchestrator: lane easy declared; user process override:
  author ≤1 Codex round, fixer 1 round (fixer declined — see Challenges),
  reviewer result-focused. Baselines captured. Author: 1 round, 5 red + 1
  guard green, contract addition sidebar-group-empty. Scope checks CLEAN.
- 2026-08-05 ~18:20 orchestrator: fixer green (w20 6/6, unit 84/84, tsc 0);
  off-limits + scripts CLEAN. w7.1a conflict surfaced → user delegated →
  380px decision → author reworking w7.1a.
- 2026-08-05 ~19:10 orchestrator: w11 judge red in isolation after home rewrite
  (screenshot caught the reserved-space "…" placeholder chips) → author added
  the settled-canvas wait → green. Reviewer PASS: live 6/6, 1 Codex round
  (2 keeps, both hardening the new test code, applied); escalation valve
  (2 keeps on an easy round) NOT taken — user's explicit 1-round speed
  directive supersedes; recorded here. Retro SKIPPED per the same directive.
  Orchestrator final run: tsc clean, w7+w11+w20 all green.
