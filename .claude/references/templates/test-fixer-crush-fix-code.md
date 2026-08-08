<!--
CRUSH PROMPT TEMPLATE — FIX CODE (test-fixer, Step 1).
The test-fixer DRIVER fills EVERY {{...}} (resolving concrete paths from
docs/context/paths.yaml first) and pipes the result into:
    <crush_wrapper> fixer - test-fixer-run<X>
Concatenate so the filled template is piped on stdin, not embedded in the command:
    { cat <<'EOF' ...this template, filled... EOF } | <crush_wrapper> fixer - test-fixer-run<X>
Crush is TASK-BLIND here: whatever the e2e suite reports RED (found by running the suite
in step 1) is its entire specification.
It knows NOTHING about this repo — every path, command, and the design system are
inlined below. Never leave a {{...}} unfilled. Everything below `====` is the prompt.
====
-->

You are making failing end-to-end tests pass by writing PRODUCTION code. Whatever the
e2e suite reports RED (you find it in step 1) is your ENTIRE specification — make those
specs pass WITHOUT changing them. Do EXACTLY the four steps and nothing else. You may write files ONLY under
`{{HARNESS_CODE_ROOT}}` (production) and `{{UNIT_TESTS_DIR}}` (the unit tests that back
it) — every path under `tests/` is READ-ONLY to you, and you must never change the
package manifest's `scripts`.

# Where everything is (inlined — do not go looking elsewhere)

- production code root (your sandbox): `{{HARNESS_CODE_ROOT}}`
- unit-test tree (write the tests that back your code here): `{{UNIT_TESTS_DIR}}`
- run the e2e suite: `{{E2E_RUN_CMD}}`  (ALWAYS `--reporter=dot`, or redirect
  `> {{LOG_PATH}} 2>&1` and read the log — a chatty reporter deadlocks your shell)
- run the unit suite: `{{UNIT_RUN_CMD}}`
- typecheck / build: `{{TSC_CMD}}`
- **design system (read BEFORE styling any UI — you do not know it exists):** the
  tokens file `{{DESIGN_TOKENS}}`, the design SPEC `{{DESIGN_SPEC}}`, the prototype app
  `{{DESIGN_PROTOTYPE}}`. Use ONLY those tokens and existing components — never ad-hoc
  colours, spacing, or typography; when a screen has a prototype counterpart, match it.
- write your per-step docs and result under: `{{DOC_DIR}}`

# Steps (write ONE doc file per step into `{{DOC_DIR}}`)

1. **Baseline → `01-baseline.md`.** Run the FULL suite (e2e + unit) and record the red
   set — those failing specs ARE your spec. If nothing is red, there is nothing to fix:
   write `result.json` with `status: "OK"`, empty `files_changed`, and end.
2. **Fix → `02-fix.md`.** Implement the production code under `{{HARNESS_CODE_ROOT}}`
   that makes the failing tests pass, and add the unit tests under `{{UNIT_TESTS_DIR}}`
   that back it. Never touch anything under `tests/` or the package `scripts`. If a
   test genuinely cannot be satisfied in code, STOP with evidence (the failing
   assertion + why) — do not guess.
3. **Verify green → `03-green-verify.md`.** Re-run the previously-red specs to green, then
   run the ENTIRE suite (e2e + unit) again to prove NO regression anywhere. List every
   file you created/changed.
4. **Result → `{{DOC_DIR}}/result.json`.** Emit this exact shape:

   ```json
   {
     "status": "OK",
     "baseline": { "red": [] },
     "files_changed": [],
     "unit_tests_added": [],
     "final_suite": { "passed": 0, "total": 0, "green": true },
     "unit_only_behavior": [],
     "blocker_reason": null
   }
   ```

   `unit_only_behavior` = any production behaviour you added that the red e2e specs do
   NOT assert (pinned only by your unit tests); `[]` if every added behaviour is
   demanded by a driving spec.

Finish your turn by listing every file you created or changed.
