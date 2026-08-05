# Codex critique loop

Shared by the four `implement-task` agents. **Take the wrapper path, artifact
directory, and invocation usage from `docs/context/paths.yaml` (the codex_* entries); do not
hardcode them.**

## How EVERY Codex call works (non-negotiable)

Every round is a FULL, independent review — never a narrow delta follow-up. Each
Codex prompt contains, in full:

1. **The entire task** — goal, all requirements, non-goals, constraints.
   (Test-fixer exception: it is task-blind and has no brief — its "task" is the
   reproduce block plus the e2e specs it is greening, read from disk. Ground its
   rounds in those; never fetch the brief or plan to fill this slot.)
2. **All current changes** — the complete current state of the artifact under review
   (the whole plan, or the full current diff plus all relevant files), not just what
   changed since the last round.
3. **Everything Codex has found so far** — the accumulated findings from ALL prior
   rounds (round 1 … N−1), so Codex sees the full history and doesn't repeat itself.
4. **The disposition of every prior finding** (from round 2 on) — applied (where and
   how) or skipped (the one-line reason from the ledger). Without this, Codex cannot
   audit what you did with round N−1.

**Codex does NOT score.** From round 2 on, ask Codex for three things, in order:
(a) **verify every applied finding** — is the fix correctly and completely
implemented in the current state?; (b) **challenge every skip** — does the recorded
reason hold?; (c) **fresh findings** (gaps, defects, improvements). Round 1 is (c)
only. Everything returns as findings with evidence and a concrete fix — no scores.
**The AGENT scores every finding itself** (see Scoring below) and decides what to
apply. Never let Codex's own ranking or severity drive the decision.

## Scoring (done by the AGENT, not Codex)

For each Codex finding assign ONE composite **relevance score 1–10** and run four
**pass/fail gates**. Keep it only if **composite ≥7 AND every gate passes**:

- Gates (any FAIL → reject, regardless of score):
  - **Correctness** — technically right, not merely asserted.
  - **Evidence** — grounded in the files/tests (Codex ran commands to verify), not memory.
  - **Scope fit** — inside the task's goal and non-goals.
  - **Regression risk** — does not break existing behavior.

Reject anything that conflicts with the task's constraints or non-goals even if it is
"relevant." Per kept item, record the composite, each gate result, and a one-line
justification.

## The loop

Repeat up to the phase's round cap, stopping early when a round is dry (step 5:
applications verified, skips unchallenged, nothing new passes the gates).
**The cap comes from the task's lane** —
see `references/complexity-matrix.md` (tiny: review-only 1 round; easy: 1 per
phase that has one; hard: ≤3). **Exception: the test-fixer runs a FIXED 3 rounds
at every lane — no early exit; step 5's dry-stop does not apply to it.** Where
this doc says "3 rounds" it describes the hard-lane cap:

1. **Write the prompt** — full task + all current changes + all prior-round findings
   + their dispositions (the four ingredients above) — to a prompt file under the
   Codex artifacts directory (paths.yaml). Give the prompt file and the wrapper's
   `-o` answer file DISTINCT paths — a shared path overwrites the prompt with an
   empty file and wastes the launch (this bit the reviewer's first round in the
   design-conformance-tests run). Ask for the three jobs in order (verify
   applications / challenge skips / fresh findings), tell Codex to run commands to
   verify its claims, and to return findings only (no scores).
2. **Run the Codex wrapper** (path + usage in paths.yaml) as a **background** task and
   arm a Monitor that fires on exit. **Always read-only** — Codex suggests; the agent
   applies the keeps and runs the tests itself. Never let Codex edit files directly
   (that would bypass the scoring gate). The answer lands in the artifacts dir.
   **Test-fixer exception — run Codex in the FOREGROUND.** The test-fixer invokes the
   wrapper as a single **blocking** Bash call (no `run_in_background`, a generous
   `timeout`), never background+Monitor. A blocking call cannot return until Codex
   exits, so the turn physically cannot end mid-round — closing the premature-stop
   window that forced four manual resume nudges in the design-conformance-tests run.
   Wait out background test runs the same way before yielding.
   **Status line (one, when you launch each round):** name the round, the cap, and the
   prompt file path — e.g. `Codex round 1 out of 3 is running in the background with
   prompt: ./tmp/codex-<label>-r1.md. I'll wait for it to complete.` Do not echo the
   prompt's contents. (The same shape applies to a blocker-consultation Codex call,
   minus the "out of N".)
3. **Score** each finding (composite + gates above); apply only the keeps. Skip the rest.
   **Confirm each application before recording:** re-read the changed lines against the
   finding and, for a multi-part finding (e.g. "validate EVERY field", "assert BOTH
   directions", two waiters/locators), check off each part. A half-applied keep caught
   here costs a re-read; caught by Codex next round it costs a whole capped verify slot.
   This does not replace the next round's independent verification — it lowers the botch
   rate feeding it.
4. **Record the round** in the Codex rounds ledger file — `<slug>.codex.md` beside
   the plan (paths.yaml → codex_ledger) — prompt file, response file, and each finding's
   composite + gates + keep/skip. The ledger lives OUTSIDE the plan so downstream
   agents reading the plan don't pay for it; the plan's "Codex rounds" section is
   just a one-line pointer to this file.
5. **Stop** when a round is **dry** — every prior application verified clean, no
   skip-challenge survived scoring, and no fresh finding passed the gates — or at
   the lane's round cap. A round that flags a botched application is never dry:
   fix it and it counts as an applied finding for the next round to verify.

Every prompt is grounded in the **task goal** and the **plan**. Apply keeps by editing
the artifact (plan / code / tests) — never silently; each change must be visible and
re-runnable.

## Blocker consultation (test-fixer only — distinct from the loop above)

When the test-fixer hits a test it cannot satisfy in code, do NOT run the relevance
loop. Make ONE focused read-only Codex call that must return a structured verdict:

- **CODE-FIX** — a concrete, executable code-only proposal (which file, which
  change), with a confidence level; OR
- **STOP — test must change** — the specific test, the architectural reason it can't
  be satisfied in code, and what a correct test would assert.

Pass the failing assertion, the relevant code, and what you already tried. The
test-fixer acts on CODE-FIX; on STOP it returns the verdict to the orchestrator
(Phase 2.4).

For full Codex mechanics (sandbox flags, Playwright-MCP setup, prompt guide) see the
mechanics doc in paths.yaml (codex_mechanics).
