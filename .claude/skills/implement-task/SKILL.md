---
name: implement-task
description: Second half of the codex-task workflow — ship an already-scoped task test-first via four model-pinned subagents: an Opus planner writes a tests-first plan, an Opus test-author writes the e2e tests (+ startup scaffolding) and leaves them red, a Sonnet coder greens them by implementing production code only (hard-blocked from touching tests), and a Fable reviewer runs a final Codex review. Each phase runs its own Codex critique loop (the agent scores composite ≥7/10 + pass/fail gates, ≤3 rounds). Use when implementing an already-scoped task ("implement/build/ship this"), or when the codex-task orchestrator calls it. All folder paths come from docs/context/paths.md.
---

# Implement task

Test-first, Codex-verified, multi-agent. Input: the task brief from `identify-task`
(path in `docs/context/paths.md` → Inputs; the orchestrator hands the slug over —
standalone, take it from the user or the most-recent brief). **Read
`docs/context/paths.md` and take every folder/file location and run command from it
— do not hardcode paths in these instructions or in agent prompts.**

## Baselines (once, at task start)

Capture these so later checks attribute to THIS task even on a dirty worktree. Read
every path from paths.md. (A "manifest" = sorted `shasum` lines; same command at
baseline and recheck → exact diff.)

- **production-only manifest** — the production-code dirs (for the Phase 1 scope check).
- **off-limits manifest** — the off-limits paths, incl. the individual config files
  (for the Phase 2 coder check).
- **package-scripts snapshot** — the `scripts` object of the package manifest (paths.md
  → Package manifest) (for the Phase 2 scripts check).
- **change-surface content copy** — a recursive copy of the production-code dirs + the
  e2e dir (excluding the change-surface exclusions listed in paths.md) (for the Phase 3
  reviewer's true content diff).
- Also record `git rev-parse HEAD` and `git status --short`.

## Orchestration

Spawn the four agents **one at a time** via the Agent tool (`subagent_type`), handing
each the `<slug>` and (from phase 1.1 on) the plan path. **Forward the test-author's
reproduce block verbatim to the coder.** Maintain a **Workflow log** section in the
plan (append a timestamped line per phase/decision) and surface agent status lines to
the user. Review each agent's return before the next.

## Phase 1 — Create end-to-end tests

1. **Plan (1.1).** Spawn `implement-task-planner` (Opus). It writes a tests-first plan
   to the plan path (paths.md → Plan) using its template, hardened by a Codex critique
   loop. Review the plan.
2. **Tests (1.2).** Spawn `implement-task-test-author` (Opus). It writes ONLY e2e
   tests (+ empty startup scaffolding) and leaves the suite **RED** — tests run but
   fail on the absent behavior. Confirm the red is an assertion failure (not a
   crash/collection error).
3. **Verify test-author scope.** Re-manifest the **production-code dirs** and diff
   against the **production-only baseline** (like-for-like). ANY difference — a file
   **added, modified, or deleted** under the production dirs — is a violation: STOP
   and report. Only NEW e2e tests and NEW scaffolding files (the patterns in paths.md
   → Architectural startup scaffolding) are permitted.

## Phase 2 — Implement code

4. **Snapshot** the off-limits tree (re-manifest) AND the package `scripts` to "before"
   files. Spawn `implement-task-coder` (Sonnet) with the reproduce block. It runs only
   those red tests and implements production code; file-edit tools AND Bash writes
   (including `git apply`/`patch` and snapshot-update flags) to off-limits paths are
   blocked by its hook.
5. **Verify, don't blindly roll back.**
   - Re-manifest the off-limits tree to an "after" file, `diff` vs "before". Any
     off-limits file added/modified/deleted → **STOP and report exactly which** (no
     blind rollback).
   - Re-snapshot the package `scripts`, `diff` vs "before". Any change to the
     `scripts` object (dependency changes elsewhere are fine) → **STOP**.
   - **Run the final test suite yourself** — do NOT trust the coder's reported green.
     (This closes the transient-edit window: if a test was weakened then restored, the
     suite goes red again under your run; if left weakened, the off-limits diff catches
     it.)
6. **If the coder stops with a blocker:** research deep (architecture, solution
   design, context7, web) for a code-only fix.
   - **Code-only fix found:** re-run the coder with that guidance.
   - **Test itself wrong:** get the user's choice via `AskUserQuestion` (pursue a
     code-only fix vs approve a test change). On approval, re-run the test-author to
     fix the test, then re-run the coder.
   - **Record a structured challenge** in the plan's Challenges section: failing
     assertion, architectural constraint, sources consulted, Codex verdict, code-only
     approaches tried, your recommendation, the user's decision, next action.
   - A **deliberate stop** is only user-approved. Loop until green or an approved
     stop. Never green the suite by editing a test.

## Phase 3 — Review

7. Spawn `implement-task-reviewer` (Fable) with a **true content diff** —
   `diff -r --no-index` the change-surface content copy against the current tree (it
   shows added, modified, AND deleted content; the reviewer inspects all three). It
   runs an independent read-only Codex review — every round sends the FULL task + the
   full diff + ALL prior-round findings, and the **reviewer scores** each finding
   (composite ≥7 + gates; ≤3 rounds) — hardening e2e tests and code; it applies the
   keeps and runs the tests itself. Confirm the suite is green afterward.

---

## Done — report & definition of done

After Phase 3, produce a final report and verify the **universal definition-of-done
checklist** below. **The task is DONE only if every checklist item is ✓; otherwise it
is NOT DONE and the report states exactly which items failed.** This checklist is
higher-level than any specific task — it's "is this task actually shipped," independent
of the task's features.

### Definition of done (universal — not task-specific)

- [ ] **Authored e2e tests green** — the final suite, re-run by the orchestrator (not
      the coder), all passing.
- [ ] **Live smoke test passed** — the reviewer drove the UI via Playwright MCP and ran
      the CLI in a shell; each applicable part passed (or was skipped-with-note where
      no surface exists).
- [ ] **Codex loops ran** — the planner, test-author, and reviewer each ran their Codex
      critique loop; the plan's "Codex rounds" ledger is populated.
- [ ] **Coder touched no tests** — off-limits manifest and package-`scripts` snapshot
      both clean (zero test-file / test-script edits).
- [ ] **Test-author touched no production code** — production-only manifest clean (zero
      production-code additions/modifications/deletions).
- [ ] **Reproduce block honored** — the coder confirmed the same red baseline before
      editing.
- [ ] **Plan & challenges recorded** — plan written; any blocker decision logged in its
      Challenges section.
- [ ] **No hardcoded paths** — skill/agents/reference read from paths.md (only the
      paths.md pointer and the coder hook path are literals).

### Report shape (surface to the user)

- **Task outcome** — each goal/requirement → WORKS / PARTIAL / DOESN'T, with evidence.
- **Definition of done** — each checklist item → ✓/✗ + evidence.
- **Smoke test** — what was driven (CLI / browser), result, evidence.
- **Challenges** — blockers, decisions, escalations.
- **Changed files** — the added/modified/deleted list (from the change-surface diff).
- **Verdict** — DONE (all ✓) or NOT DONE (list the failing items).

---

Shared Codex loop (full-context every round; the agent scores) and Codex
mechanics/wrapper: paths.md → Codex.
