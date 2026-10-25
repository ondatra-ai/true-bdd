# Issue 205 recording manifest

Copied from #203 at `6e061cf75bbb946a7a6417fe86e30bf6e63c3a3d`; original migration baseline `71d93858ec420d6df9e903f3587b8e4d9f5b3482`. All 106 migration rows remain, plus two new #205 Scenarios. No cassette is edited by hand.

Declared record command, each fixture alone:

```sh
go test -tags bdd -json -count=1 -timeout=180m ./tests/bdd-cli/ -mode=services:record,tests:record -run '^Test<CamelCase key>$'
```

Issue #205 recorded, in order: `us-apply-duplicate-collapse`, `codex-apply-us-refine`, `codex-apply-build-tests`, `build-tests-fix-happy-path`. Only the last fixture changes requests among #203’s eight recorded fixtures: its codex apply argv gains the confined flags. The five build-code fixtures retain identical argv/stdin; runtime changes are configuration/environment only. The other two fixtures have no AI requests.

| Scenario | Fixture | Migration status | #205 status | Recorded CLI minutes |
| --- | --- | --- | --- | --- |
| `build-code-run-004-fix-reruns-fixed-test` | `build-code-fix-named-test` | recorded | unchanged / deferred | 1.32 |
| `build-code-run-005-playwright-fix-loop-converges` | `build-code-playwright-nextjs` | recorded | unchanged / deferred | 4.52 |
| `build-code-frameworks-002-jest-fix-reruns-test-by-name` | `build-code-jest-fix-named-test` | recorded | unchanged / deferred | 1.75 |
| `build-code-frameworks-003-playwright-fix-reruns-test-by-grep` | `build-code-playwright-failing-assertion` | recorded | unchanged / deferred | 1.34 |
| `build-code-frameworks-004-go-package-not-compiling-rerun-unfiltered` | `build-code-go-package-does-not-compile` | recorded | unchanged / deferred | 1.52 |
| `build-tests-steps-001-unbound-scenario-gets-test-and-steps` | `build-tests-fix-happy-path` | recorded | recorded on #205 | 6.19 |
| `build-tests-refusals-003-undeclared-registry-refused` | `build-tests-unconfigured-registry` | recorded | unchanged / deferred | 0.00 |
| `engine-004-unknown-model-cli-refused` | `build-tests-unknown-cli-tier` | recorded | unchanged / deferred | 0.00 |
| `us-apply-002-duplicates-collapse` | `us-apply-duplicate-collapse` | recorded | recorded on #205 | 3.04 |
| `engine-003-locations-come-from-config` | `us-apply-relocated-registry` | remaining | unchanged / deferred | 0.00 |
| `us-apply-001-missing-criteria-added` | `us-apply-rewalk-converges` | remaining | unchanged / deferred | 0.00 |
| `us-refine-fix-001-vague-description-made-measurable` | `us-refine-fix-desc-qualifier` | remaining | unchanged / deferred | 0.00 |
| `us-refine-fix-002-gherkin-description-states-a-rule` | `us-refine-fix-descriptions` | remaining | unchanged / deferred | 0.00 |
| `us-refine-fix-003-forbidden-verb-made-observable` | `us-refine-fix-forbidden-verb` | remaining | unchanged / deferred | 0.00 |
| `us-refine-fix-004-vague-step-made-measurable` | `us-refine-fix-step-qualifier` | remaining | unchanged / deferred | 0.00 |
| `us-refine-fix-005-missing-steps-written` | `us-refine-fix-steps` | remaining | unchanged / deferred | 0.00 |
| `us-refine-fix-006-feature-label-survives` | `us-refine-fix-keeps-feature` | remaining | unchanged / deferred | 0.00 |
| `us-refine-fix-007-step-outside-catalogue-gets-phrase` | `us-refine-fix-records-phrase` | remaining | unchanged / deferred | 0.00 |
| `scen-check-fix-001-unmatched-step-gets-phrase` | `scen-check-fix-records-phrase` | remaining | unchanged / deferred | 0.00 |
| `scen-check-fix-002-undefined-term-gets-definition` | `scen-check-fix-records-term` | remaining | unchanged / deferred | 0.00 |
| `scen-check-fix-003-unmapped-term-gets-mapping` | `scen-check-fix-records-mapping` | remaining | unchanged / deferred | 0.00 |
| `scen-check-quality-fix-001-missing-step-added` | `scen-check-fix-adds-missing-step` | remaining | unchanged / deferred | 0.00 |
| `scen-check-quality-fix-002-description-qualifier-replaced` | `scen-check-fix-rewrites-description-qualifier` | remaining | unchanged / deferred | 0.00 |
| `scen-check-quality-fix-003-step-qualifier-replaced` | `scen-check-fix-rewrites-step-qualifier` | remaining | unchanged / deferred | 0.00 |
| `scen-check-quality-fix-004-forbidden-verb-replaced` | `scen-check-fix-rewrites-forbidden-verb` | remaining | unchanged / deferred | 0.00 |
| `scen-check-quality-fix-005-implementation-step-rewritten` | `scen-check-fix-rewrites-implementation` | remaining | unchanged / deferred | 0.00 |
| `scen-check-quality-fix-006-measurable-criterion-added` | `scen-check-fix-adds-measurable-criterion` | remaining | unchanged / deferred | 0.00 |
| `scen-check-quality-fix-007-subjective-outcome-rewritten` | `scen-check-fix-rewrites-subjective-outcome` | remaining | unchanged / deferred | 0.00 |
| `scen-check-fix-005-restated-meaning-rewritten` | `scen-check-fix-rewrites-phrase-meaning` | remaining | unchanged / deferred | 0.00 |
| `scen-check-fix-006-misused-phrase-step-rewritten` | `scen-check-fix-rewrites-phrase-use` | remaining | unchanged / deferred | 0.00 |
| `scen-check-fix-004-shared-definition-merges-phrases` | `scen-check-fix-merges-phrases` | remaining | unchanged / deferred | 0.00 |
| `build-tests-steps-006-steps-authored-from-spec` | `build-tests-fix-from-spec` | remaining | unchanged / deferred | 0.00 |
| `scen-check-fix-007-whole-checklist-fixes-description` | `scen-check-fix-rewrites-description` | remaining | unchanged / deferred | 0.00 |
| `engine-documents-003-undeclared-architecture-refused` | `build-tests-undeclared-architecture` | remaining | unchanged / deferred | 0.00 |
| `engine-documents-004-undeclared-registry-refused-by-us-apply` | `us-apply-undeclared-registry` | remaining | unchanged / deferred | 0.00 |
| `engine-documents-005-undeclared-registry-refused-by-scen-check` | `scen-check-undeclared-registry` | remaining | unchanged / deferred | 0.00 |
| `engine-documents-006-directory-document-refused` | `build-code-directory-architecture` | remaining | unchanged / deferred | 0.00 |
| `fix-loop-choices-001-refine-reshapes-fix` | `fix-loop-refine-feedback` | remaining | unchanged / deferred | 0.00 |
| `fix-loop-choices-002-own-words-answer-kept` | `fix-loop-own-words` | remaining | unchanged / deferred | 0.00 |
| `fix-loop-bounds-004-walks-exhausted-keep-term-documents` | `fix-loop-walks-exhausted-terms` | remaining | unchanged / deferred | 0.00 |
| `fix-loop-bounds-001-one-walk-keeps-story` | `fix-loop-attempts-exhausted` | remaining | unchanged / deferred | 0.00 |
| `fix-loop-bounds-002-refixed-prompt-stops-walk` | `fix-loop-refix-stops` | remaining | unchanged / deferred | 0.00 |
| `engine-routes-003-prompt-tiers-win` | `scen-check-prompt-names-its-tiers` | remaining | unchanged / deferred | 0.00 |
| `engine-routes-004-failed-claude-turn-stops-run` | `scen-check-claude-turn-fails` | remaining | unchanged / deferred | 0.00 |
| `engine-routes-005-failed-codex-turn-stops-run` | `scen-check-codex-turn-fails` | remaining | unchanged / deferred | 0.00 |
| `engine-config-002-malformed-model-entries-refused` | `build-tests-malformed-model-entries` | remaining | unchanged / deferred | 0.00 |
| `engine-config-004-prompt-tier-unconfigured-refused` | `scen-check-prompt-tier-unconfigured` | remaining | unchanged / deferred | 0.00 |
| `engine-config-006-default-write-roots` | `build-tests-fix-default-write-roots` | remaining | unchanged / deferred | 0.00 |
| `engine-checklists-001-missing-checklist-refused` | `scen-check-checklist-missing` | remaining | unchanged / deferred | 0.00 |
| `build-tests-run-001-failed-definition-repaired` | `build-tests-run-recovery` | remaining | unchanged / deferred | 0.00 |
| `build-tests-run-002-attempt-bound-reports-failing-step` | `build-tests-run-bound` | remaining | unchanged / deferred | 0.00 |
| `build-tests-run-003-shared-definition-scenarios-run` | `build-tests-run-shared` | remaining | unchanged / deferred | 0.00 |
| `build-tests-run-004-selected-package-must-pass` | `build-tests-run-package` | remaining | unchanged / deferred | 0.00 |
| `build-code-run-006-shared-fix-skips-green-peer` | `build-code-shared-fix` | remaining | unchanged / deferred | 0.00 |
| `build-code-run-007-next-walk-finds-regression` | `build-code-cross-walk` | remaining | unchanged / deferred | 0.00 |
| `build-code-frameworks-005-not-applied-revalidated` | `build-code-not-applied` | remaining | unchanged / deferred | 0.00 |
| `build-code-frameworks-006-not-applied-hits-fix-bound` | `build-code-not-applied-bound` | remaining | unchanged / deferred | 0.00 |
| `build-tests-run-005-prebuilt-engine-without-source` | `build-tests-run-prebuilt` | remaining | unchanged / deferred | 0.00 |
| `build-code-spec-005-matched-scenario-embedded` | `build-code-spec-scenario` | remaining | unchanged / deferred | 0.00 |
| `build-code-spec-006-unreadable-product-warned` | `build-code-spec-missing-product` | remaining | unchanged / deferred | 0.00 |
| `build-code-spec-007-unreadable-registry-warned` | `build-code-spec-missing-registry` | remaining | unchanged / deferred | 0.00 |
| `build-tests-run-006-identical-failures-grouped` | `build-tests-run-grouped` | remaining | unchanged / deferred | 0.00 |
| `build-tests-run-007-distinct-failures-whole` | `build-tests-run-distinct` | remaining | unchanged / deferred | 0.00 |
| `build-code-set-001-partial-repair` | `build-code-partial` | remaining | unchanged / deferred | 0.00 |
| `build-code-set-002-shared-evidence` | `build-code-grouped` | remaining | unchanged / deferred | 0.00 |
| `build-code-set-003-mixed-synthetic` | `build-code-mixed-build` | remaining | unchanged / deferred | 0.00 |
| `build-code-set-004-missing-result` | `build-code-missing-member` | remaining | unchanged / deferred | 0.00 |
| `build-code-set-005-shrinking-counter` | `build-code-shrinking-bound` | remaining | unchanged / deferred | 0.00 |
| `build-code-confirmation-001-passes` | `build-code-no-confirmation-passes` | remaining | unchanged / deferred | 0.00 |
| `build-code-confirmation-002-retries` | `build-code-no-confirmation-retries` | remaining | unchanged / deferred | 0.00 |
| `build-code-confirmation-003-bound` | `build-code-no-confirmation-bound` | remaining | unchanged / deferred | 0.00 |
| `build-code-writer-001-nonzero` | `build-code-writer-nonzero` | remaining | unchanged / deferred | 0.00 |
| `build-code-writer-002-empty` | `build-code-writer-empty` | remaining | unchanged / deferred | 0.00 |
| `build-code-writer-003-bound` | `build-code-writer-bound` | remaining | unchanged / deferred | 0.00 |
| `build-code-claude-005-replay` | `build-code-claude-replay` | remaining | unchanged / deferred | 0.00 |
| `build-code-claude-006-failure` | `build-code-claude-failure` | remaining | unchanged / deferred | 0.00 |
| `build-code-codex-001-selection` | `build-code-codex-selection` | remaining | unchanged / deferred | 0.00 |
| `build-code-codex-002-nonzero` | `build-code-codex-nonzero` | remaining | unchanged / deferred | 0.00 |
| `build-code-codex-003-missing-root` | `build-code-codex-missing-root` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-001-read-only-tool-allowed` | `claude-guard-read-only-tool` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-002-shell-denied-without-grant` | `claude-guard-shell-denied` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-004-unknown-tool-denied` | `claude-guard-unknown-tool` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-005-missing-policy-denied` | `claude-guard-missing-policy` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-006-unreadable-policy-denied` | `claude-guard-unreadable-policy` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-007-payload-names-the-call` | `claude-guard-hook-payload` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-writes-001-write-inside-root-allowed` | `claude-guard-write-inside-root` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-writes-002-write-outside-roots-denied` | `claude-guard-write-outside-roots` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-writes-003-symlink-escape-denied` | `claude-guard-symlink-escape` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-writes-004-shared-prefix-denied` | `claude-guard-shared-prefix` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-writes-006-unnamed-path-denied` | `claude-guard-unnamed-path` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-reads-001-outside-project-denied` | `claude-guard-read-outside` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-reads-002-relative-escape-denied` | `claude-guard-read-relative-escape` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-reads-003-symlink-escape-denied` | `claude-guard-read-symlink-escape` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-reads-004-unnamed-path-allowed` | `claude-guard-read-unnamed-path` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-reads-005-missing-read-root-denied` | `claude-guard-read-missing-root` | remaining | unchanged / deferred | 0.00 |
| `claude-guard-reads-006-relative-path-allowed` | `claude-guard-read-relative-path` | remaining | unchanged / deferred | 0.00 |
| `claude-replay-001-whole-command` | `claude-replay-whole-command` | remaining | unchanged / deferred | 0.00 |
| `claude-replay-002-selected-command` | `claude-replay-selected-command` | remaining | unchanged / deferred | 0.00 |
| `claude-replay-003-shell-operator` | `claude-replay-shell-operator` | remaining | unchanged / deferred | 0.00 |
| `claude-replay-004-assignment` | `claude-replay-assignment` | remaining | unchanged / deferred | 0.00 |
| `claude-replay-005-another-program` | `claude-replay-another-program` | remaining | unchanged / deferred | 0.00 |
| `claude-replay-006-extra-flag` | `claude-replay-extra-flag` | remaining | unchanged / deferred | 0.00 |
| `claude-replay-007-double-quotes` | `claude-replay-double-quotes` | remaining | unchanged / deferred | 0.00 |
| `claude-replay-008-wrong-directory` | `claude-replay-wrong-directory` | remaining | unchanged / deferred | 0.00 |
| `claude-replay-009-no-command` | `claude-replay-no-command` | remaining | unchanged / deferred | 0.00 |
| `build-code-claude-007-unrepresentable` | `build-code-unrepresentable-replay` | remaining | unchanged / deferred | 0.00 |
| `codex-apply-001-us-refine-writes-scratch` | `codex-apply-us-refine` | recorded | recorded on #205 | 1.85 |
| `codex-apply-002-build-tests-writes-configured-root` | `codex-apply-build-tests` | recorded | recorded on #205 | 5.81 |

