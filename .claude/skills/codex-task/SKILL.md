---
name: codex-task
description: The full test-first, Codex-verified workflow for a substantial change in this repo — runs identify-task (understand + lock the goal) THEN implement-task (Playwright tests before code, Codex-critiqued plan, opus coding subagents, final Codex review). Use this whenever the user wants a Codex-reviewed/critiqued substantial change ("consult codex", "codex critique/review", "codex-reviewed", "use the codex loop", "do this task"), for test-first/Playwright work ("write the tests first"), AND proactively for any sizable feature, refactor, or architectural change. This project requires this Codex-involved, test-first process for substantial tasks.
---

# Codex task (orchestrator)

Two phases, in strict order:

1. **identify-task** — understand the task, surface risks, lock the goal with the user via Codex-driven discovery. **Pass your skill argument (the task idea) through to identify-task** so it isn't re-asked. Output: `docs/tasks/<slug>.md`.
2. **implement-task** — ship it test-first (Playwright tests BEFORE code), Codex-critiqued plan, opus coding subagents until all Playwright tests pass, final Codex review.

Do not start implement-task until identify-task's goal is locked. Hand the brief path (`docs/tasks/<slug>.md`) from identify-task to implement-task.

Shared Codex mechanics (invocation, prompt guide, Playwright-MCP setup): `references/codex.md`. Flag-baking wrapper: `scripts/codex.sh`.
