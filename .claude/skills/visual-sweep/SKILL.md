---
name: visual-sweep
description: EXPERIMENTAL exploratory visual-QA loop for a live web UI — hunts jangling/jiggle/jank/layout shifts/flicker and animation glitches by exploring the running app generically (DOM-discovered hover enumeration over every interactive element, fast cursor transit sweeps across nav-like regions, route cycles, cold-cache font loads, random cursor walks), instrumented with a layout-shift observer, a bounding-box jiggle sampler, an overlay-flash sampler, and a mutation observer. Each round pins every NEW finding as a RED e2e spec via the visual-sweep-test-author agent (a crush driver — crush/GLM-5.2-1M writes the specs; ONE crush session, explicitly NO Codex), then greens it via the task-blind visual-sweep-test-fixer agent (a crush driver — crush writes the code; ONE crush session, explicitly NO Codex), and repeats until a round finds nothing new (hard cap 5 rounds), finishing with one full regression run. Deliberately lighter than implement-task — no lanes, no phase gates, no critique loops; iteration speed is the point and the user reviews the final report + live UI, not code. Use when asked to "sweep the UI for jank", "hunt visual bugs", "exploratory-test the visuals", "check hovers/animations", "does anything jiggle/flash", or as a jank patrol after UI work lands. Dedupes against docs/context/visual-waivers.md, already-pinned specs, and the run ledger; artifacts under tmp/visual-sweep/<ts>/.
---

# Visual sweep

An experimental explore → pin → fix loop for visual quality. The e2e suite
stays the spec: no production code changes without a RED spec pinning the
defect first — the loop is red → green per finding, just without Codex. ALL
file writing in PIN and FIX happens inside **crush** (`zhipu-coding/glm-5.2`,
the 1M-context model, pinned in `.crush.json` — crush displays it as
"glm-5.2[1m]" but the config id is plain `glm-5.2`); the two Claude agents
are drivers/verifiers with no write tools.

Read `docs/context/paths.yaml` first; take `visual_waivers`,
`visual_sweep_artifacts`, `visual_probe_scripts`, `e2e_tests`,
`harness_code_root`, `design_system`, `crush_wrapper`, `crush_artifacts`, and
run commands from it. Then read this
skill's two references: `references/methodology.md` (HOW to explore — generic,
normative) and `references/target-harness.md` (the target adapter: launch,
entry URLs, pin/fix conventions for THIS repo's app; swap this file to point
the skill at another app).

## Ground rules

- **You (the orchestrator) explore, triage, and verify INLINE** — exploration
  is the judgment-heavy part; never delegate it. Only pinning and fixing go to
  agents.
- **NO Codex anywhere in this skill.** ONE authoring round, ONE fix pass per
  iteration — each is a single crush session with a small bounded follow-up
  budget (author 3, fixer 2).
- **Crush writes; agents drive.** The author's specs and the fixer's code are
  written by crush through the `crush_wrapper` script; the role passed to the
  wrapper selects the write-guard sandbox (author → the e2e tree only, fixer →
  the production harness tree only). The drivers have no Write/Edit tools; if
  crush can't deliver (quota, repeated 124-stalls, brief-violations past the
  cap) the driver returns a blocker — the finding goes OPEN with its truthful
  red spec, nothing is ever hand-written.
- The test-author's crush never touches production code; the test-fixer's
  crush never touches tests (both guard-enforced; the fixer driver also keeps
  the block_test_edits hook). The fixer is task-blind: it gets the reproduce
  file's path, nothing else.
- Never re-flag anything matching the waivers file, an already-pinned spec, or
  this run's ledger.
- Fully autonomous: never pause mid-loop to ask; collect questions for the
  final report.
- Keep bulk data on disk — read probe SUMMARY lines and findings files, not raw
  telemetry JSON, unless a summary warrants digging.
- Never touch `tmp/implement-task/` (that state belongs to implement-task).

## Setup (once per run)

