---
name: task-retro
description: Self-improvement retro for a completed (or abandoned) implement-task run — reads the task's plan, workflow log, Codex ledger, phase-state events, and metrics, writes an analysis with concrete diff proposals against the skills/agents to docs/context/retro/<slug>.md, then the ORCHESTRATOR applies the proposals automatically (user-mandated auto-apply, 2026-08-04; the retro file is the audit record, the working tree carries the applied diffs uncommitted). The analyst subagent itself never edits .claude/*. Runs as the mandated final step of implement-task (before phase_state close), or standalone via /task-retro <slug>.
---

# Task retro

Turn one finished task into concrete, reviewable improvements to the workflow that
produced it. Output: `docs/context/retro/<slug>.md` (create the folder if absent;
`docs/context/` survives `/new-task` by design).

**Division of labor (auto-apply policy, user-mandated 2026-08-04).** The analyst
SUBAGENT writes proposals only — it never edits `.claude/skills/*`,
`.claude/agents/*`, `.claude/hooks/*`, or `CLAUDE.md`. The ORCHESTRATOR then
applies every proposal's diff immediately after the retro is written — no user
verdict required. The retro file is the audit record of what changed and why;
the applied edits sit uncommitted in the working tree, reviewable via `git diff`
and revertable file-by-file. `CLAUDE.md` stays orchestrator-off-limits too —
propose CLAUDE.md changes in the retro but leave application to the user. If a
proposal's diff conflicts with the current file (drifted since the retro was
written), apply the intent manually and note the adjustment in the retro file.
Do not depend on the context archivist.

## Inputs (all read, none required to exist)

Take `<slug>` from the skill argument, or from the active phase state, or ask.
Paths per `docs/context/paths.yaml`:

- The plan `docs/tasks/plans/<slug>.md` — especially **Workflow log** and
  **Challenges** — and the brief `docs/tasks/<slug>.md`.
- The Codex rounds ledger `docs/tasks/plans/<slug>.codex.md` (kept/skipped per round).
- Phase state: `tmp/implement-task/active.json` if it is this slug (retro runs
  BEFORE `phase_state.py close`), else the newest `tmp/implement-task/<slug>/state.*.json`
  plus the slug's line in `docs/context/skill-metrics.jsonl`.
- Enforcement audit trail: `tmp/implement-task/phase-state.log`.

## Do

Spawn ONE subagent (`model: opus`, general-purpose) with the slug and the input
paths above (paths, not contents). It must:

1. **Reconstruct the run** from the workflow log + phase-state events: what ran,
   in what order, wall-clock and tokens per phase, re-runs, stop-blocks, order
   denies, blocker escalations.
2. **Judge cost vs. contribution.** Which phase cost the most (tokens/time) and
   what did it contribute? Which Codex rounds produced keeps vs. pure
   confirmation (kept/skipped per round from the ledger)? What did the
   orchestrator improvise outside the skill's script (compare the log against
   `.claude/skills/implement-task/SKILL.md`)? Any violations or near-misses in
   the audit logs?
3. **Judge regeneratability (standing lens, user-mandated 2026-08-04).** CORE
   PRINCIPLE: ALL harness code (paths.yaml → harness_code_root) must be
   REGENERATABLE from the e2e suite alone — the tests are the spec, the code is
   a build artifact. Every retro evaluates this run against that principle and
   proposes changes that make regeneration MORE effective over time: Did the
   task-blind test-fixer need knowledge the tests didn't carry (a gap in the
   e2e specs or the binding contract)? Did any behavior ship that no test
   demands (unregeneratable)? Would deleting harness_code_root and re-running
   implement-task reproduce the app? Proposals that strengthen the tests-as-spec
   loop rank above cost optimizations.
4. **Write `docs/context/retro/<slug>.md`** with exactly these sections:
   - **Run summary** — one table: phase → spawns/completions, tokens, duration.
   - **What cost the most** — top 2–3 cost centers with numbers.
   - **Deviations & violations** — improvisations, blocked attempts, escapes.
   - **Codex loop efficiency** — keeps per round; rounds that were pure confirmation.
   - **Regeneratability** — the standing-lens verdict (step 3): gaps found, or a
     clean pass.
   - **Proposals** — each: the problem (with evidence from THIS run), then a
     ready-to-apply **unified diff** against the specific `.claude/skills/*/SKILL.md`
     / `.claude/agents/*.md` file. Only propose changes this run's evidence
     supports; 2–5 sharp proposals beat 10 speculative ones. If the evidence
     supports no change, say so — an empty Proposals section is a valid result.
- The subagent returns ≤10 lines: the retro path + one line per proposal.

## Apply (orchestrator, immediately after the subagent returns)

Read the retro's Proposals section and apply every diff to its target file
(`.claude/skills/*`, `.claude/agents/*`, `.claude/hooks/*` — everything except
`CLAUDE.md`). Syntax-check what's checkable (`python3 -m py_compile` for hooks).
Leave everything uncommitted.

## Exit

Tell the user: the retro path, the proposal count, and which proposals were
auto-applied (with target files) — plus any CLAUDE.md-targeted proposal left for
their review. They can inspect via `git diff` and revert selectively.
