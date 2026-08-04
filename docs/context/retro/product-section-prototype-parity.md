# Retro — product-section-prototype-parity

First FULL **hard-lane** run of the redesigned workflow (all four agents: planner
→ test-author → task-blind test-fixer → reviewer, every phase at its hard cap).
Task: make prod's Product section + shell chrome (`harness/src`) match the
runnable file-as-source prototype, tests-first. Outcome: **DONE / green** — 7
deterministic parity cases (w12–w14) + 3 codex vision-judge cases (w15) + the
reviewer's added w14.2/w13.5, full workspace regression 62/62, live smoke PASS,
orchestrator-verified. This is the second retro under the redesign (the third
task, `workspace-overview-design-parity`, closed WITHOUT a retro — user-stopped).
Reconstructed from `active.json`, `phase-state.log`, `block_test_edits.log`
(session f7adaf7d), the plan Workflow log, and the four-section Codex ledger.

## Run summary

| Phase | spawn→complete | duration | Codex (keep/skip per round) |
|---|---|---|---|
| planner | 14:04:25 → 14:13:48 | **9m23s** | 3 rounds — 16 / 9 / 3 keeps (28 total) |
| ↳ plan review + R7 decision gap | 14:13:48 → 14:36:13 | 22m25s | orchestrator resolved R7, overrode w13.4 |
| test_author | 14:36:13 → 15:17:53 | **41m40s** | 3 rounds — 5 / 2 / 2 keeps (9) |
| test_fixer | 15:18:32 → 16:18:13 | **59m41s** | fixed 3 — 1 / 1 / 0 keeps (2) |
| ↳ independent verify gap | 16:18:13 → 16:20:50 | 2m37s | orchestrator re-ran 60/60 fresh image |
| reviewer | 16:20:50 → 16:44:58 | **24m08s** | 3 rounds — 2 / 2 / 0 keeps (4) |

