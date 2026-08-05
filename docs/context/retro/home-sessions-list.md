# Task retro — `home-sessions-list`

Lane **hard**. Replace the harness root `/` placeholder with a live sessions
list (P1–P10) and retire the pre-workspace contract (H1) in the same step.
Clean run: 4 phases, each spawned once and completed via `subagent-stop`; zero
phase-state denies, zero blocks, zero escalations, zero off-limits edits. Every
authored behavior shipped GREEN (tsc clean, w18+w19 13/13, p1+p2 2/2, unit
79/79, live CLI+browser smoke PASS).

## Run summary

Durations from `active.json` UTC spawn→completion. Tokens are not recorded
per-phase (no per-agent token accounting in phase state; the metrics line is not
appended until `close`, which runs after this retro) — duration is the cost proxy.

| Phase | spawns / completions | Codex rounds (keeps) | tokens | duration |
|---|---|---|---|---|
| Planner (Opus) | 1 / 1 | 3 — 12 / 6 / 7 keeps (2 botched re-fixes, 1 overclaim purge, FF1 skip held ×2) | n/r | 30m16s |
| → orch. scope-verify | — | — | — | 28s |
| Test-author (Opus) | 1 / 1 | 3 — 2 / 3 / 2 keeps (1 skip held ×2; 2 re-fixes R3) | n/r | 37m17s |
| → orch. scope-verify + snapshot | — | — | — | 41s |
| Test-fixer (Sonnet, task-blind) | 1 / 1 | fixed 3 — 3 / 2 / 1 keeps (+1 discarded round: prompt/answer path collision) | n/r | 48m06s |
| → orch. gate re-run (tsc/unit/lint/full-project + isolate flakes + diff) | — | — | — | 11m14s |
| Reviewer (Opus) | 1 / 1 | 3 — 7 applied, R3 DRY (1 re-fix R2) | n/r | 26m38s |
| **Total** (start → reviewer done) | 4 / 4 | **12 scored (+1 discarded)** | n/r | **2h35m30s** |

Enforcement audit: `phase-state.log` — 4 `allow` spawns + 4 clean stops, no
deny. `block_test_edits.log` — every entry `allow`; the test-fixer's hard block
was never tripped (it touched only `page.tsx`, `lib/sessions/poll.ts`,
`sessions-poll.test.ts`, `globals.css`). Manifests: production-before ==
production-after-author (test-author touched zero production code);
offlimits-fixer-before == offlimits-fixer-after (test-fixer touched zero
off-limits e2e). Change surface: 13 files / 1808 lines.

## What cost the most

1. **Test-fixer, 48m — the single longest phase.** ~29m elapsed before its first
   production edit (07:21 spawn → 07:50 first Edit): spec-reading, red-baseline
   reproduce, and the discarded collision round (below). The fixed-3-rounds
   mandate (no early exit) paid off here — round 3 caught a genuine
   `min-height`→`height` scroll-container bug in its own CSS (finding b), not mere
   confirmation — so this is defensible cost, not waste.
2. **Orchestrator inter-phase gate re-run, 11m, inflated by unrelated repo state.**
   Running the FULL workspace project (79 tests) surfaced w4/w11 **contention**
   flakes (passed isolated) that cost an isolation re-run + analysis, and the
   pre-existing **1735-line** design-prototype lint sweep had to be traced to
   confirm the new files were clean. Both are non-task noise the DoD's "final
   suite … all passing" invited.
3. **Botched Codex applications — the most repeated inefficiency, ~7 across all
   four phases.** Planner (F3, F4, V3), test-author (F1′, F3′), test-fixer (pid
   validation), reviewer (characterData) each claimed a keep applied that the
   NEXT round's Codex verification found half-applied. In a capped-3 hard lane,
   each self-inflicted miss consumes a whole verify slot that could have gone to
   fresh findings.
4. **Codex prompt/answer path collision (test-fixer R1).** The first invocation
   passed the prompt file and the `-o` answer path as the same path; the prompt
   artifact was overwritten and the round was discarded and redone with distinct
   `-r1-prompt.md`/`-r1.md`. One wasted launch — and codex-loop.md ALREADY
   forbids this (the rule was added by a prior retro), so it is an unenforced-rule
   recurrence, not a new lesson.

## Deviations & violations

- **No protocol violations.** Phase order, task-blind fixer prompt (reproduce
  block + ledger path only), separate reviewer agent, orchestrator-run suite —
  all honored. Scope gates clean on both manifests.
