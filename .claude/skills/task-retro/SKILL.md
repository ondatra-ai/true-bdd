---
name: task-retro
description: Self-improvement retro for a completed (or abandoned) implement-task run — reads the task's plan, workflow log, Codex ledger, phase-state events, and metrics, then writes an analysis with concrete diff proposals against the skills/agents to docs/context/retro/<slug>.md. Proposals ONLY — it never edits .claude/skills or .claude/agents itself. Runs as the mandated final step of implement-task (before phase_state close), or standalone via /task-retro <slug>.
---

# Task retro

Turn one finished task into concrete, reviewable improvements to the workflow that
produced it. Output: `docs/context/retro/<slug>.md` (create the folder if absent;
`docs/context/` survives `/new-task` by design).

**Hard rule: proposals only.** This skill and its agent NEVER edit
`.claude/skills/*`, `.claude/agents/*`, `.claude/hooks/*`, or `CLAUDE.md`. The user
reads the retro and applies what they accept as a normal reviewed change ("apply
retro <slug>"). Do not depend on the context archivist.

## Inputs (all read, none required to exist)

Take `<slug>` from the skill argument, or from the active phase state, or ask.
Paths per `docs/context/paths.md`:

- The plan `docs/tasks/plans/<slug>.md` — especially **Workflow log** and
  **Challenges** — and the brief `docs/tasks/<slug>.md`.
- The Codex rounds ledger `docs/tasks/plans/<slug>.codex.md` (kept/skipped per round).
- Phase state: `tmp/implement-task/active.json` if it is this slug (retro runs
  BEFORE `phase_state.py close`), else the newest `tmp/implement-task/<slug>/state.*.json`
  plus the slug's line in `docs/context/skill-metrics.jsonl`.
- Enforcement audit trails: `tmp/implement-task/phase-state.log`,
  `tmp/block_test_edits.log` (violation attempts).

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
3. **Write `docs/context/retro/<slug>.md`** with exactly these sections:
   - **Run summary** — one table: phase → spawns/completions, tokens, duration.
   - **What cost the most** — top 2–3 cost centers with numbers.
   - **Deviations & violations** — improvisations, blocked attempts, escapes.
   - **Codex loop efficiency** — keeps per round; rounds that were pure confirmation.
   - **Proposals** — each: the problem (with evidence from THIS run), then a
     ready-to-apply **unified diff** against the specific `.claude/skills/*/SKILL.md`
     / `.claude/agents/*.md` file. Only propose changes this run's evidence
     supports; 2–5 sharp proposals beat 10 speculative ones. If the evidence
     supports no change, say so — an empty Proposals section is a valid result.
- The subagent returns ≤10 lines: the retro path + one line per proposal.

## Exit

Tell the user: the retro path, the proposal count, and that nothing was applied —
they apply by saying "apply retro <slug>" (a normal reviewed change on its own
branch).
