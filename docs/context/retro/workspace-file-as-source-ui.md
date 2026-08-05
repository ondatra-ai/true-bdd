# Task retro — workspace-file-as-source-ui

Lane: **hard**. Verdict: shipped GREEN (36 workspace specs + a10 real-Claude, all
pass — `tmp/final-gate-workspace.log`, `tmp/final-gate-a10.log`). This retro is
**proposals only**; nothing here is applied. Reconstructed from the plan Workflow
log + Challenges, the Codex ledger, `tmp/implement-task/active.json`,
`phase-state.log`, `block_test_edits.log`, and the four suite-evidence logs.

## Run summary

| Phase | Spawns/Completions | Tokens | Wall-clock (spawn→completion) | Codex rounds |
|---|---|---|---|---|
| Planner | 1 / 1 | ~283k | 13m (20:32:45→20:45:48, 08-03) | 3 rounds, 44 kept / 0 skipped |
| Test-author | 1 / 1 (+1 unrecorded resume) | ~437k | 52m (21:08:01→21:59:43) | 3 rounds, 17 applied / 1 churned |
| Coder | 1 / 1 | ~753k | 78m (22:01:03→23:19:32) | 1 blocker consult → STOP |
| Reviewer | 1 / 1 | ~316k | 35m (05:29:02→06:03:33, 08-04) | 3 rounds, 4 applied / 3 skip-upheld, R3 dry |
| **Total** | 4 / 4 | **~1.79M** | ~3h agent + ~6h coder→reviewer gap | 10 Codex loops |

Enforcement events: order_denies **0**; stop_blocks **3** (all spurious — see
Deviations); escalations **0** (already top lane); `block_test_edits` denials for
this run (08-03/08-04) **0** — the coder never attempted an off-limits edit. Phase
state currently reads `status: auto_blocked` (retro runs before close).

## What cost the most

1. **Coder — ~753k tokens (42% of the run), 78m.** Highest cost *and* highest
   contribution: greened slices 0–5, then during **autonomous a10 verification**
   (multiple real-Claude retries) found and fixed **two real production bugs** —
   chat route reused the 30s `DEADLINE_MS.inventory` (spurious 504 before Claude
   replied → dedicated `DEADLINE_MS.chat`), and Claude wrapping `new_content` in
   `---` document markers that the browser's strict `YAML.parse` silently rejected
   (`stripAccidentalDocumentMarkers` + Go regression). The real-Claude retry loop
   inside the coder is the single biggest token driver; it paid off.
2. **Test-author — ~437k tokens, 52m.** 36 `w*` specs + a10 + `workspace-env.ts` +
   the 11 scaffolding files + 3 Codex rounds (17 rigor findings applied).
3. **The ~6h coder→reviewer wall-clock gap** dwarfs any single phase but is not
   agent compute — it is the w3.3 blocker detour (Codex STOP consult →
   AskUserQuestion → test-author resume for one line), a resource-contaminated
   concurrent suite run, then two clean isolated final runs, plus an overnight
   human gap. Most of that interval is process friction, not review.

## Deviations & violations

- **Spurious stop-gate cascade → stale `auto_blocked` (enforcement false-positive).**
  Reviewer spawned 05:29:02; the stop gate fired `block` at 05:29:11 (**9s later**),
  05:29:41, then `yield-auto-blocked` at 05:30:08 — all while the just-spawned
  reviewer ran in the background (it completed at 06:03:33). The gate arms on
  `reviewer_pending` (coder-done ∧ reviewer-not-*completed*) and is blind to a live
  reviewer *spawn*, so a correct turn-end right after spawning the reviewer reads as
  "you skipped the reviewer." The reviewer completion never cleared the status, so
  `active.json` still sits at `auto_blocked` with a now-false `block_reason`. No
  orchestrator fault — the hook misfired. → Proposal 3.
