# Codex review loop

Shared by the `test-author` and `fixer` agents. **codex is the read-only REVIEWER;
crush is the only WRITER; the driver agent organizes the loop and SCORES.** Take the
wrapper, artifact dir, and prompt templates from `docs/context/paths.yaml` (the
`codex_*` and `crush_*` entries); never hardcode. codex invocation mechanics (anti-hang
flags, wrapper, Playwright): `codex_mechanics`.

## The cap: `codex_cap`

The caller passes each agent a `codex_cap` ∈ {`0`, `1`, `3`, `5`} — the number of
codex↔crush review cycles to run. `0` = no review at all (the crush work round is the
whole job). There are no lanes and no plan; the cap is the only knob.

## What every codex call contains (full context, every round)

Every round is a FULL, independent review — never a narrow delta. Each codex prompt
carries, in full:

1. **The task.** test-author: the requirements list + the reconcile / expected-RED
   plan. Fixer (task-blind): the reproduce block + the e2e specs it is greening (read
   from disk) — never a brief or plan.
2. **All current changes** — the complete current diff under review (specs for the
   author; production + unit code for the fixer), not just what changed since last round.
3. **All prior findings** (rounds 1 … N−1) so codex doesn't repeat itself.
4. **The disposition of every prior finding** (round 2 on) — applied (where/how) or
   skipped (the one-line reason) — so codex can audit round N−1.

The `*_review` templates under `paths.yaml → codex_prompts` bake these four slots in.

## codex does NOT score

Round 1: fresh findings only. Round 2+: (a) verify every APPLIED finding is correctly
and completely implemented; (b) challenge every SKIP — does the reason still hold?;
(c) fresh findings. Everything returns as findings with evidence and a concrete fix —
no scores, no ranking, no severity. codex is read-only and runs commands to verify its
own claims.

## The AGENT scores (composite ≥7 + four gates)

For each finding assign ONE composite relevance score 1–10 and run four pass/fail
gates; keep it only if **composite ≥7 AND every gate passes**:

- **Correctness** — technically right, not merely asserted.
- **Evidence** — grounded in files/tests codex actually ran, not memory.
- **Scope fit** — inside the task's intent (the requirements, or the reproduce block).
- **Regression risk** — does not break existing behaviour or the only-expected-red
  invariant.

Record each kept item's composite + gate results + one-line justification in the ledger.

## The cycle (repeat up to `codex_cap` times)

1. **Fill the review template** (`codex_prompts.*`) with the four slots above, write it
   to a prompt file under `codex_artifacts` — give the prompt file and the wrapper's
   answer file DISTINCT paths (a shared path overwrites the prompt with an empty file
   and wastes the launch) — and run `<codex_wrapper> ro <prompt-file> <label>`
   **foreground/blocking** (a blocking call cannot return mid-round, closing the
   premature-stop window). Status line when you launch: `codex round N of <cap> …`
   (don't echo the prompt contents).
2. **Score** each finding (composite + gates above); keep the passes.
3. **crush applies the keeps** — fill the matching `*_apply_review` crush template with
   the kept findings and pipe it to `<crush_wrapper> <role> - <label> --continue` (the
   SAME crush session). crush applies ONLY those, then re-runs ALL tests and confirms
   only the expected specs are red (author) / the suite is fully green (fixer), and
   refreshes `result.json`.
4. **Record the round** in the ledger (`paths.yaml → codex_ledger`): prompt file,
   answer file, each finding's composite + gates + keep/skip.

Stop at `codex_cap` cycles, or earlier if a round is **dry** — every prior application
verified clean, no skip-challenge survives scoring, and no fresh finding passes the
gates. `codex_cap = 0` skips the loop entirely.

## If a spec is impossible to satisfy in code (fixer)

The fixer NEVER edits a driving e2e/BDD test. If a spec genuinely cannot be satisfied
in code, STOP and escalate to the caller with evidence — the failing assertion, the
architectural reason it can't hold, and what was tried — rather than weakening the test.
