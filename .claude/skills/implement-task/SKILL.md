---
name: implement-task
description: Second half of the task workflow (identify-task then implement-task) — ship an already-scoped task test-first via four model-pinned subagents: an Opus planner writes a tests-first plan, an Opus test-author writes the e2e/BDD tests (+ startup scaffolding) and leaves them red, a Sonnet coder greens them by implementing production code plus its backing unit tests (hard-blocked from touching the e2e/BDD tests that drive the task), and an Opus reviewer runs a final Codex review. The task's lane (tiny/easy/hard, auto-classified per references/complexity-matrix.md) sets which agents run and each phase's Codex round cap — review always gets ≥1 round; each Codex loop has the agent score findings (composite ≥7/10 + pass/fail gates). Phase order (lane-aware) and reviewer completion are hook-enforced via .claude/hooks/phase_state.py. Use when implementing an already-scoped task ("implement/build/ship this", "do this task", "consult codex") — take the brief from identify-task's docs/tasks/<slug>.md. All folder paths come from docs/context/paths.md.
---

# Implement task

Test-first, Codex-verified, multi-agent. Input: the task brief from `identify-task`
(path in `docs/context/paths.md` → Inputs; the orchestrator hands the slug over —
standalone, take it from the user or the most-recent brief). **Read
`docs/context/paths.md` and take every folder/file location and run command from it
— do not hardcode paths in these instructions or in agent prompts.**

## Lane + phase state (step 0, once, at task start)

