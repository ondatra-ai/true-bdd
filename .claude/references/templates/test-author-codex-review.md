<!--
CODEX PROMPT TEMPLATE — REVIEW SPECS (test-author, Step 2 review cycle).
The test-author DRIVER fills EVERY {{...}}, writes the result to a prompt file
under the codex artifacts dir (paths.yaml → codex_artifacts, e.g.
tmp/codex-ta-review-r<N>.md — give the prompt file and the wrapper's answer file
DISTINCT paths), and runs:
    <codex_wrapper> ro <prompt-file> ta-review-r<N>
codex is READ-ONLY: it finds problems and NEVER edits, and it returns findings
ONLY with NO scores — the DRIVER scores each (composite ≥7 + four gates) and
decides what to keep. Round 1 asks for fresh findings only; round 2+ also
verifies applied findings and challenges skips (fill {{PRIOR_FINDINGS}}).
Everything below `====` is the prompt.
====
-->

You are reviewing end-to-end Playwright specs a writer just produced. You are
READ-ONLY — do NOT edit any file. Run read-only commands to VERIFY every claim
(never review from memory).

# The task these specs must pin

{{REQUIREMENTS}}

# The authoring plan (what should be RED, and why)

{{RECONCILE_AND_EXPECTED_RED}}

# What to review

- spec directory: `{{E2E_DIR}}`
- the writer's changes — review this diff: run `{{DIFF_CMD}}`
- the writer's current run results (verify red/green against THESE — you are
  READ-ONLY and cannot start the suite's webServer, so do NOT re-run it; read
  the run log at `{{LOG_PATH}}` for more detail):

  {{RUN_RESULTS}}
- typecheck (read-only, no writes): `{{TSC_CMD}}`

# Prior findings + how the driver disposed of them  (EMPTY on round 1)

{{PRIOR_FINDINGS}}

# Your jobs, in order

- **Round 1:** fresh findings only.
- **Round 2+:** (a) verify every APPLIED finding is correctly and completely
  implemented in the current specs; (b) challenge every SKIP — does the recorded
  reason still hold?; (c) fresh findings.

# What to look for (e2e-authoring review dimensions)

1. **Coverage** — is every requirement pinned by exactly one focused `test()`? Any
   requirement left unpinned, or one test conflating several requirements?
2. **Assertion strength** — would each test actually FAIL if the behaviour were
   broken or absent? Flag tests that pass regardless (tautologies, over-broad
   matchers, asserting mere presence when the requirement is about a value/state).
3. **Intent, not implementation** — do the specs assert user-observable INTENT,
   never implementation details, internal timings, or private structure?
4. **Right-reason RED** — is each intended-red spec red because of an assertion on
   the absent/changed behaviour — not a crash, collection error, timeout, or a bad
   selector that would fail even once the behaviour exists?
5. **Only-expected-red** — does anything make a spec OUTSIDE the expected-RED set
   fail (a collateral regression)?
6. **Flakiness** — waits on state (not `sleep`); deterministic selectors; no
   race-prone ordering or shared-state coupling between tests.
7. **testid contract** — every new testid registered as a contract entry only (no
   behaviour smuggled into the contract); locator conventions matched.

# Output — findings ONLY, no scores, no ranking

For EACH finding return:

- `id` — short stable slug.
- `location` — spec file + `test()` name (and line where useful).
- `problem` — one sentence: what is wrong.
- `evidence` — the command you ran and what it showed (or the exact offending lines).
- `fix` — the concrete, minimal change the writer should make.
- `category` — one of: coverage | assertion-strength | intent-vs-impl |
  wrong-reason-red | collateral | flakiness | testid-contract.

Do NOT score, rank, or assign severity — the driver scores each finding and decides
what to apply.
