# Issue 203 recording manifest

Baseline: `71d93858ec420d6df9e903f3587b8e4d9f5b3482`. Scenario order is Registry insertion order, the declared deterministic order. Each fixture is run alone. Old minutes sum the service CLI `duration_ms` in `cassettes/*/meta.json`; judge time is excluded. A missing duration is shown as 0. No cassette is hand-edited. Unlisted surviving recording files remain byte-identical.

Declared record command:

```sh
go test -tags bdd -json -count=1 -timeout=180m ./tests/bdd-cli/ -mode=services:record,tests:record -run '^Test<CamelCase key>$'
```

Failure fixtures prepend their own `record-bin` to PATH and set `TRUE_BDD_RECORD_CODEX` to the installed codex path. Empty transport clears the `-o` file. Each fixture gets a separate log under `tmp/203/record-logs/`; failures remain pending and staged cassettes are not promoted. Codex usage-limit output stops recording and its reset time is reported.

| Scenario | Fixture | Reason | Old CLI minutes | Status | New CLI minutes |
| --- | --- | --- | --- | --- | --- |
| `build-code-run-004-fix-reruns-fixed-test` | `build-code-fix-named-test` | crush cassette / default writer changed | claude 1.01, crush 0.72 | recorded | 1.32 |
| `build-code-run-005-playwright-fix-loop-converges` | `build-code-playwright-nextjs` | crush cassette / default writer changed | claude 2.73, crush 3.09 | recorded | 4.52 |
| `build-code-frameworks-002-jest-fix-reruns-test-by-name` | `build-code-jest-fix-named-test` | crush cassette / default writer changed; explicit applied-repair confirmation protocol | claude 0.95, crush 0.47 | recorded | 1.75 |
| `build-code-frameworks-003-playwright-fix-reruns-test-by-grep` | `build-code-playwright-failing-assertion` | crush cassette / default writer changed; explicit applied-repair confirmation protocol | claude 0.84, crush 0.56 | recorded | 1.34 |
| `build-code-frameworks-004-go-package-not-compiling-rerun-unfiltered` | `build-code-go-package-does-not-compile` | crush cassette / default writer changed; explicit applied-repair confirmation protocol | claude 1.65, crush 1.00 | recorded | 1.52 |
| `build-tests-steps-001-unbound-scenario-gets-test-and-steps` | `build-tests-fix-happy-path` | crush cassette / default writer changed | claude 4.24, crush 1.16 | remaining | — |
| `build-tests-refusals-003-undeclared-registry-refused` | `build-tests-unconfigured-registry` | fixture input/template/configuration changed | 0 | remaining | — |
| `engine-004-unknown-model-cli-refused` | `build-tests-unknown-cli-tier` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `us-apply-002-duplicates-collapse` | `us-apply-duplicate-collapse` | crush cassette / default writer changed | claude 2.19, crush 0.68 | remaining | — |
| `engine-003-locations-come-from-config` | `us-apply-relocated-registry` | crush cassette / default writer changed; fixture input/template/configuration changed | claude 1.49, crush 0.46 | remaining | — |
| `us-apply-001-missing-criteria-added` | `us-apply-rewalk-converges` | crush cassette / default writer changed | claude 2.56, crush 0.76 | remaining | — |
| `us-refine-fix-001-vague-description-made-measurable` | `us-refine-fix-desc-qualifier` | crush cassette / default writer changed | claude 1.38, crush 0.36 | remaining | — |
| `us-refine-fix-002-gherkin-description-states-a-rule` | `us-refine-fix-descriptions` | crush cassette / default writer changed | claude 1.48, crush 0.37 | remaining | — |
| `us-refine-fix-003-forbidden-verb-made-observable` | `us-refine-fix-forbidden-verb` | crush cassette / default writer changed | claude 1.58, crush 0.20 | remaining | — |
| `us-refine-fix-004-vague-step-made-measurable` | `us-refine-fix-step-qualifier` | crush cassette / default writer changed | claude 1.47, crush 0.21 | remaining | — |
| `us-refine-fix-005-missing-steps-written` | `us-refine-fix-steps` | crush cassette / default writer changed | claude 1.63, crush 0.21 | remaining | — |
| `us-refine-fix-006-feature-label-survives` | `us-refine-fix-keeps-feature` | crush cassette / default writer changed | claude 1.72, crush 0.77 | remaining | — |
| `us-refine-fix-007-step-outside-catalogue-gets-phrase` | `us-refine-fix-records-phrase` | crush cassette / default writer changed | claude 1.27, crush 0.55 | remaining | — |
| `scen-check-fix-001-unmatched-step-gets-phrase` | `scen-check-fix-records-phrase` | crush cassette / default writer changed; rewritten/replacement Scenario | claude 1.31, crush 0.57 | remaining | — |
| `scen-check-fix-002-undefined-term-gets-definition` | `scen-check-fix-records-term` | crush cassette / default writer changed | claude 1.09, crush 0.50 | remaining | — |
| `scen-check-fix-003-unmapped-term-gets-mapping` | `scen-check-fix-records-mapping` | crush cassette / default writer changed | claude 1.07, crush 0.42 | remaining | — |
| `scen-check-quality-fix-001-missing-step-added` | `scen-check-fix-adds-missing-step` | crush cassette / default writer changed | claude 1.02, crush 0.37 | remaining | — |
| `scen-check-quality-fix-002-description-qualifier-replaced` | `scen-check-fix-rewrites-description-qualifier` | crush cassette / default writer changed | claude 1.13, crush 0.22 | remaining | — |
| `scen-check-quality-fix-003-step-qualifier-replaced` | `scen-check-fix-rewrites-step-qualifier` | crush cassette / default writer changed | claude 1.57, crush 0.41 | remaining | — |
| `scen-check-quality-fix-004-forbidden-verb-replaced` | `scen-check-fix-rewrites-forbidden-verb` | crush cassette / default writer changed | claude 1.27, crush 0.34 | remaining | — |
| `scen-check-quality-fix-005-implementation-step-rewritten` | `scen-check-fix-rewrites-implementation` | crush cassette / default writer changed | claude 1.29, crush 0.37 | remaining | — |
| `scen-check-quality-fix-006-measurable-criterion-added` | `scen-check-fix-adds-measurable-criterion` | crush cassette / default writer changed | claude 1.86, crush 0.40 | remaining | — |
| `scen-check-quality-fix-007-subjective-outcome-rewritten` | `scen-check-fix-rewrites-subjective-outcome` | crush cassette / default writer changed | claude 2.40, crush 0.34 | remaining | — |
| `scen-check-fix-005-restated-meaning-rewritten` | `scen-check-fix-rewrites-phrase-meaning` | crush cassette / default writer changed | claude 2.27, crush 0.42 | remaining | — |
| `scen-check-fix-006-misused-phrase-step-rewritten` | `scen-check-fix-rewrites-phrase-use` | crush cassette / default writer changed | claude 1.94, crush 0.30 | remaining | — |
| `scen-check-fix-004-shared-definition-merges-phrases` | `scen-check-fix-merges-phrases` | crush cassette / default writer changed | claude 0.88, crush 0.87 | remaining | — |
| `build-tests-steps-006-steps-authored-from-spec` | `build-tests-fix-from-spec` | crush cassette / default writer changed | claude 4.83, crush 0.70 | remaining | — |
| `scen-check-fix-007-whole-checklist-fixes-description` | `scen-check-fix-rewrites-description` | crush cassette / default writer changed | claude 5.94, crush 0.31 | remaining | — |
| `engine-documents-003-undeclared-architecture-refused` | `build-tests-undeclared-architecture` | fixture input/template/configuration changed | 0 | remaining | — |
| `engine-documents-004-undeclared-registry-refused-by-us-apply` | `us-apply-undeclared-registry` | fixture input/template/configuration changed | 0 | remaining | — |
| `engine-documents-005-undeclared-registry-refused-by-scen-check` | `scen-check-undeclared-registry` | fixture input/template/configuration changed | 0 | remaining | — |
| `engine-documents-006-directory-document-refused` | `build-code-directory-architecture` | fixture input/template/configuration changed | 0 | remaining | — |
| `fix-loop-choices-001-refine-reshapes-fix` | `fix-loop-refine-feedback` | crush cassette / default writer changed | claude 1.78, crush 0.33 | remaining | — |
| `fix-loop-choices-002-own-words-answer-kept` | `fix-loop-own-words` | crush cassette / default writer changed | claude 1.26, crush 0.35 | remaining | — |
| `fix-loop-bounds-004-walks-exhausted-keep-term-documents` | `fix-loop-walks-exhausted-terms` | crush cassette / default writer changed | claude 0.45, crush 0.51 | remaining | — |
| `fix-loop-bounds-001-one-walk-keeps-story` | `fix-loop-attempts-exhausted` | crush cassette / default writer changed | claude 1.32, crush 0.27 | remaining | — |
| `fix-loop-bounds-002-refixed-prompt-stops-walk` | `fix-loop-refix-stops` | crush cassette / default writer changed | claude 1.99, crush 0.94 | remaining | — |
| `engine-routes-003-prompt-tiers-win` | `scen-check-prompt-names-its-tiers` | crush cassette / default writer changed; fixture input/template/configuration changed; rewritten/replacement Scenario | claude 0.22, codex 0.30, crush 0.63 | remaining | — |
| `engine-routes-004-failed-claude-turn-stops-run` | `scen-check-claude-turn-fails` | fixture input/template/configuration changed | claude 0.03 | remaining | — |
| `engine-routes-005-failed-codex-turn-stops-run` | `scen-check-codex-turn-fails` | fixture input/template/configuration changed; rewritten/replacement Scenario | crush 0.02 | remaining | — |
| `engine-config-002-malformed-model-entries-refused` | `build-tests-malformed-model-entries` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `engine-config-004-prompt-tier-unconfigured-refused` | `scen-check-prompt-tier-unconfigured` | fixture input/template/configuration changed | 0 | remaining | — |
| `engine-config-006-default-write-roots` | `build-tests-fix-default-write-roots` | fixture input/template/configuration changed | claude 4.38 | remaining | — |
| `engine-checklists-001-missing-checklist-refused` | `scen-check-checklist-missing` | fixture input/template/configuration changed | 0 | remaining | — |
| `build-tests-run-001-failed-definition-repaired` | `build-tests-run-recovery` | crush cassette / default writer changed | claude 4.03, crush 1.09 | remaining | — |
| `build-tests-run-002-attempt-bound-reports-failing-step` | `build-tests-run-bound` | crush cassette / default writer changed | claude 1.86, crush 1.65 | remaining | — |
| `build-tests-run-003-shared-definition-scenarios-run` | `build-tests-run-shared` | crush cassette / default writer changed | claude 5.58, crush 0.94 | remaining | — |
| `build-tests-run-004-selected-package-must-pass` | `build-tests-run-package` | crush cassette / default writer changed | claude 1.88, crush 0.53 | remaining | — |
| `build-code-run-006-shared-fix-skips-green-peer` | `build-code-shared-fix` | crush cassette / default writer changed; rewritten/replacement Scenario | claude 0.83, crush 1.11 | remaining | — |
| `build-code-run-007-next-walk-finds-regression` | `build-code-cross-walk` | crush cassette / default writer changed; fixture input/template/configuration changed; rewritten/replacement Scenario | claude 1.54, crush 1.26 | remaining | — |
| `build-code-frameworks-005-not-applied-revalidated` | `build-code-not-applied` | crush cassette / default writer changed; rewritten/replacement Scenario | claude 1.14, crush 1.62 | remaining | — |
| `build-code-frameworks-006-not-applied-hits-fix-bound` | `build-code-not-applied-bound` | crush cassette / default writer changed; rewritten/replacement Scenario | claude 1.91, crush 2.07 | remaining | — |
| `build-tests-run-005-prebuilt-engine-without-source` | `build-tests-run-prebuilt` | crush cassette / default writer changed | claude 2.69, crush 0.60 | remaining | — |
| `build-code-spec-005-matched-scenario-embedded` | `build-code-spec-scenario` | crush cassette / default writer changed; fixture input/template/configuration changed | claude 0.97, crush 0.68 | remaining | — |
| `build-code-spec-006-unreadable-product-warned` | `build-code-spec-missing-product` | crush cassette / default writer changed; fixture input/template/configuration changed | claude 0.90, crush 0.79 | remaining | — |
| `build-code-spec-007-unreadable-registry-warned` | `build-code-spec-missing-registry` | crush cassette / default writer changed; fixture input/template/configuration changed | claude 0.78, crush 0.74 | remaining | — |
| `build-tests-run-006-identical-failures-grouped` | `build-tests-run-grouped` | crush cassette / default writer changed | claude 4.64, crush 1.24 | remaining | — |
| `build-tests-run-007-distinct-failures-whole` | `build-tests-run-distinct` | crush cassette / default writer changed | claude 4.41, crush 1.16 | remaining | — |
| `build-code-set-001-partial-repair` | `build-code-partial` | crush cassette / default writer changed; fixture input/template/configuration changed; rewritten/replacement Scenario | claude 1.21, crush 2.04 | remaining | — |
| `build-code-set-002-shared-evidence` | `build-code-grouped` | crush cassette / default writer changed; rewritten/replacement Scenario | claude 0.78, crush 0.77 | remaining | — |
| `build-code-set-003-mixed-synthetic` | `build-code-mixed-build` | crush cassette / default writer changed; rewritten/replacement Scenario | claude 0.89, crush 0.52 | remaining | — |
| `build-code-set-004-missing-result` | `build-code-missing-member` | crush cassette / default writer changed; fixture input/template/configuration changed; rewritten/replacement Scenario | claude 1.90, crush 2.09 | remaining | — |
| `build-code-set-005-shrinking-counter` | `build-code-shrinking-bound` | crush cassette / default writer changed; fixture input/template/configuration changed; rewritten/replacement Scenario | claude 1.98, crush 0.86 | remaining | — |
| `build-code-confirmation-001-passes` | `build-code-no-confirmation-passes` | crush cassette / default writer changed; rewritten/replacement Scenario | claude 0.85, crush 0.75 | remaining | — |
| `build-code-confirmation-002-retries` | `build-code-no-confirmation-retries` | crush cassette / default writer changed; rewritten/replacement Scenario | claude 1.73, crush 1.07 | remaining | — |
| `build-code-confirmation-003-bound` | `build-code-no-confirmation-bound` | crush cassette / default writer changed; rewritten/replacement Scenario | claude 2.60, crush 1.87 | remaining | — |
| `build-code-writer-001-nonzero` | `build-code-writer-nonzero` | crush cassette / default writer changed; fixture input/template/configuration changed; rewritten/replacement Scenario | claude 1.69, crush 0.93 | remaining | — |
| `build-code-writer-002-empty` | `build-code-writer-empty` | crush cassette / default writer changed; fixture input/template/configuration changed; rewritten/replacement Scenario | claude 0.84, crush 0.59 | remaining | — |
| `build-code-writer-003-bound` | `build-code-writer-bound` | crush cassette / default writer changed; fixture input/template/configuration changed; rewritten/replacement Scenario | claude 1.76, crush 1.38 | remaining | — |
| `build-code-claude-005-replay` | `build-code-claude-replay` | fixture input/template/configuration changed | claude 1.09 | remaining | — |
| `build-code-claude-006-failure` | `build-code-claude-failure` | fixture input/template/configuration changed | claude 0.76 | remaining | — |
| `build-code-codex-001-selection` | `build-code-codex-selection` | fixture input/template/configuration changed | claude 0.68, codex 0.58 | remaining | — |
| `build-code-codex-002-nonzero` | `build-code-codex-nonzero` | fixture input/template/configuration changed | claude 0.80, codex 0.47 | remaining | — |
| `build-code-codex-003-missing-root` | `build-code-codex-missing-root` | fixture input/template/configuration changed | claude 1.07, codex 0.74 | remaining | — |
| `claude-guard-001-read-only-tool-allowed` | `claude-guard-read-only-tool` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-002-shell-denied-without-grant` | `claude-guard-shell-denied` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-004-unknown-tool-denied` | `claude-guard-unknown-tool` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-005-missing-policy-denied` | `claude-guard-missing-policy` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-006-unreadable-policy-denied` | `claude-guard-unreadable-policy` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-007-payload-names-the-call` | `claude-guard-hook-payload` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-writes-001-write-inside-root-allowed` | `claude-guard-write-inside-root` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-writes-002-write-outside-roots-denied` | `claude-guard-write-outside-roots` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-writes-003-symlink-escape-denied` | `claude-guard-symlink-escape` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-writes-004-shared-prefix-denied` | `claude-guard-shared-prefix` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-writes-006-unnamed-path-denied` | `claude-guard-unnamed-path` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-reads-001-outside-project-denied` | `claude-guard-read-outside` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-reads-002-relative-escape-denied` | `claude-guard-read-relative-escape` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-reads-003-symlink-escape-denied` | `claude-guard-read-symlink-escape` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-reads-004-unnamed-path-allowed` | `claude-guard-read-unnamed-path` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-reads-005-missing-read-root-denied` | `claude-guard-read-missing-root` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-guard-reads-006-relative-path-allowed` | `claude-guard-read-relative-path` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-replay-001-whole-command` | `claude-replay-whole-command` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-replay-002-selected-command` | `claude-replay-selected-command` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-replay-003-shell-operator` | `claude-replay-shell-operator` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-replay-004-assignment` | `claude-replay-assignment` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-replay-005-another-program` | `claude-replay-another-program` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-replay-006-extra-flag` | `claude-replay-extra-flag` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-replay-007-double-quotes` | `claude-replay-double-quotes` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-replay-008-wrong-directory` | `claude-replay-wrong-directory` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `claude-replay-009-no-command` | `claude-replay-no-command` | fixture input/template/configuration changed; rewritten/replacement Scenario | 0 | remaining | — |
| `build-code-claude-007-unrepresentable` | `build-code-unrepresentable-replay` | crush cassette / default writer changed; fixture input/template/configuration changed; rewritten/replacement Scenario | claude 0.91, crush 0.65 | remaining | — |

Recorded: 5; remaining: 101; historical service CLI minutes: 176.45; new recorded service CLI minutes: 10.46.