**Classify the task's lane first** — tiny / easy / hard, per the signals in
`references/complexity-matrix.md` (the matrix is normative: which agents run and
each phase's Codex round cap). Announce it in ONE line with the reasons — e.g.
`lane: tiny — 1 requirement, single file, existing pattern` — so the user can
override with a word. When unsure, pick the harder lane.

Then run `.claude/hooks/phase_state.py start <slug> --lane <tiny|easy|hard>`. It
opens the task's phase state (`tmp/implement-task/active.json`) — the file the
enforcement hooks read to gate phase order (lane-aware), turn-ends, and commits.
The hooks also auto-create it on the first agent spawn as a fallback (defaulting
to lane hard), but the explicit start records the slug, branch, and lane cleanly.
Related protocol used throughout this skill:

- **Every agent prompt's first two lines are `slug: <slug>` and `lane: <lane>`** —
  the hook keys spawns to the task by the slug line; agents take their Codex round
  cap for the lane from the complexity matrix.
- **Auto-escalation:** on any matrix trigger (coder blocker, red-baseline drift,
  suite still red after ~3 coder iterations, a tiny-lane review round returning
  multiple keeps) run `.claude/hooks/phase_state.py escalate --reason "<trigger>"`
  (bumps one lane, audited), log it in the Workflow log, and grant the remaining
  phases the new lane's caps. Escalation never restarts completed work.
- **Every "STOP and report" below means:** run `.claude/hooks/phase_state.py block
  --reason "<the exact violation>"`, then report to the user. That records the stop
  deliberately (auditable, keeps the commit gate armed) instead of just ending the
  turn.
- The gates are enforcement, not workflow: a denied spawn or blocked turn-end means
  a protocol step was missed — fix the step, never work around the hook.

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

Spawn the lane's agents **one at a time** via the Agent tool (`subagent_type`),
handing each a prompt whose first lines are `slug: <slug>` and `lane: <lane>`, plus
(once a plan exists) the plan path. **Forward the test-author's reproduce block
verbatim to the coder.** Maintain a
**Workflow log** section in the plan (append a timestamped line per phase/decision)
and surface agent status lines to the user. Review each agent's return before the
next.

**Prompt discipline:** pass artifacts by PATH, never inline their contents — the plan,
the brief, the diff, hook contracts all live on disk and the agents read paths.md.
The reproduce block is the one exception (small and load-bearing). Agents return
compact reports (≤30 lines); when you need detail, read their on-disk artifacts
instead of asking for longer returns.

**Bulk file transfer:** when any step pulls many files or large payloads (design
mirrors, fixtures, downloads), write them straight to disk via Bash (curl/cp/base64
decode to the target path) — never round-trip file bodies through model context as
Read-then-Write. This applies to you and to every agent.

## Phase 1 — Create end-to-end tests

1. **Plan (1.1) — by lane:**
   - **hard**: spawn `implement-task-planner` (Opus). It writes a tests-first plan
     to the plan path (paths.md → Plan) using its template, hardened by its Codex
     critique loop (matrix cap). Review the plan.
   - **easy**: no planner agent — the test-author writes the mini-plan (see 1.2).
   - **tiny**: no planner agent — YOU write a ~10-line mini-plan to the plan path
     yourself (Goal, e2e test cases with exact assertions, files to touch) before
     spawning the test-author. A plan artifact exists at every lane.
2. **Tests (1.2).** Spawn `implement-task-test-author` (Opus). It writes ONLY e2e
   tests (+ empty startup scaffolding) and leaves the suite **RED** — tests run but
   fail on the absent behavior. On the **easy** lane it FIRST writes the mini-plan
   to the plan path (Goal, test cases + assertions, files to touch), then the tests.
   Its Codex rounds follow the matrix cap for the lane (tiny: 0, easy: 1, hard: ≤3).
   Confirm the red is an assertion failure (not a crash/collection error).
3. **Verify test-author scope.** Re-manifest the **production-code dirs** and diff
   against the **production-only baseline** (like-for-like). ANY difference — a file
   **added, modified, or deleted** under the production dirs — is a violation: STOP
   and report. Only NEW e2e tests and NEW scaffolding files (the patterns in paths.md
   → Architectural startup scaffolding) are permitted.

## Phase 2 — Implement code

4. **Snapshot** the off-limits tree (re-manifest) AND the package `scripts` to "before"
   files. Spawn `implement-task-coder` (Sonnet) with the reproduce block. It runs only
   those red tests and implements production code (plus the unit tests that back it);
   file-edit tools AND Bash writes (including `git apply`/`patch` and snapshot-update
   flags) to the off-limits e2e/BDD paths are blocked by its hook.
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

7. Spawn `implement-task-reviewer` (Opus) with a **minimal prompt**: the `slug:` line,
   the plan path, and the **diff artifact path/command** — the true content diff is
   `diff -r --no-index` of the change-surface content copy against the current tree
   (it shows added, modified, AND deleted content; the reviewer inspects all three).
   Never inline the diff, the task, or the plan into the prompt — pass paths; an
   oversized prompt is exactly what once made this spawn fail. It runs an independent
   read-only Codex review — every round sends the FULL task + the full diff + ALL
   prior-round findings, and the **reviewer scores** each finding (composite ≥7 +
   gates; ≤3 rounds) — hardening e2e tests and code; it applies the keeps and runs
   the tests itself. Confirm the suite is green afterward.

   **If the reviewer spawn fails:** retry ONCE with the minimal prompt above. If it
   fails again, run `.claude/hooks/phase_state.py block --reason "reviewer spawn
   failed: <error>"` and put the decision to the user via `AskUserQuestion`.
   **NEVER run the review inline yourself** — an inline review is not a review, and
   the stop/commit gates will hold the task open until the reviewer agent has
   actually completed.

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
- [ ] **Codex loops ran per the lane's matrix caps** (`references/complexity-matrix.md`)
      — review ALWAYS ≥1 round at every lane; the Codex rounds ledger (paths.md →
      Plan: `<slug>.codex.md` beside the plan) is populated for every phase that ran
      rounds, and the plan points to it.
- [ ] **Coder touched no e2e/BDD tests** — off-limits manifest (the `tests/` tree) and
      package-`scripts` snapshot both clean (zero e2e/BDD-test / test-script edits). The
      coder MAY have written unit tests — those are not off-limits and don't count here.
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
- **Lane** — the declared lane, plus any escalation and its trigger.
- **Verdict** — DONE (all ✓) or NOT DONE (list the failing items).

### Close (after the report)

1. **Retro** — invoke the `task-retro` skill for `<slug>`. It writes the
   self-improvement analysis + proposals to `docs/context/retro/<slug>.md`
   (proposals-only; it never edits skills/agents itself).
2. **`.claude/hooks/phase_state.py close --done`** — refuses unless the reviewer
   agent actually completed; appends the task's metrics line to
   `docs/context/skill-metrics.jsonl` and archives the phase state. If the task is
   being dropped instead, `close --abandoned --approved-by-user "<their words>"` —
   only with the user's explicit approval.

---

Shared Codex loop (full-context every round; the agent scores) and Codex
mechanics/wrapper: paths.md → Codex.
