# Complexity matrix — Codex rounds per phase

Normative for `implement-task`: the task's **lane** (tiny / easy / hard) sets which
agents run and how many Codex rounds each phase gets. The lane is auto-classified
by the orchestrator from the brief, announced in one line with its reasons, user-
overridable with a word, and declared in the phase state so the gates and metrics
know it.

## The matrix

| Phase (agent) | tiny | easy | hard |
|---|---|---|---|
| **Plan** (`implement-task-planner`) | no agent — orchestrator writes a ~10-line mini-plan inline | no agent — planning folds into the test-author (mini-plan + tests, one Opus pass) | full planner agent — **≤3** rounds |
| **Tests** (`implement-task-test-author`) | agent runs, **0** rounds | agent runs (plan + tests), **1** round | agent runs, **≤3** rounds |
| **Code** (`implement-task-coder`) | blocker consultation only, as needed | blocker consultation only, as needed | blocker consultation only, as needed |
| **Review** (`implement-task-reviewer`) | **1** round | **1** round | **≤3** rounds |
| **Total Codex rounds** | **1** | **2** | **≤9** |

**Review floor = 1 round at every lane, non-negotiable.** The review's adversarial
substance IS the Codex loop (the reviewer scores Codex findings); a reviewer with
zero rounds is only a smoke-tester. Whatever else a lane trims, review never drops
below 1.

The coder never runs the relevance loop at any lane — its single focused blocker
consultation (CODE-FIX / STOP) fires only when a test looks impossible to satisfy.

"≤N" keeps the existing early exit (codex-loop.md): stop when a round yields no
keeps.

## Lane classification signals

- **tiny** — 1–2 requirements; expected change surface ~1 file (or config/copy/
  template tweak); follows an existing pattern; no new scaffolding; single surface.
- **easy** — up to ~5 requirements; a few files; no new services/architecture;
  known patterns.
- **hard** — any of: new scaffolding (services, compose files, Dockerfiles),
  cross-service change, >5 requirements, novel patterns, both UI and CLI surfaces,
  or genuine uncertainty about the approach. **When unsure, pick the harder lane.**

## Auto-escalation (misclassification valve)

Escalate ONE lane (tiny→easy, easy→hard) — log it in the workflow log and the
phase state — when any of:

- the coder stops with a blocker;
- the red baseline drifts from the reproduce block;
- the suite is still red after ~3 coder iterations;
- the reviewer's single round returns multiple kept findings (the task was bigger
  than classified — give the next round the full cap).

Escalation adds the *next* lane's missing rounds; it never restarts completed work.

## Invariant at every lane (never trimmed)

Red tests first (authored by a non-coder agent, assertion-red confirmed) · hook-
blocked coder, separate agent · orchestrator re-runs the suite itself · reviewer
agent completes (hook-enforced) · phase-state lifecycle with the lane recorded in
the metrics line — so `task-retro` can judge whether the lane call was right.