0. Crush preflight: `crush` on `$PATH`, then one trivial call through the
   wrapper — `echo "Reply with exactly OK" | <crush_wrapper> author - preflight`.
   Auth/model failure or a stall → ABORT the sweep and tell the user (don't
   discover it mid-loop in round 1).
1. `RUN=tmp/visual-sweep/$(date +%Y%m%d-%H%M%S)`; `mkdir -p $RUN/round-1`.
2. Boot the target app and register a session per the adapter's **Launch**;
   verify the entry URL responds.
3. Build the dedupe baseline: read the waivers file (paths.yaml
   `visual_waivers`) and the doc-comments of existing pinned specs (adapter
   **Cross-run memory**). Seed `$RUN/ledger.md` with one row per known
   fingerprint so nothing known is re-flagged.

## The loop (rounds 1..5, hard cap)

```text
for round in 1..5:
  1 EXPLORE (inline): run the three probes from the skill's scripts/ dir
    against the adapter's entry URLs (VS_URL/VS_OUT per methodology.md),
    artifacts -> $RUN/round-N/. Follow up every probe signal with a Playwright
    MCP judgment pass (hover the flagged element, watch it, screenshot at
    decision points). Add judgment-only passes: click-where-safe, focus rings,
    animations, composition.
  2 TRIAGE: write $RUN/round-N/findings.md — one entry per finding:
    fingerprint, symptom, evidence path (JSON/webm/screenshot), user impact.
  3 DEDUPE + GATE: drop rows matching the ledger. Zero NEW findings -> break.
    Record the keepers in the ledger as NEW.
  4 PIN: spawn visual-sweep-test-author (findings path + adapter path).
    Its crush (role: author) writes the specs; the driver returns a reproduce
    block. Verify its specs went RED for the right reason and only
    test/contract files changed (git status).
  5 FIX: write the reproduce block VERBATIM to $RUN/round-N/reproduce.md,
    then spawn visual-sweep-test-fixer (that file's path, nothing else).
    Its crush (role: fixer) writes the code; follow-ups happen INSIDE the
    driver (cap 2, same crush session). If the driver returns red or blocked
    -> mark the finding OPEN in the ledger, keep the truthful red spec,
    continue.
  6 VERIFY (inline): re-run the new specs + the adapter's regression canary
    yourself — never trust a reported green. Update ledger: PINNED/FIXED/OPEN.
  7 RESTACK: rebuild + restart the app per the adapter so round N+1 explores
    the FIXED build, not the stale image.
FINAL: run the adapter's full regression once; write $RUN/report.md; report.
```

## Agent contracts

- **visual-sweep-test-author** (opus driver): prompt = two paths (round
  findings file, target adapter file). Its crush session (wrapper role
  `author`) writes the specs; the driver verifies valid RED and returns ≤30
  lines + a COMPLETE reproduce block.
- **visual-sweep-test-fixer** (sonnet driver): prompt = the reproduce file's
  path, nothing else. Never include findings, goals, or fix hints. Its crush
  session (wrapper role `fixer`) writes the code; red-after-crush retries
  happen inside the driver (`--continue`, cap 2) — the orchestrator does NOT
  relaunch the agent with guidance.
- Crush mechanics (both drivers): invoke only via `crush_wrapper`; prompts
  are piped heredocs preserved under `crush_artifacts`; playwright inside
  crush always runs `--reporter=dot` or redirects to a tmp/crush/*.log; a
  stalled run is tree-killed by the wrapper (exit 124) — relaunch once, then
  blocker.
- Pass artifacts by path, never inline. Both agents are capped at one crush
  session by their own instructions — do not extend them.

## Report (final message + $RUN/report.md)

- Per-round table: fingerprint → disposition (NEW/DUP/WAIVED/PINNED/FIXED/OPEN)
  → spec → fix summary.
- Specs added (paths), production files changed (from fixer returns — the fix
  tree is gitignored, so no diff exists), contract/testid additions.
- OPEN issues with evidence paths; the artifact index (videos, screenshots).
- Waiver proposals for findings judged intended-behavior — propose only; write
  to the waivers file solely with explicit user approval.
- Full-regression result, verbatim tail.
