<!--
CODEX PROMPT TEMPLATE — REVIEW THE FIX (test-fixer, Step 2 review cycle).
The test-fixer DRIVER fills EVERY {{...}}, writes the result to a prompt file under the
codex artifacts dir (paths.yaml → codex_artifacts, e.g. tmp/codex-fx-review-r<N>.md —
give the prompt file and the wrapper's answer file DISTINCT paths), and runs:
    <codex_wrapper> ro <prompt-file> fx-review-r<N>
The fixer is TASK-BLIND: codex's task is the failing e2e specs being greened (read
from disk), NOT a brief. codex is READ-ONLY, returns findings ONLY with
NO scores — the DRIVER scores each (composite ≥7 + four gates). Round 1 = fresh
findings; round 2+ also verifies applied findings + challenges skips (fill
{{PRIOR_FINDINGS}}). Only {{HARNESS_CODE_ROOT}}, {{UNIT_TESTS_DIR}}, {{E2E_DIR}},
{{PRIOR_FINDINGS}}; codex reveals everything else from the repo itself.
Everything below `====` is the prompt.
====
-->

You are reviewing the PRODUCTION code (and its unit tests) a writer produced to make a
set of failing end-to-end tests pass. You are READ-ONLY — never edit a file. Verify
every claim by running read-only commands or reading files; never review from memory.
Discover whatever you need from the repo itself — `docs/context/paths.yaml` (the
path/config source of truth) and the relevant `package.json` — rather than assuming.

# The task (the failing e2e specs — this is the whole spec)

The end-to-end specs the writer greened ARE the task — read them under `{{E2E_DIR}}` and
run the suite to see the set the code must satisfy. There is no separate brief.

# What to review

The writer's work lives under `{{HARNESS_CODE_ROOT}}` (production) and
`{{UNIT_TESTS_DIR}}` (the unit tests backing it). Both are git-ignored
(regenerated-from-tests), so read them WHOLE — they ARE the change surface; a `git
diff` will not show them. The e2e specs under `{{E2E_DIR}}` are the read-only source of
truth the code must satisfy — read them, and use `git diff` there to confirm the writer
left them UNCHANGED. The writer is barred from the off-limits tree declared in
`paths.yaml → off_limits_test_fixer` and from the package `scripts` — confirm nothing
there changed. Run the e2e + unit
suites yourself to verify green / no-regression (find the commands in `package.json`);
for UI, read the design system declared in `paths.yaml → design_system` (production must
use only those tokens).

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
2. **No test edits** — confirm nothing under the off-limits tree (`paths.yaml →
   off_limits_test_fixer`) and no package `scripts` changed; the specs are unchanged.
3. **Regeneratability risk** — the e2e suite is the ONLY committed spec;
   `{{HARNESS_CODE_ROOT}}` + its unit tests are git-ignored, regenerated-from-tests.
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