- **Concurrent suite run contaminated results.** The orchestrator's concurrent
  workspace run reported 33/36 (`tmp/workspace-orchestrator-run.log`): w3.3 (real
  import bug), **plus w4.1 and w7.3 which were resource-contamination flakes** — both
  passed in the isolated final run (`tmp/final-gate-workspace.log`, 36 passed). The
  container-per-test harness starves under co-running load; 2 of 3 "failures" were
  noise, wasting triage. → Proposal 4.
- **w3.3 non-behavioral test bug took the full escalation path.** A one-line wrong
  import (`lineIndex` from `workspace-env`; it lives in `ui`) — coder hook-blocked
  from `tests/`, Codex read-only consult verdict **STOP** (unfixable in production),
  orchestrator verified the mismatch directly — still routed through
  `AskUserQuestion` + a test-author resume for one line, exactly per SKILL Phase 2.
  Correct to the letter; heavier than a Codex-confirmed mechanical repair warrants.
  → Proposal 2.
- **Test-author resume not recorded in phase state.** The post-completion resume for
  the approved w3.3 fix left no second `test_author` spawn in `active.json` (still 1
  spawn / 1 completion). Metrics under-count re-spawns. Minor; noted, no proposal.
- **Reviewer skipped re-running a10 (budget); orchestrator re-ran it (1/1, real
  Claude).** This *worked* and satisfies the def-of-done ("suite re-run by the
  orchestrator"). Division of labor held; no change proposed.
- **Clean scope enforcement (positive).** Production-only manifest = exactly the 11
  scaffolding files after the test-author; off-limits + `scripts` snapshots clean
  after the coder; zero `block_test_edits` denials this run. The hard block was never
  even exercised — scope was respected and the one test bug was escalated properly.

## Codex loop efficiency

- **Planner — 44 kept / 0 skipped across 3 rounds** (R1 14/14, R2 15/15, R3 4/4).
  Genuinely productive early (R1/R2 caught the S1-proof overclaim, scheduler
  `ClassRead` default, `/api/agent/reply` not browser-observable, revision-as-counter,
  atomicity/idempotency gaps). But **R3 produced 0 fresh findings** — all 4 keeps were
  partial-application fixes of prior rounds; R3 was a verification round, not a
  discovery round. A **0-skip rate over 44 findings** means the composite≥7 gate never
  once filtered on the plan — worth watching as a possible scoring-calibration signal,
  though the catches were real.
- **Test-author — 17 applied / 3 rounds**, with the `yaml` devDep item **churning
  across all three rounds** (R1 skip → R2 re-raise → R3 reframe) without ever being a
  code change — it was an authorization/disclosure question Codex kept surfacing.
  ~1 finding of pure churn.
- **Reviewer — the only discriminating loop.** R1 2 keeps / 3 skips, R2 2 keeps (incl.
  the high-value fresh #6: CLI YAML gate accepted multi-doc streams the browser
  rejects) / 3 skips upheld, **R3 fully dry (0 keeps)**. The three skips (path TOCTOU,
  concurrent idempotency, dir-fsync-on-failure) were challenged and upheld under the
  threat model — the scoring gate did real work here. R3 was pure confirmation; per
  "stop early on a dry round" it could have stopped at R2, though a final DRY
  verification has value at the hard cap.
- **Coder blocker consult — one read-only Codex call, STOP verdict**, correctly
  classifying w3.3 as a test-internal bug unfixable in production. Exactly the
  intended use of the blocker channel.

## Proposals

Four proposals, each grounded in this run. Two target `.claude/agents` /
`.claude/skills`; one targets the enforcement hook `.claude/hooks/phase_state.py`
(the bug lives there and cannot be fixed elsewhere). Apply by branching and editing —
this retro applies nothing.

### Proposal 1 — Add a `tsc --noEmit` static gate to the test-author's definition-of-red

**Problem.** w3.3 shipped a wrong import (`lineIndex` from `./helpers/workspace-env`;
it is exported from `./helpers/ui`). It was **invisible in red**: `tests/harness`
has no tsconfig/typecheck, Playwright transpiles specs *without* type-checking, so a
missing named export compiles clean and only throws when the runtime reaches the call
site — and every red spec died earlier at the `env.start()` session gate, so the
broken call site was never reached (the red sample ran w3.1, not w3.3). A static
typecheck at author time would have caught it before the coder ever saw it.

```diff
--- a/.claude/agents/implement-task-test-author.md
+++ b/.claude/agents/implement-task-test-author.md
@@ -41,6 +41,13 @@ coder. If booting needs a stub, create it as a new empty scaffolding file.
 4. Verify service readiness, then run only the new specs using paths.md commands.
    Valid RED is an assertion failure caused by absent behavior. Collection errors,
    crashes, timeouts, missing dependencies, or startup failures are defects in your
    tests/scaffolding: fix them. The tests must execute and fail on the missing
    behavior.
+5. Before declaring RED, gate the e2e test package with a **static typecheck**: run
+   `npx tsc --noEmit` over the configured e2e directory (create a minimal
+   `tsconfig.json` in that package as scaffolding if none exists) and drive it to
+   zero errors. Playwright transpiles specs WITHOUT type-checking, so a wrong import
+   path or a missing named export compiles clean and only explodes when the runtime
+   reaches the call site — which a spec that fails earlier (e.g. at the env/session
+   gate) hides. A green `tsc --noEmit` is part of a valid RED.
```

Corresponding config touch (not a skill/agent file, so shown here, not diffed):
`docs/context/paths.md` → Run commands could add a canonical
`cd tests/harness && npx tsc --noEmit` line so the coder and reviewer reuse the same
gate. The agent step above is self-contained without it (it creates the tsconfig and
runs the gate).

### Proposal 2 — Let the orchestrator apply Codex-confirmed NON-behavioral test repairs without user escalation

**Problem.** SKILL Phase 2 routes **any** test change to the user via
`AskUserQuestion`. That is the right guard for the core threat (assertion weakening),
but it is noise for a mechanical repair. w3.3 was a one-line import fix, coder
hook-blocked from `tests/`, Codex blocker verdict **STOP** (non-behavioral, unfixable
in production), orchestrator-verified — yet it still burned a full AskUserQuestion
round plus a test-author resume for one line.

```diff
--- a/.claude/skills/implement-task/SKILL.md
+++ b/.claude/skills/implement-task/SKILL.md
@@ -120,9 +120,17 @@
 6. **If the coder stops with a blocker:** research deep (architecture, solution
    design, context7, web) for a code-only fix.
    - **Code-only fix found:** re-run the coder with that guidance.
-   - **Test itself wrong:** get the user's choice via `AskUserQuestion` (pursue a
-     code-only fix vs approve a test change). On approval, re-run the test-author to
-     fix the test, then re-run the coder.
+   - **Test itself wrong — NON-BEHAVIORAL repair** (a wrong import path, a typo, a
+     syntax/compile error; provably ZERO change to any assertion, expected value, or
+     selector semantics, and Codex's blocker verdict confirms the fix is
+     non-behavioral): the orchestrator may direct the test-author to apply it
+     directly — no `AskUserQuestion` — recording the diff plus the Codex confirmation
+     in the Challenges section as the evidence. The coder stays hard-blocked from
+     tests, so the test-author still makes the edit.
+   - **Test itself wrong — BEHAVIORAL change** (any assertion weakening, expected-
+     value edit, or selector-semantics change — the core threat — or any doubt about
+     whether the change is behavioral): get the user's choice via `AskUserQuestion`
+     (code-only fix vs approve the test change). On approval, re-run the test-author
+     to fix the test, then re-run the coder.
    - **Record a structured challenge** in the plan's Challenges section: failing
```

### Proposal 3 — Stop gate must treat a live reviewer *spawn* as satisfying the window; clear `auto_blocked` on reviewer completion

**Problem.** The stop gate blocked turn-end **9 seconds after** the reviewer was
spawned and cascaded to `auto_blocked` within 57s, all while the reviewer legitimately
ran in the background (spawn 05:29:02, blocks at 05:29:11/:41 + yield 05:30:08,
completion 06:03:33). `reviewer_pending()` checks only coder-done ∧
reviewer-not-*completed*; it is blind to a recorded reviewer *spawn*. Since subagents
run in the background, ending the turn right after spawning the reviewer is exactly
correct — but the gate reads it as skipping the reviewer. The completion then never
cleared the status, so `active.json` still shows `auto_blocked` with a false
`block_reason`. Scoping the spawn-awareness to the **stop** gate is safe: the commit
gate and the nag banner still use spawn-agnostic `reviewer_pending`, so a
spawned-but-never-completed reviewer keeps the task open for commit/close.

```diff
--- a/.claude/hooks/phase_state.py
+++ b/.claude/hooks/phase_state.py
@@ -390,8 +390,11 @@ def hook_stop_gate(payload: dict) -> int:
             audit({"mode": "stop-gate", "decision": "allow", "via": "marker"})
             return allow()
+        # A reviewer that has been spawned but not yet completed IS running in the
+        # background — the gate exists to catch a MISSING reviewer, not a slow one.
+        reviewer_spawned = len(state["phases"]["reviewer"]["spawns"]) >= 1
         armed = (
             reviewer_pending(state)
+            and not reviewer_spawned
             and state["turn"].get("activity")
             and state["turn"].get("prompt_id") == payload.get("prompt_id")
         )
         if not armed:
             return allow()
@@ -343,6 +346,10 @@ def hook_subagent_stop(payload: dict) -> int:
         state["phases"][phase]["completions"].append(
             {"ts": now(), "agent_id": agent_id, "source": "subagent-stop"})
         add_event(state, "completion", phase=phase, source="subagent-stop")
+        # A stop-gate-induced auto_block is stale once the reviewer actually completes.
+        if state["status"] == "auto_blocked" and not reviewer_pending(state):
+            state["status"] = "in_progress"
+            state["block_reason"] = None
+            add_event(state, "auto_unblocked_by_completion", phase=phase)
         save_state(state)
     audit({"mode": "subagent-stop", "phase": phase})
     return allow()
```

(Mirror the same three-line `auto_blocked` clear in `hook_agent_post` after its
completion append, since either path may record the reviewer completion. A deliberate
CLI `block` — status `"blocked"` — is intentionally NOT auto-cleared.)

### Proposal 4 — Orchestrator must run the container-per-test e2e suite in isolation

**Problem.** The orchestrator's concurrent workspace run reported 33/36
(`tmp/workspace-orchestrator-run.log`); **2 of the 3 "failures" (w4.1, w7.3) were
resource-contamination flakes** that passed cleanly in the isolated final run
(`tmp/final-gate-workspace.log`, 36 passed). The harness launches one docker
container per test, so a co-running suite/build starves it and manufactures failures
that masquerade as real — costing triage effort to separate noise from the one genuine
failure (w3.3).

```diff
--- a/.claude/skills/implement-task/SKILL.md
+++ b/.claude/skills/implement-task/SKILL.md
@@ -116,6 +116,11 @@
    - **Run the final test suite yourself** — do NOT trust the coder's reported green.
      (This closes the transient-edit window: if a test was weakened then restored, the
      suite goes red again under your run; if left weakened, the off-limits diff catches
      it.)
+     Run the e2e suite in **isolation** — never concurrently with another suite,
+     `docker compose` stack, or heavy build. The harness launches one container per
+     test, so a co-running load starves it and produces resource-contamination flakes
+     that masquerade as real failures; re-run any failure isolated before treating it
+     as genuine.
```
