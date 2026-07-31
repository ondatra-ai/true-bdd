---
name: implement-task
description: Second half of the codex-task workflow — take a locked goal and ship it test-first: Playwright tests written BEFORE code, a Codex-critiqued plan, implementation via opus coding subagents until all Playwright tests pass, then a final Codex review. Use when implementing an already-scoped task ("implement/build/ship this"), for test-first/Playwright work, or when the codex-task orchestrator calls it. Reads the brief at docs/tasks/<slug>.md (from identify-task).
---

# Implement task

Test-first, Codex-verified. Input: the brief at `docs/tasks/<slug>.md` produced by identify-task (the orchestrator hands the path over; if running standalone, take it from the user or the most recent file in `docs/tasks/`).

1. **Plan, tests first.** The plan leads with Playwright tests (what they assert), then the code that makes them pass.
2. **Codex plan critique (≤3 rounds):** give Codex the plan + docs; ask for gaps/improvements; apply only relevant. Run a **separate** Playwright-specific critique (coverage, assertions, flakiness, "would it fail if broken?").
3. **Implement via opus coding subagents** (`model: opus` — never inherit the session model). Tests first, then code. Manage them; review diffs.
4. **Tests are the source of truth.** Change a test only if the test itself is wrong — never to green the suite.
5. **All Playwright tests must pass before leaving this stage.**
6. **Codex solution critique (≤3 rounds):** ideally let Codex run the tests + open the site in Playwright MCP; fix relevant, skip irrelevant.

Run Codex non-interactively — without a sandbox flag it hangs:
```bash
codex exec -s read-only --ephemeral -C "$PWD" --color never \
  -c model_reasoning_effort=low -o ./tmp/codex-review.md - < ./tmp/codex-prompt.md
```
Use `-s workspace-write` when Codex must run the tests itself. Background it; full guide + wrapper: `.claude/skills/codex-task/`.
