<!--
CRUSH PROMPT TEMPLATE — WRITE TESTS (test-author, Step 1).
The test-author DRIVER fills EVERY {{...}} (resolving the concrete paths from
docs/context/paths.yaml first) and pipes the result into:
    <crush_wrapper> author - author-run<X>
Crush knows NOTHING about this repo's layout, conventions, or tooling — every
path and rule it needs is inlined below. Never leave a {{...}} unfilled; never
pass a paths.yaml KEY (resolve it to the real path). Lines starting with this
comment block are stripped by the driver; everything below `====` is the prompt.
====
-->

You are writing end-to-end Playwright specs for this repository. Do EXACTLY the
five steps below and NOTHING else. You may write files ONLY under `{{E2E_DIR}}`
(your sandbox) — writes anywhere else are blocked.

# The requirements to pin (one `test()` per requirement)

{{REQUIREMENTS}}

# Where everything is (inlined — do not go looking elsewhere)

- e2e spec directory (your sandbox): `{{E2E_DIR}}`
- read these existing specs FIRST and copy their conventions — spec naming &
  numbering, per-test environment/fixtures, viewport, and the oracle/locator
  patterns: {{EXAMPLE_SPECS}}
- testid / locator contract files — register every NEW testid here as a
  contract entry ONLY (never behaviour): {{TESTID_CONTRACT_FILES}}
- run the e2e suite with: `{{E2E_RUN_CMD}}`
  ALWAYS pass `--reporter=dot` (a chatty reporter deadlocks your shell); for a
  long run redirect `> {{LOG_PATH}} 2>&1` and then read that log.
- typecheck the e2e package with: `{{TSC_CMD}}` — drive it to ZERO errors.
- write your per-step docs and the final result under: `{{DOC_DIR}}`

# Steps (write ONE doc file per step into `{{DOC_DIR}}`)

1. **Baseline → `01-baseline.md`.** Read the existing specs and list, one line
   each, what every `test()` already pins. Run the FULL e2e suite and confirm it
   is GREEN. If it is ALREADY red, STOP now: write `result.json` with
   `status: "RED-BASELINE"` + the failing tail, and end. (You cannot author
   against a failing suite.)
2. **Reconcile → `02-reconcile.md`.** For each requirement decide **ADD** (no
   spec pins it yet), **UPDATE** (a spec pins it but the intended behaviour
   changed), or **DELETE** (an existing pin is no longer wanted). If a
   requirement CONTRADICTS an existing pin (both cannot hold), STOP: write
   `result.json` with `status: "CONFLICT"`, both statements, and the pinning
   spec. Otherwise write the **expected-RED list**: the specs your ADD/UPDATE
   will make fail.
3. **Author → `03-author.md`.** Write / update / delete the specs. One `test()`
   per requirement, asserting the INTENT a user observes — NEVER implementation
   details, internal timings, or private structure. Where a requirement has a
   healthy counterpart, add a green-guard test for it. Register every new testid
   in the contract files above. Drive `{{TSC_CMD}}` to zero errors.
4. **Verify RED → `04-red-verify.md`.** Run the FULL e2e suite again. The red set
   must be EXACTLY your expected-RED list — each red for the RIGHT reason (an
   assertion failure on the absent/changed behaviour, NOT a crash, collection
   error, or timeout), every green-guard green, and NOTHING else red anywhere. If
   it does not match, fix your specs and re-run until it does.
5. **Result → `{{DOC_DIR}}/result.json`.** Emit this exact shape:

   ```json
   {
     "status": "OK",
     "reconcile": { "add": [], "update": [], "delete": [] },
     "expected_red": [],
     "actual_red": [],
     "files_changed": [],
     "testids_added": [],
     "reproduce_block": "<the full run command, then each failing spec title with its assertion excerpt>",
     "blocker_reason": null
   }
   ```

Finish your turn by listing every file you created or changed.