- **Improvisation beyond SKILL.md (diligence, not drift):** the orchestrator
  re-ran the full workspace project and manually traced the 1735-line lint noise
  and the w4/w11 contention flakes — beyond the letter of the DoD, and the source
  of most confirmation cost (see cost #2).
- **codex-loop distinct-paths rule unenforced (root-caused).** The test-fixer R1
  collision AND the reviewer's own rounds both diverge from codex-loop.md's
  "give the prompt and `-o` DISTINCT paths": the reviewer deliberately used ONE
  combined prompt+answer path per round (`…-reviewer-rN.md` is "also the answer
  file"), tolerated because stdin is consumed before `-o` writes. The wrapper
  `codex.sh` has no guard, so the rule holds only by caller memory — and this run
  shows it does not hold. (Proposal 2.)

## Codex loop efficiency

- **Contribution was real, not confirmation.** Planner's 25 keeps materially
  hardened P4/P5/P8/P9/P10 determinism and stripped F12's implementation-steering
  from the scaffolding (protecting the task-blind fixer). Reviewer's 7 keeps ALL
  landed as e2e hardening (w18 → 12 cases). Only the reviewer's R3 was DRY; every
  other capped round was cap-bound, not dry — so the hard-lane cap, not
  early-exit, set the round count.
- **Skip discipline was sound.** FF1 (blank stale rows in the 0–30s grace window)
  was skipped on Scope-fit and HELD across two separate challenges (planner R2/R3)
  — correctly refusing to invent a grace-period UX the brief doesn't mandate. The
  test-fixer's three R1 skips (tie-breaker, CSS-token literals, aria-live) were
  each well-grounded (the tie-break is deliberately untestable per w18.7's own
  comment; the literals are verified 1:1 prototype-source copies with no matching
  token) and all held under challenge.
- **The tax was self-inflicted partial applications** (cost #3), not Codex
  mis-scoring. The loop's design (next round independently verifies applications)
  worked — it just paid full-round price to catch misses a re-read would have
  caught for free.

## Regeneratability

Standing lens: `harness/src/**` — production code AND its unit tests — is
gitignored (confirmed: `git check-ignore` matches `page.tsx`, `poll.ts`, and
`sessions-poll.test.ts`). Only the committed e2e suite survives a
regenerate-from-tests.

- **The task-blind fixer needed no knowledge the tests didn't carry.** It derived
  the whole home page from the red specs alone; the discarded round aside, it
  stayed in-lane and never reached for the brief.
- **The flag → re-pin pipeline WORKED, and it is the run's most important
  durability outcome.** The fixer added three robustness behaviors no e2e test
  demanded — hung-read `AbortController` timeout, malformed-200 payload
  validation, unmount-abort — and flagged them (its agent mandate). The reviewer's
  mandated regeneratability audit then **re-pinned the two observable ones in
  durable e2e**: `w18.11` (hung read still surfaces the unavailable notice) and
  `w18.12` (malformed 200 ≠ empty state). Without those, a naive
  `body.sessions ?? []` regen would show "No sessions connected" on garbage and a
  no-timeout regen would stall the poll loop forever — both now caught by
  committed specs.
- **What remains unpinned (accepted regeneration-loss, reviewer finding 6):**
  unmount-abort cancellation, the 10s `SESSIONS_REQUEST_TIMEOUT_MS` value, the 3s
  poll interval, `firstFailureAt` bookkeeping, and sort immutability internals.
  These are genuinely e2e-unobservable in this Next.js/React-18 stack
  (setState-after-unmount is no longer a console warning), so accepting their loss
  is correct, not a gap. On a fresh regen the OBSERVABLE app reproduces in full;
  only these internal niceties may differ.
- **Gap that survived:** the fixer already NAMED hung-read and malformed-200 in
  its return, yet the reviewer had to REDISCOVER them via its own audit because
  the Phase-3 reviewer prompt (slug + plan + diff) never forwards the fixer's
  unit-only-behavior list. The pipeline succeeded on reviewer diligence, not on a
  wired handoff. (Proposal 1.)

## Proposals

### 1. Wire the test-fixer's unit-only-behavior list into the reviewer's prompt

**Problem (this run):** the test-fixer's agent output already lists "production
behavior … pinned only by your (gitignored) unit tests" (its mandate) — here
hung-read timeout, malformed-200 validation, unmount-abort. But Phase 3 step 7
builds the reviewer prompt from `slug + plan path + diff path` only, so the
reviewer's regeneratability audit **rediscovered** hung-read and malformed-200
from scratch (reviewer findings 1 and 5) instead of starting from the known list.
The re-pins (w18.11/w18.12) happened by reviewer diligence, not by design. Forward
the list so the audit is seeded, not lucky.

```diff
--- a/.claude/skills/implement-task/SKILL.md
+++ b/.claude/skills/implement-task/SKILL.md
@@ Phase 3 — Review
-7. Spawn `implement-task-reviewer` (Opus) with a **minimal prompt**: the `slug:` line,
-   the plan path, and the **diff artifact path/command** — the true content diff is
+7. Spawn `implement-task-reviewer` (Opus) with a **minimal prompt**: the `slug:` line,
+   the plan path, the **diff artifact path/command**, and the test-fixer's
+   **unit-only-behavior list** — the behaviors it reported (verbatim from its return)
+   as pinned only by its gitignored unit tests, or "none" — so the reviewer's
+   regeneratability audit starts from that known list instead of rediscovering it.
+   The true content diff is
```

Matching hunk so the reviewer expects the input (one coherent change, two ends):

```diff
--- a/.claude/agents/implement-task-reviewer.md
+++ b/.claude/agents/implement-task-reviewer.md
@@ ## Input
-The orchestrator provides `<slug>`, lane, plan path, recorded challenges, and the
-exact command/artifact producing the full task-attributable diff.
+The orchestrator provides `<slug>`, lane, plan path, recorded challenges, the
+exact command/artifact producing the full task-attributable diff, and the
+test-fixer's unit-only-behavior list (behaviors pinned only by gitignored unit
+tests, or "none"). Seed the regeneratability audit from that list, then extend it.
```

### 2. Make the collision impossible in `codex.sh`, not just forbidden in prose

**Problem (this run):** codex-loop.md already mandates DISTINCT prompt vs `-o`
paths (added by a prior retro), yet the test-fixer's R1 still collided them —
prompt overwritten, round discarded and redone. The reviewer's rounds diverge the
other way (deliberate combined path). The wrapper derives `out=./tmp/codex-<label>.md`
and never checks it against `prompt_file`, so the rule holds only by caller memory.
A 6-line realpath guard fails loud BEFORE burning the Codex call and regularizes
both agents onto distinct paths. (Flagged: `codex.sh` is a script, not SKILL.md /
agent.md — the enforcement point the doc rule needs.)

```diff
--- a/.claude/skills/implement-task/scripts/codex.sh
+++ b/.claude/skills/implement-task/scripts/codex.sh
@@ mkdir -p ./tmp
 out="./tmp/codex-${label}.md"
 trace="./tmp/codex-${label}.trace.log"
+
+# Refuse the prompt==answer collision: -o "$out" overwrites the prompt file.
+# The run still "works" (stdin is consumed before -o writes) but the prompt
+# artifact is destroyed, which has twice forced a discard+redo. Fail loudly
+# before burning a Codex call so the caller uses distinct paths
+# (e.g. <label>-rN-prompt.md in, <label>-rN.md out).
+if [ "$(cd "$(dirname "$out")" && pwd -P)/$(basename "$out")" = \
+      "$(cd "$(dirname "$prompt_file")" && pwd -P)/$(basename "$prompt_file")" ]; then
+  printf 'codex.sh: -o answer path (%s) equals the prompt file — use distinct paths\n' "$out" >&2
+  exit 3
+fi
```

### 3. Self-check each applied keep before recording the round

**Problem (this run):** ~7 keeps across all four phases were claimed applied but
found half-applied by the NEXT Codex round — pid-field validation (fixer R2),
`characterData` (reviewer R2), the P10 clock sequence (planner V3), the P8
render-race and gradient geometry gate (author R3). Each cost a full capped verify
slot. A cheap re-read of the changed lines against the finding — especially
multi-part findings — catches the miss for free without weakening Codex's
independent next-round verification.

```diff
--- a/.claude/skills/implement-task/references/codex-loop.md
+++ b/.claude/skills/implement-task/references/codex-loop.md
@@ ## The loop
-3. **Score** each finding (composite + gates above); apply only the keeps. Skip the rest.
+3. **Score** each finding (composite + gates above); apply only the keeps. Skip the rest.
+   **Confirm each application before recording:** re-read the changed lines against the
+   finding and, for a multi-part finding (e.g. "validate EVERY field", "assert BOTH
+   directions", two waiters/locators), check off each part. A half-applied keep caught
+   here costs a re-read; caught by Codex next round it costs a whole capped verify slot.
+   This does not replace the next round's independent verification — it lowers the botch
+   rate feeding it.
```

*(No proposal on the fixed-3 test-fixer mandate, the lane call, or the regen-audit
directive — all three fired correctly this run: fixed-3 caught a real R3 CSS bug,
hard was the right lane with no escalation, and the audit re-pinned the two
regeneration-critical behaviors.)*
