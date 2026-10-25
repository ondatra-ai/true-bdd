# Issue 203 continuation

Branch: `impl/203-remove-crush-product`; base: `71d93858ec420d6df9e903f3587b8e4d9f5b3482`.

Steps 1–4 are committed. Product removal, configuration, harness migration, Stories, Registry, generated tests and ADR-0023 are implemented. The Registry had 259 Scenarios at the actual integration base; it now has 250: 36 deleted, 20 rewritten, 27 replacements added. `scenarios.md` lists all 56 affected original Scenarios, every original invariant and exact old/new steps. `inventory.md` covers the original 304 tracked-file hits.

## Recording blocker

Ordered recording stops at `us-apply-002-duplicates-collapse`, fixture `us-apply-duplicate-collapse`. The real codex applier logged `sandbox=read-only` and returned applied false because it could not edit the scratch Registry. Its rejected recording is under `tmp/test_run/2026-10-25_10-00-24/.cassettes-staging/us-apply-duplicate-collapse`; evidence is also in that session’s fixture `bdd-cli-logs` and `record-logs/us-apply-002-duplicates-collapse.log`.

The brief says to retain every other codex apply sandbox until #205. The ticket’s assertion that all those apply turns already use workspace-write is incorrect: `FixApplier.selectMode` uses EditMode for scratch-only document work, and `codexSandbox` keeps that read-only. Do not weaken this Scenario, select Claude for its recording, hand-edit its shelf, or widen permissions under #203. Resolve through #205 or an explicit revised Owner decision, then record this fixture first and continue in manifest order.

Recorded 8 of 106 fixtures; 98 remain. Recorded service CLI time: 15.63 minutes; manifest historical service time: 176.45 minutes, excluding judges. No usage limit occurred.

## Completed recordings

- `build-code-run-004-fix-reruns-fixed-test` → `build-code-fix-named-test` (1.32 CLI minutes)
- `build-code-run-005-playwright-fix-loop-converges` → `build-code-playwright-nextjs` (4.52 CLI minutes)
- `build-code-frameworks-002-jest-fix-reruns-test-by-name` → `build-code-jest-fix-named-test` (1.75 CLI minutes)
- `build-code-frameworks-003-playwright-fix-reruns-test-by-grep` → `build-code-playwright-failing-assertion` (1.34 CLI minutes)
- `build-code-frameworks-004-go-package-not-compiling-rerun-unfiltered` → `build-code-go-package-does-not-compile` (1.52 CLI minutes)
- `build-tests-steps-001-unbound-scenario-gets-test-and-steps` → `build-tests-fix-happy-path` (5.17 CLI minutes)
- `build-tests-refusals-003-undeclared-registry-refused` → `build-tests-unconfigured-registry` (0.00 CLI minutes)
- `engine-004-unknown-model-cli-refused` → `build-tests-unknown-cli-tier` (0.00 CLI minutes)

## Resuming

The committed `recordings.md` is the canonical status table. Ignored helpers `recordings.json`, `record_batch.py` and `render_manifest.py` remain in this worktree. Each record command is exactly the architecture’s declared record command plus the anchored Scenario selector; helpers execute sequentially in the foreground. Failure fixtures additionally prepend their `record-bin` and set `TRUE_BDD_RECORD_CODEX=/opt/homebrew/bin/codex`. Empty wrappers truncate `-o`. A fixture’s rejected shelf is never promoted. Commit after five successful fixtures, and stop immediately at any codex usage limit, reporting the reset.

Jest, Playwright selected-assertion and Go compile fixtures have explicit applied-repair confirmation protocols. They report a completed service edit as applied true while preserving truthful verification limitations and leaving the engine to run the suite. This preserves their existing positive branch assertions without changing permissions. The Go compile fixture must use inherited cache/scratch settings: a model-created alternate Go cache once generated 71 MB of cassette noise, and that recording was replaced by a real re-recording with the cache control. Never prune it manually.

The additional regression check makes CodexWriter prompt eligibility require the explicit apply option; ordinary codex appliers receive no replay forms even if a caller has supplied a TestCommand. Build-code writer profiles and plan/validation read-only behavior remain intact.

## Acceptance work still open

- Finish the 98 remaining recordings after the scratch-edit blocker is resolved, then run full offline replay and all required checks. Do not call partial recording complete.
- Review literal word-inventory exceptions. #204 tooling and instructions are deliberately untouched. Historical `docs/qna/20261021-112516-self-spec-and-doc-validation-chain.yaml` and tooling ADR-0016 retain historical/tooling names. The mandated committed inventory/disposition/recording artifacts also necessarily retain them. No runtime product dependency remains; the negative CLI refusal remains intentionally. Do not silently rewrite Owner decision transcripts to satisfy a text search.
- Full replay currently fails on pending stale or absent cassettes. `TestBuildTestsRun007DistinctFailuresWhole` also failed alone on its stale shelf; this was not classified as its intermittent failure.
- #181’s two-generation cut-A verification has not run; it follows a completed/reviewed #203 and is separate from recording.

No GitHub writes, pushes, merges or amended commits occurred. Every commit ends in Refs #203. The final report gives the last commit and exact final checks.