Recorded migration/new fixtures: 11 of 108; pending migration fixtures: 97. #205 recorded 4 fixtures, 16.88 service CLI minutes, excluding judge time.

## Pending Scenarios

- `engine-003-locations-come-from-config` — `us-apply-relocated-registry`
- `us-apply-001-missing-criteria-added` — `us-apply-rewalk-converges`
- `us-refine-fix-001-vague-description-made-measurable` — `us-refine-fix-desc-qualifier`
- `us-refine-fix-002-gherkin-description-states-a-rule` — `us-refine-fix-descriptions`
- `us-refine-fix-003-forbidden-verb-made-observable` — `us-refine-fix-forbidden-verb`
- `us-refine-fix-004-vague-step-made-measurable` — `us-refine-fix-step-qualifier`
- `us-refine-fix-005-missing-steps-written` — `us-refine-fix-steps`
- `us-refine-fix-006-feature-label-survives` — `us-refine-fix-keeps-feature`
- `us-refine-fix-007-step-outside-catalogue-gets-phrase` — `us-refine-fix-records-phrase`
- `scen-check-fix-001-unmatched-step-gets-phrase` — `scen-check-fix-records-phrase`
- `scen-check-fix-002-undefined-term-gets-definition` — `scen-check-fix-records-term`
- `scen-check-fix-003-unmapped-term-gets-mapping` — `scen-check-fix-records-mapping`
- `scen-check-quality-fix-001-missing-step-added` — `scen-check-fix-adds-missing-step`
- `scen-check-quality-fix-002-description-qualifier-replaced` — `scen-check-fix-rewrites-description-qualifier`
- `scen-check-quality-fix-003-step-qualifier-replaced` — `scen-check-fix-rewrites-step-qualifier`
- `scen-check-quality-fix-004-forbidden-verb-replaced` — `scen-check-fix-rewrites-forbidden-verb`
- `scen-check-quality-fix-005-implementation-step-rewritten` — `scen-check-fix-rewrites-implementation`
- `scen-check-quality-fix-006-measurable-criterion-added` — `scen-check-fix-adds-measurable-criterion`
- `scen-check-quality-fix-007-subjective-outcome-rewritten` — `scen-check-fix-rewrites-subjective-outcome`
- `scen-check-fix-005-restated-meaning-rewritten` — `scen-check-fix-rewrites-phrase-meaning`
- `scen-check-fix-006-misused-phrase-step-rewritten` — `scen-check-fix-rewrites-phrase-use`
- `scen-check-fix-004-shared-definition-merges-phrases` — `scen-check-fix-merges-phrases`
- `build-tests-steps-006-steps-authored-from-spec` — `build-tests-fix-from-spec`
- `scen-check-fix-007-whole-checklist-fixes-description` — `scen-check-fix-rewrites-description`
- `engine-documents-003-undeclared-architecture-refused` — `build-tests-undeclared-architecture`
- `engine-documents-004-undeclared-registry-refused-by-us-apply` — `us-apply-undeclared-registry`
- `engine-documents-005-undeclared-registry-refused-by-scen-check` — `scen-check-undeclared-registry`
- `engine-documents-006-directory-document-refused` — `build-code-directory-architecture`
- `fix-loop-choices-001-refine-reshapes-fix` — `fix-loop-refine-feedback`
- `fix-loop-choices-002-own-words-answer-kept` — `fix-loop-own-words`
- `fix-loop-bounds-004-walks-exhausted-keep-term-documents` — `fix-loop-walks-exhausted-terms`
- `fix-loop-bounds-001-one-walk-keeps-story` — `fix-loop-attempts-exhausted`
- `fix-loop-bounds-002-refixed-prompt-stops-walk` — `fix-loop-refix-stops`
- `engine-routes-003-prompt-tiers-win` — `scen-check-prompt-names-its-tiers`
- `engine-routes-004-failed-claude-turn-stops-run` — `scen-check-claude-turn-fails`
- `engine-routes-005-failed-codex-turn-stops-run` — `scen-check-codex-turn-fails`
- `engine-config-002-malformed-model-entries-refused` — `build-tests-malformed-model-entries`
- `engine-config-004-prompt-tier-unconfigured-refused` — `scen-check-prompt-tier-unconfigured`
- `engine-config-006-default-write-roots` — `build-tests-fix-default-write-roots`
- `engine-checklists-001-missing-checklist-refused` — `scen-check-checklist-missing`
- `build-tests-run-001-failed-definition-repaired` — `build-tests-run-recovery`
- `build-tests-run-002-attempt-bound-reports-failing-step` — `build-tests-run-bound`
- `build-tests-run-003-shared-definition-scenarios-run` — `build-tests-run-shared`
- `build-tests-run-004-selected-package-must-pass` — `build-tests-run-package`
- `build-code-run-006-shared-fix-skips-green-peer` — `build-code-shared-fix`
- `build-code-run-007-next-walk-finds-regression` — `build-code-cross-walk`
- `build-code-frameworks-005-not-applied-revalidated` — `build-code-not-applied`
- `build-code-frameworks-006-not-applied-hits-fix-bound` — `build-code-not-applied-bound`
- `build-tests-run-005-prebuilt-engine-without-source` — `build-tests-run-prebuilt`
- `build-code-spec-005-matched-scenario-embedded` — `build-code-spec-scenario`
- `build-code-spec-006-unreadable-product-warned` — `build-code-spec-missing-product`
- `build-code-spec-007-unreadable-registry-warned` — `build-code-spec-missing-registry`
- `build-tests-run-006-identical-failures-grouped` — `build-tests-run-grouped`
- `build-tests-run-007-distinct-failures-whole` — `build-tests-run-distinct`
- `build-code-set-001-partial-repair` — `build-code-partial`
- `build-code-set-002-shared-evidence` — `build-code-grouped`
- `build-code-set-003-mixed-synthetic` — `build-code-mixed-build`
- `build-code-set-004-missing-result` — `build-code-missing-member`
- `build-code-set-005-shrinking-counter` — `build-code-shrinking-bound`
- `build-code-confirmation-001-passes` — `build-code-no-confirmation-passes`
- `build-code-confirmation-002-retries` — `build-code-no-confirmation-retries`
- `build-code-confirmation-003-bound` — `build-code-no-confirmation-bound`
- `build-code-writer-001-nonzero` — `build-code-writer-nonzero`
- `build-code-writer-002-empty` — `build-code-writer-empty`
- `build-code-writer-003-bound` — `build-code-writer-bound`
- `build-code-claude-005-replay` — `build-code-claude-replay`
- `build-code-claude-006-failure` — `build-code-claude-failure`
- `build-code-codex-001-selection` — `build-code-codex-selection`
- `build-code-codex-002-nonzero` — `build-code-codex-nonzero`
- `build-code-codex-003-missing-root` — `build-code-codex-missing-root`
- `claude-guard-001-read-only-tool-allowed` — `claude-guard-read-only-tool`
- `claude-guard-002-shell-denied-without-grant` — `claude-guard-shell-denied`
- `claude-guard-004-unknown-tool-denied` — `claude-guard-unknown-tool`
- `claude-guard-005-missing-policy-denied` — `claude-guard-missing-policy`
- `claude-guard-006-unreadable-policy-denied` — `claude-guard-unreadable-policy`
- `claude-guard-007-payload-names-the-call` — `claude-guard-hook-payload`
- `claude-guard-writes-001-write-inside-root-allowed` — `claude-guard-write-inside-root`
- `claude-guard-writes-002-write-outside-roots-denied` — `claude-guard-write-outside-roots`
- `claude-guard-writes-003-symlink-escape-denied` — `claude-guard-symlink-escape`
- `claude-guard-writes-004-shared-prefix-denied` — `claude-guard-shared-prefix`
- `claude-guard-writes-006-unnamed-path-denied` — `claude-guard-unnamed-path`
- `claude-guard-reads-001-outside-project-denied` — `claude-guard-read-outside`
- `claude-guard-reads-002-relative-escape-denied` — `claude-guard-read-relative-escape`
- `claude-guard-reads-003-symlink-escape-denied` — `claude-guard-read-symlink-escape`
- `claude-guard-reads-004-unnamed-path-allowed` — `claude-guard-read-unnamed-path`
- `claude-guard-reads-005-missing-read-root-denied` — `claude-guard-read-missing-root`
- `claude-guard-reads-006-relative-path-allowed` — `claude-guard-read-relative-path`
- `claude-replay-001-whole-command` — `claude-replay-whole-command`
- `claude-replay-002-selected-command` — `claude-replay-selected-command`
- `claude-replay-003-shell-operator` — `claude-replay-shell-operator`
- `claude-replay-004-assignment` — `claude-replay-assignment`
- `claude-replay-005-another-program` — `claude-replay-another-program`
- `claude-replay-006-extra-flag` — `claude-replay-extra-flag`
- `claude-replay-007-double-quotes` — `claude-replay-double-quotes`
- `claude-replay-008-wrong-directory` — `claude-replay-wrong-directory`
- `claude-replay-009-no-command` — `claude-replay-no-command`
- `build-code-claude-007-unrepresentable` — `build-code-unrepresentable-replay`
