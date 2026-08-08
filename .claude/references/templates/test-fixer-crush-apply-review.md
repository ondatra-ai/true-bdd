<!--
CRUSH PROMPT TEMPLATE — APPLY REVIEW FINDINGS (test-fixer, Step 2 review cycle).
The test-fixer DRIVER fills EVERY {{...}} and pipes the result into the SAME crush
session as the fix round:
    <crush_wrapper> fixer - test-fixer-run<X> --continue
codex (read-only) reviewed the production/unit diff crush just wrote; the driver
has already SCORED the findings and lists only the KEPT ones below. Crush applies
ONLY those — no new scope. Never leave a {{...}} unfilled. Everything below `====`
is the prompt.
====
-->

A read-only reviewer (codex) reviewed the production and unit code you just wrote and
proposed the fixes below. Apply ONLY these — do not add scope. Same sandbox as before:
write ONLY under `{{HARNESS_CODE_ROOT}}` (production) and `{{UNIT_TESTS_DIR}}` (unit
tests); everything under `tests/` stays READ-ONLY and never touch the package `scripts`.

# Review findings to apply (already filtered — apply each one)

{{KEPT_FINDINGS}}

# After applying every finding

- Keep UI styling on the design system — tokens `{{DESIGN_TOKENS}}`, SPEC
  `{{DESIGN_SPEC}}`, prototype `{{DESIGN_PROTOTYPE}}`; only those tokens/components.
- Typecheck / build to zero errors: `{{TSC_CMD}}`.
- Re-run the previously-red specs to green, then the ENTIRE suite (e2e + unit):
  `{{E2E_RUN_CMD}}` and `{{UNIT_RUN_CMD}}` (ALWAYS `--reporter=dot`, or redirect
  `> {{LOG_PATH}} 2>&1` and read it). The suite must be FULLY GREEN — no regressions.
  If a finding would break green and can't be resolved in code, do NOT silently drift:
  note it in the doc and stop instead.
- Append what you changed to `{{DOC_DIR}}/{{ROUND_DOC}}` (e.g. `04-review-round-1.md`)
  and REFRESH `{{DOC_DIR}}/result.json` in place (same schema — update `files_changed`,
  `unit_tests_added`, `final_suite`, `unit_only_behavior`; set `status` to `OK`, or
  `BLOCKER` with a reason if you had to stop).

Finish your turn by listing every file you created or changed.
