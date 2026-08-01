# Codex critique loop

Shared by the four `implement-task` agents. **Take the wrapper path, artifact
directory, and invocation usage from `docs/context/paths.md` (Codex section); do not
hardcode them.**

## How EVERY Codex call works (non-negotiable)

Every round is a FULL, independent review — never a narrow delta follow-up. Each
Codex prompt contains, in full:

1. **The entire task** — goal, all requirements, non-goals, constraints.
2. **All current changes** — the complete current state of the artifact under review
   (the whole plan, or the full current diff plus all relevant files), not just what
   changed since the last round.
3. **Everything Codex has found so far** — the accumulated findings from ALL prior
   rounds (round 1 … N−1), so Codex sees the full history and doesn't repeat itself.

**Codex does NOT score.** Ask Codex only for findings (gaps, defects, improvements),
each with evidence and a concrete fix. **The AGENT scores every finding itself** (see
Scoring below) and decides what to apply. Never let Codex's own ranking or severity
drive the decision.

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

Repeat up to **3 rounds**, stopping early when a round returns nothing that passes
the gates at composite ≥7:

1. **Write the prompt** — full task + all current changes + all prior-round findings
   (above) — to a prompt file under the Codex artifacts directory (paths.md). Ask
   Codex to run commands to verify its claims and to return findings only (no scores).
2. **Run the Codex wrapper** (path + usage in paths.md) as a **background** task and
   arm a Monitor that fires on exit. **Always read-only** — Codex suggests; the agent
   applies the keeps and runs the tests itself. Never let Codex edit files directly
   (that would bypass the scoring gate). The answer lands in the artifacts dir.
   **Status line (one, when you launch each round):** name the round, the cap, and the
   prompt file path — e.g. `Codex round 1 out of 3 is running in the background with
   prompt: ./tmp/codex-<label>-r1.md. I'll wait for it to complete.` Do not echo the
   prompt's contents. (The same shape applies to a blocker-consultation Codex call,
   minus the "out of N".)
3. **Score** each finding (composite + gates above); apply only the keeps. Skip the rest.
4. **Record the round** in the plan's "Codex rounds" ledger: prompt file, response
   file, and each finding's composite + gates + keep/skip.
5. **Stop** when a round yields no keeps, or after **3 rounds**.

Every prompt is grounded in the **task goal** and the **plan**. Apply keeps by editing
the artifact (plan / code / tests) — never silently; each change must be visible and
re-runnable.

## Blocker consultation (coder only — distinct from the loop above)

When the coder hits a test it cannot satisfy in code, do NOT run the relevance loop.
Make ONE focused read-only Codex call that must return a structured verdict:

- **CODE-FIX** — a concrete, executable code-only proposal (which file, which
  change), with a confidence level; OR
- **STOP — test must change** — the specific test, the architectural reason it can't
  be satisfied in code, and what a correct test would assert.

Pass the failing assertion, the relevant code, and what you already tried. The coder
acts on CODE-FIX; on STOP it returns the verdict to the orchestrator (Phase 2.4).

For full Codex mechanics (sandbox flags, Playwright-MCP setup, prompt guide) see the
mechanics doc in paths.md (Codex section).