Run total ≈ **2h40m** (14:04:25 → 16:44:58, before retro) — roughly double the
easy-lane `design-conformance-tests` run (~1h27m), as expected for a full four-agent
hard lane. **All 12 hard-lane Codex rounds spent** (planner 3 · author 3 · fixer 3 ·
reviewer 3 = the matrix ceiling). 43 total keeps. `turn.stop_blocks` 0, order-denies
0 (this run). Escalations 0 — the lane was called `hard` correctly at the start and
never had to be bumped. Tokens/durations still `null` in `skill-metrics.jsonl` (the
standing capture gap flagged in retro #1; unchanged).

## What cost the most

1. **Test-fixer phase (59m41s) — the single longest phase.** Fixed 3 Codex rounds
   plus repeated full 60-case Playwright regression (one container per test) plus the
   w15 prototype boot inside the suite (`npm ci` + `next` compile + codex vision judge
   ×3). Round 3 kept 0 / skipped 3 (all "more prototype fidelity than any spec
   demands") — the mandated-dry confirmation round the fixed-3 rule always spends,
   exactly as in retro #1. Policy-correct (user mandate "always, depends on nothing"),
   noted not re-proposed.
2. **Test-author phase (41m40s).** 3 Codex rounds + RED verification that boots the
   prototype LIVE (`npm ci` minutes-long, per-route readiness). The prototype boot is
   paid AGAIN here, then again in the fixer regression, again in the orchestrator's
   verify, again in the reviewer — four live boots of the same app across the run, the
   dominant repeated compute alongside the 60-case suite passes.
3. **Planner babysitting + the 22m review gap.** The planner premature-stopped TWICE
   waiting on Codex rounds; the orchestrator process-watched and SendMessage-resumed it
   (see Deviations). The 22m planner→author gap also carried the genuine R7-conflict
   resolution, so it is not pure waste — but the babysitting slice is avoidable and is
   a straight recurrence of retro #1's defect in a phase whose brief was never fixed.

## Deviations & violations

- **RECURRENCE — the planner premature-stopped 2× mid-Codex-loop** (the exact defect
  retro #1 diagnosed for the test-fixer, now in a new phase). Root cause is the same
  structural one: `codex-loop.md` step 2 runs Codex as a **background** task + a
  future-turn `Monitor`, so the model retains a window to decide it is "done" and stop
  before the Monitor fires. Retro #1 fixed this for the **test-fixer only** (foreground
  rule in its brief + the codex-loop exception). The **planner brief still carries the
  old pattern verbatim** (`implement-task-planner.md:48-49`: "Monitor each Codex round
  with bounded `Monitor` until it exits; do not end the turn while it runs") — so it
  bit the planner this run. The test-author and reviewer briefs carry the **identical
  latent pattern** (`implement-task-test-author.md:57-58`, `implement-task-reviewer.md:56-57`);
  they ran clean only because the orchestrator patched foreground discipline into their
  spawn prompts by hand — per-run compensation, not a durable fix. `phase_state`'s
  `stop_blocks` still counts only the orchestrator's own stops (0 this run), so the two
  planner premature-stops were invisible to the gate and had to be caught by eye. →
  **Proposal 1.**
- **RECURRENCE — the e2e global-teardown downed the user's shared compose stack
  AGAIN** (this time during the orchestrator's own full-suite verify runs). `global-
  teardown.ts` unconditionally calls `takeRedisDown()`, which runs
  `docker compose -f docker-compose.yml down` against the **repo-root** compose file on
  every exit path — the same file a developer's shared dev stack comes up from — so
  running the suite tears the dev stack down. The reviewer restored it per its prompt
  instruction (a manual band-aid). No durable guard (keep-stack flag or dev-stack
  detection) exists. → **Proposal 2.**
- **1 flake (w2.2) in the orchestrator's final full-suite run, green in isolation** —
  the documented resource-contamination pattern (SKILL Phase 2.5 warns of exactly this:
  one container per test starves under co-running load). Handled correctly (re-run
  isolated, treated as non-genuine). Not a violation; confirms the isolation rule is
  load-bearing.
- **No true violations.** Test-fixer stayed task-blind (prompt = reproduce block +
  ledger path only); `block_test_edits.log` (f7adaf7d, 14:04–16:18 window) shows **zero**
  e2e-edit denies — it never reached for the driving specs. Off-limits tree + package
  `scripts` clean. Reviewer completed (hook-enforced, 16:44:58); no inline review. The
  fixer also greened a **pre-existing red w1.1b** (home-landing testid demanded by the
  existing w1 spec, swept in with the reproduce set) — legitimate task-blind behavior;
  orchestrator flagged it, reviewer verified it live. **retro #1's `-o` collision guard
  HELD** — it is durably present in `codex-loop.md:60-63`, and this run's ledgers show
  no empty-prompt/collision (the fixer used distinct `-rN-header/footer` prompt files +
  distinct answer paths). No recurrence.

### Positive: two handshakes that COLLAPSED in retro #1 worked cleanly here

- **R7 escalation handshake worked as designed** (contrast retro #1's collapsed
  escalation). The planner detected a genuine 3-source design conflict (scenarios page:
  static TABLE vs `/requirements` FileView), did NOT silently pick, and escalated. The
  orchestrator resolved it from the captured evidence (`tmp/proto-reference/scenarios.png`
  + the live prototype route), **overrode plan case w13.4 to the table reading**, and
  logged the ruling in the Workflow log (2026-08-04T14:35Z) BEFORE the task-blind fixer
  ran. Model "planner escalates → orchestrator decides from evidence → logged" round-trip.
- **The regeneratability handoff (retro #1's Proposals 2+3, now auto-applied into the
  briefs) fired end-to-end** — see Regeneratability.

## Codex loop efficiency

- **Planner — 28 keeps / 3 rounds, high value with a ~5-keep churn tail.** The genuine
  fresh findings are what make the red specs precise: F2 (R7 conflict surfaced), F4
  (small-caps `text-transform`+`--ls-label`), F5 (file-card header-bar anatomy), F6
  (`--text-muted` token), F8 (unaligned linked title), F9 (pill label/value/change
  anatomy), F10 (skip discipline: only-codex-skips), F11–F15 (prototype boot mechanics),
  F17 (edit round-trip), n2 (pill→relay persistence), n3 (description equality), n4
  (line-count formula off-by-one), n5 (successful-boot teardown), n7 (arch-leak negative
  guard). But **~5 of the 28 keeps were re-fixes of the planner's OWN incompletely-applied
  round-1 edits** — F1→F1r, F10→F10r, F18→F18r→F18r2 (production-guidance removal took
  **three** rounds to fully purge), n6 — where the primary edit landed but a contradicting
  clause was left in Challenges/Startup for Codex's "verify applied" job to re-flag. That
  is self-inflicted churn, not fresh signal. → **Proposal 3.**
- **Test-author — 9 keeps / 3 rounds, clean.** Principled, stable skips (F2/F5/F7 upheld
  across all three rounds); each keep tightened a real weakness (RED-first reordering,
  every-registry-row coverage + empty-state, left/right geometry via boundingBox, exact
  `toHaveText`, visibility gate before box reads). No churn.
- **Test-fixer — 2 keeps / 3 fixed rounds.** R1 kept F3 (feature-detail row exact-line
  jump wiring) and correctly scope-skipped F1/F2 (raw-px / rgba that PRE-DATE the diff and
  live under `design_system`, outside its edit permission — both re-challenged and upheld
  across all 3 rounds with fresh corroborating evidence each time). R2 kept n1 (title font
  token `--fs-h3`) and empirically REFUTED n2 (traced `tokens.css:99 body{font:...}`
  inheritance — Codex's own grep had missed the file). R3 kept 0 (mandated-dry). Efficient
  reasoning; the only spend-without-contribution is the fixed-3 R3.
- **Reviewer — 4 keeps / 3 rounds, escalation-free, ZERO net regeneration-loss.** R1 kept
  the link-role hardening (w13.4) + the jump pin (w14.2) and first RECORDED the breadcrumb
  fallback as accepted-loss (R3); R2 then **overturned its own accepted-loss** on corrected
  evidence (StoryPage renders not-found INSIDE the shell → the fallback IS reachable) and
  added w13.5, plus `toHaveRole("link")`. R3 steady-state (0 keeps). The self-correction is
  the audit working as intended.

Net: 43 keeps across the full 12 rounds. Two mandated/structural zero-yield rounds (fixer
R3, reviewer R3) and a ~5-keep planner churn tail are the only non-contributing spend; the
rest is durable spec-hardening.

## Regeneratability

Standing lens: **the committed `tests/harness/` e2e suite is the spec; `harness/src`
(production + its unit tests) is gitignored, regenerated-from-tests build artifact.**

**This run is the headline regeneratability WIN of the redesign so far: ZERO accepted
regeneration-losses.** Retro #1 identified the leak class (fixer-added behavior pinned
only by gitignored unit tests, with no channel to promote it) and proposed the reviewer
regeneratability audit (P2) + the fixer "surface unpinned behavior" output (P3). Both were
auto-applied into the briefs — and this run they **fired end-to-end**:

- The fixer's new output listed its unpinned addition — the feature-detail
  requirement/unaligned exact-line **jump** wiring (`requestJump`/`lineOfMapKey`) — which
  the orchestrator relayed to the reviewer (Workflow log 16:20Z).
- The reviewer's regeneratability audit then **promoted every unpinned behavior to the
  durable e2e suite**:
  - **w14.2** — the exact-line jump (was pinned by nothing) → committed e2e for both row
    kinds (click row link → scenarios page → `file-view-flash` at the scenario's line).
  - **w13.4 hardening** — the scenario-id link (was TEXT-only; a regen could degrade it to
    a `<span>`) → `toHaveRole("link")` + href pinned for every row.
  - **w13.5** — the unknown-story breadcrumb FALLBACK (was pinned only by the gitignored
    `breadcrumb.test.ts`) → committed e2e once the reviewer proved the fallback is reachable
    in-shell.

Contrast retro #1, where the equivalent unit-only behaviors (F3 non-routable crumb, F4
`safeDecode`) were LEFT as durable leaks with no way to promote them. **The retro system
demonstrably closed its own loop:** retro #1 diagnosis → auto-applied briefs → this run
verifies the leak is gone.

- **Did the fixer need knowledge the tests didn't carry?** No — task-blindness held; it
  derived every UI change from the red specs and read prod source, never the brief.
- **Residual (recorded, not a loss):** the scenario-id link is an inert self-link to
  `/product/scenarios` (prod doesn't jump on it; the prototype's `/scenarios` is a static
  leftover — no clear behavior to mirror). Reviewer recorded it in residual risk.
- **Structural note:** the R7 override created a genuine spec tension on one route —
  w13.4 (overridden to a table) vs the immutable already-green w5.1/w5.1a/w5.3 (which
  demand a live-editable `file-view-editor` on the SAME `/product/scenarios` route). The
  fixer's resolution is a **dual `scenario-table` + `FileView` surface**, the only shape
  satisfying both spec sets. Both sets are committed, so it regenerates deterministically —
  but it is a non-obvious coupling worth remembering: an orchestrator override that
  contradicts an existing green spec forces a compound production surface.

## Proposals

### Proposal 1 — Extend the FOREGROUND Codex rule to the planner, test-author, and reviewer (close the recurring premature-stop window)

**Evidence (this run).** The planner premature-stopped twice mid-loop, forcing
process-watch + SendMessage babysitting; its brief still carries the background+Monitor
pattern retro #1 already diagnosed and fixed **for the test-fixer only**. The test-author
and reviewer carry the identical latent pattern and ran clean only because the orchestrator
hand-patched foreground discipline into their spawn prompts — per-run compensation. The
fixer, whose brief WAS fixed, ran a single clean turn through all 3 rounds. The fix is
proven; it just needs to cover the other three agents. Mirror the fixer's exact language.

```diff
--- a/.claude/agents/implement-task-planner.md
+++ b/.claude/agents/implement-task-planner.md
@@ ## Status
 
-Print start, each round N/3 with kept/skipped counts, and plan written. Monitor each
-Codex round with bounded `Monitor` until it exits; do not end the turn while it runs.
+Print start, each round N/3 with kept/skipped counts, and plan written. Run every Codex
+round as a single **foreground/blocking** Bash invocation (no `run_in_background` + a
+future-turn `Monitor`, a generous `timeout`) — a blocking call cannot return until Codex
+exits, so the turn physically cannot end mid-round. **Never end your turn with a Codex
+round still in flight** — if one is running you are not done; there is no valid reason to
+yield mid-round.
```

```diff
--- a/.claude/agents/implement-task-test-author.md
+++ b/.claude/agents/implement-task-test-author.md
@@ ## Status
 
 Print start, each Codex round N/cap, and the run result with readiness and
-passed/failed counts. Monitor each Codex round with bounded `Monitor` until it exits;
-do not end the turn while it runs.
+passed/failed counts. Run every Codex round as a single **foreground/blocking** Bash
+invocation (no backgrounding), and wait out every background test run, before you
+continue. **Never end your turn with a Codex round or a test run still in flight.**
```

```diff
--- a/.claude/agents/implement-task-reviewer.md
+++ b/.claude/agents/implement-task-reviewer.md
@@ ## Status
 
 Print start, each round N/cap with kept/skipped counts, CLI and browser smoke
-pass/fail, and final pass/fail. Monitor each Codex round with bounded `Monitor` until
-it exits; do not end the turn while it runs. Inspect images only when required for
-a decision, preferring one composite check.
+pass/fail, and final pass/fail. Run every Codex round as a single **foreground/blocking**
+Bash invocation (no backgrounding), and wait out every background run, before you
+continue. **Never end your turn with a Codex round still in flight.** Inspect images only
+when required for a decision, preferring one composite check.
```

### Proposal 2 — Give the e2e global-teardown a keep-stack guard so suite runs stop downing the user's shared dev stack

**Evidence (this run).** The teardown bit AGAIN — this time during the orchestrator's own
full-suite verify runs — and the reviewer had to restore the stack by hand. `global-
teardown.ts` → `takeRedisDown()` runs `docker compose down` against the repo-root compose
file unconditionally. The **durable** fix is a one-line env guard in `redis.ts`; the
orchestrator's own runs then opt into keeping the stack. The `redis.ts` change is outside
the `.claude/*` auto-apply surface, so it is **user-applied**; the SKILL diff (auto-applied)
makes the orchestrator set the flag before its own isolated runs.

_User-applied companion (test-harness code — the actual guard):_
```diff
--- a/tests/harness/helpers/redis.ts
+++ b/tests/harness/helpers/redis.ts
@@ /** Stops the Redis compose stack. Called once from global-teardown (best-effort). */
 export async function takeRedisDown(): Promise<void> {
+  if (process.env.TRUE_BDD_E2E_KEEP_STACK === "1") {
+    console.log("[harness-e2e] TRUE_BDD_E2E_KEEP_STACK=1 — leaving the compose stack up");
+    return;
+  }
   const file = composeFile();
   try {
     await execFileAsync("docker", ["compose", "-f", file, "down"], {
```

_Auto-applied (.claude) — teach the orchestrator's own runs to opt in:_
```diff
--- a/.claude/skills/implement-task/SKILL.md
+++ b/.claude/skills/implement-task/SKILL.md
@@ Phase 2 — step 5, the isolation paragraph
      Run the e2e suite in **isolation** — never concurrently with another suite,
      `docker compose` stack, or heavy build. The harness launches one container per
      test, so a co-running load starves it and produces resource-contamination flakes
      that masquerade as real failures; re-run any failure isolated before treating it
      as genuine.
+     The suite's global-teardown `docker compose down`s the **repo-root** compose stack
+     on every exit — which tears down a developer's shared dev stack running from the
+     same file. Export `TRUE_BDD_E2E_KEEP_STACK=1` for your own suite runs (and tell the
+     reviewer to) so the teardown leaves the stack up; otherwise restore it afterward.
```

### Proposal 3 — Planner: apply each kept finding COMPLETELY — purge every contradicting clause across the whole plan in the same round

**Evidence (this run).** ~5 of the planner's 28 keeps were re-fixes of its own
incompletely-applied round-1 edits — F1→F1r, F10→F10r, F18→F18r→F18r2 (removing production
guidance took **three** rounds), n6 — each because the primary edit landed but a
contradicting clause survived in Challenges/Startup for Codex's "verify applied" job to
re-flag next round. That is round-spend on churn, not fresh signal. A one-line discipline
in the planner's apply step turns those multi-round drips into single-round fixes.

```diff
--- a/.claude/agents/implement-task-planner.md
+++ b/.claude/agents/implement-task-planner.md
@@ 3. Run the Codex critique loop for at most 3 rounds, following paths.yaml. Send the
    full task, plan, and all prior findings each round. Include questions about
    Playwright coverage, assertion strength, flakiness, and whether tests fail when
    behavior is broken. Codex is read-only and never edits. You score every finding;
-   keep only composite ≥7 with all four gates satisfied.
+   keep only composite ≥7 with all four gates satisfied. **Apply each kept finding
+   COMPLETELY in the round you accept it: after the primary edit, sweep the WHOLE plan
+   (Current/Target state, Challenges, Startup, End-to-end cases) and purge every
+   contradicting clause — a stale clause left elsewhere makes the next round's "verify
+   applied" pass re-flag the same fix (this run spent ~5 keeps across rounds 2–3 that
+   way; F18 took three rounds to fully remove production guidance).**
```

_Not proposed:_ the fixed-3 fixer R3 (0 keeps) and the reviewer R3 (0 keeps) are
policy-mandated / natural steady-state, not waste to remove. `total_tokens` /
`total_duration_ms` remain `null` in the metrics schema — the standing capture gap from
retro #1, still out of this analyst's edit scope, still flagged for the user.
