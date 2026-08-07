<!--
CRUSH PROMPT TEMPLATE — APPLY REVIEW FINDINGS (test-author, Step 2 cycle).
The test-author DRIVER fills EVERY {{...}} and pipes the result into the SAME
crush session as the write round:
    <crush_wrapper> author - author-run<X> --continue
codex (read-only) reviewed the specs crush just wrote; the driver has already
SCORED the findings and lists only the KEPT ones below. Crush applies ONLY
those — no new scope. Never leave a {{...}} unfilled. Everything below `====`
is the prompt.
====
-->

A read-only reviewer (codex) reviewed the specs you just wrote and proposed the
fixes below. Apply ONLY these — do not add scope, and do not weaken any
assertion to make a test easier. Same sandbox as before: write ONLY under
`{{E2E_DIR}}`.

# Review findings to apply (already filtered — apply each one)

{{KEPT_FINDINGS}}

# After applying every finding

- Register any new testid you introduce in the contract files (contract entries
  only): {{TESTID_CONTRACT_FILES}}
- Drive `{{TSC_CMD}}` to ZERO errors.
- Run the FULL e2e suite: `{{E2E_RUN_CMD}}` (ALWAYS `--reporter=dot`, or redirect
  `> {{LOG_PATH}} 2>&1` and read it). The red set must STILL be EXACTLY the
  **expected-RED list** from your `02-reconcile.md` — no more, no less, each red
  for the right reason, every green-guard green. If applying a finding WOULD
  change that expected-RED set, do NOT silently drift: note it in the doc and
  stop instead.
- Append what you changed to `{{DOC_DIR}}/{{ROUND_DOC}}` (e.g.
  `05-review-round-1.md`) and REFRESH `{{DOC_DIR}}/result.json` in place (same
  schema — update `actual_red`, `files_changed`, `testids_added`, and
  `reproduce_block`; set `status` to `OK`, or to `BLOCKER` with a reason if you
  had to stop).

Finish your turn by listing every file you created or changed.
