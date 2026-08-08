<!--
CODEX PROMPT TEMPLATE — REVIEW THE FIX (test-fixer, Step 2 review cycle).
The test-fixer DRIVER fills EVERY {{...}}, writes the result to a prompt file under the
codex artifacts dir (paths.yaml → codex_artifacts, e.g. tmp/codex-fx-review-r<N>.md —
give the prompt file and the wrapper's answer file DISTINCT paths), and runs:
    <codex_wrapper> ro <prompt-file> fx-review-r<N>
The fixer is TASK-BLIND: codex's "task" is the reproduce block + the e2e specs being
greened (read from disk), NOT a brief. codex is READ-ONLY, returns findings ONLY with
NO scores — the DRIVER scores each (composite ≥7 + four gates). Round 1 = fresh
findings; round 2+ also verifies applied findings + challenges skips (fill
{{PRIOR_FINDINGS}}). Everything below `====` is the prompt.
====
-->

You are reviewing PRODUCTION code (and its unit tests) that a writer produced to make a
set of failing end-to-end tests pass. You are READ-ONLY — do NOT edit any file. Run
read-only commands to VERIFY every claim (never review from memory), then return
findings only.

# The spec being satisfied (the failing tests — this is the whole task)

{{REPRODUCE_BLOCK}}

The e2e specs that drive this are under `{{E2E_DIR}}` — read them; they are the source
of truth. The writer must NOT have changed them.

# What to review (the writer's changes)

- production code root: `{{HARNESS_CODE_ROOT}}`
- unit-test tree: `{{UNIT_TESTS_DIR}}`
- the diff to review: run `{{DIFF_CMD}}`
- verify green/red claims by running the suites yourself: `{{E2E_RUN_CMD}}` and
  `{{UNIT_RUN_CMD}}` (always `--reporter=dot`, or redirect `> {{LOG_PATH}} 2>&1`)
- typecheck / build: `{{TSC_CMD}}`
- design system (for UI correctness): tokens `{{DESIGN_TOKENS}}`, SPEC `{{DESIGN_SPEC}}`,
  prototype `{{DESIGN_PROTOTYPE}}`

# Prior findings + how the driver disposed of them  (EMPTY on round 1)

{{PRIOR_FINDINGS}}

# Your jobs, in order

- **Round 1:** fresh findings only.
- **Round 2+:** (a) verify every APPLIED finding is correctly and completely
  implemented; (b) challenge every SKIP — does the recorded reason still hold?;
  (c) fresh findings.

# What to look for (code-review dimensions)

1. **Correctness** — does the code actually satisfy every driving spec, for the right
   reason (not a hardcoded value or a coincidence that makes the assertion pass)?
2. **No test edits** — confirm nothing under `tests/` and no package `scripts` changed;
   the specs are unchanged.
3. **Regeneratability risk** — the e2e suite is the ONLY committed spec;
   `{{HARNESS_CODE_ROOT}}` + its unit tests are gitignored, regenerated-from-tests.
   Flag any production behaviour pinned ONLY by a unit test (no e2e assertion covers
   it): on a fresh regenerate it vanishes.
4. **Unit-test quality** — do the unit tests actually exercise the code, fail when it
   breaks, and avoid tautologies/over-mocking?
5. **Regression risk** — does the change break existing behaviour or leave the suite
   flaky?
6. **Design-system adherence** — for UI, only design tokens/components; no ad-hoc
   colours/spacing/typography; matches the prototype where one exists.
7. **Scope** — no gold-plating beyond what the specs demand.

# Output — findings ONLY, no scores, no ranking

For EACH finding return:

- `id` — short stable slug.
- `location` — file + function / line.
- `problem` — one sentence: what is wrong.
- `evidence` — the command you ran and what it showed (or the exact offending lines).
- `fix` — the concrete, minimal change the writer should make.
- `category` — one of: correctness | test-edit | regeneratability | unit-test-quality |
  regression | design-system | scope.

Do NOT score, rank, or assign severity — the driver scores each finding and decides
what to apply.
