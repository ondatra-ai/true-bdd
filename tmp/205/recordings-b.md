# Issue 203 recordings: second half

Baseline: `d707d7aeaea835e9a1c709f09505d75ee3d79904`. These are the last 55 remaining rows of `tmp/205/recordings.md`, in manifest order. Each recording runs alone using the architecture record command and its anchored Scenario selector, then passes offline replay. CLI minutes exclude judges. No cassette is hand-edited.

| Scenario | Fixture | Migration status | #205 status | Recorded CLI minutes |
| --- | --- | --- | --- | --- |
| `build-tests-run-003-shared-definition-scenarios-run` | `build-tests-run-shared` | recorded | recorded on #203-b; replay passed | 10.06 |
| `build-tests-run-004-selected-package-must-pass` | `build-tests-run-package` | recorded | recorded on #203-b; replay passed | 2.63 |
| `build-code-run-006-shared-fix-skips-green-peer` | `build-code-shared-fix` | recorded | recorded on #203-b; replay passed | 1.23 |
| `build-code-run-007-next-walk-finds-regression` | `build-code-cross-walk` | recorded | recorded on #203-b; replay passed | 3.59 |
| `build-code-frameworks-005-not-applied-revalidated` | `build-code-not-applied` | recorded | recorded on #203-b; replay passed | 3.18 |
| `build-code-frameworks-006-not-applied-hits-fix-bound` | `build-code-not-applied-bound` | recorded | recorded on #203-b; replay passed | 4.94 |
| `build-tests-run-005-prebuilt-engine-without-source` | `build-tests-run-prebuilt` | recorded | recorded on #203-b; replay passed | 4.93 |
| `build-code-spec-005-matched-scenario-embedded` | `build-code-spec-scenario` | recorded | recorded on #203-b; replay passed | 1.82 |
| `build-code-spec-006-unreadable-product-warned` | `build-code-spec-missing-product` | recorded | recorded on #203-b; replay passed | 1.83 |
| `build-code-spec-007-unreadable-registry-warned` | `build-code-spec-missing-registry` | recorded | recorded on #203-b; replay passed | 1.89 |
| `build-tests-run-006-identical-failures-grouped` | `build-tests-run-grouped` | recorded | recorded on #203-b; replay passed | 6.09 |
| `build-tests-run-007-distinct-failures-whole` | `build-tests-run-distinct` | recorded | recorded on #203-b; replay passed | 7.59 |
| `build-code-set-001-partial-repair` | `build-code-partial` | recorded | recorded on #203-b; replay passed | 3.01 |
| `build-code-set-002-shared-evidence` | `build-code-grouped` | recorded | recorded on #203-b; replay passed | 1.50 |
| `build-code-set-003-mixed-synthetic` | `build-code-mixed-build` | recorded | recorded on #203-b; replay passed | 1.78 |
| `build-code-set-004-missing-result` | `build-code-missing-member` | recorded | recorded on #203-b; replay passed | 3.73 |
| `build-code-set-005-shrinking-counter` | `build-code-shrinking-bound` | recorded | recorded on #203-b; replay passed | 2.42 |
| `build-code-confirmation-001-passes` | `build-code-no-confirmation-passes` | recorded | recorded on #203-b; replay passed | 1.36 |
| `build-code-confirmation-002-retries` | `build-code-no-confirmation-retries` | recorded | recorded on #203-b; replay passed | 3.18 |
| `build-code-confirmation-003-bound` | `build-code-no-confirmation-bound` | recorded | recorded on #203-b; replay passed | 3.88 |
| `build-code-writer-001-nonzero` | `build-code-writer-nonzero` | remaining | pending | — |
| `build-code-writer-002-empty` | `build-code-writer-empty` | remaining | pending | — |
| `build-code-writer-003-bound` | `build-code-writer-bound` | remaining | pending | — |
| `build-code-claude-005-replay` | `build-code-claude-replay` | remaining | pending | — |
| `build-code-claude-006-failure` | `build-code-claude-failure` | remaining | pending | — |
| `build-code-codex-001-selection` | `build-code-codex-selection` | remaining | pending | — |
| `build-code-codex-002-nonzero` | `build-code-codex-nonzero` | remaining | pending | — |
| `build-code-codex-003-missing-root` | `build-code-codex-missing-root` | remaining | pending | — |
| `claude-guard-001-read-only-tool-allowed` | `claude-guard-read-only-tool` | remaining | pending | — |
| `claude-guard-002-shell-denied-without-grant` | `claude-guard-shell-denied` | remaining | pending | — |
| `claude-guard-004-unknown-tool-denied` | `claude-guard-unknown-tool` | remaining | pending | — |
| `claude-guard-005-missing-policy-denied` | `claude-guard-missing-policy` | remaining | pending | — |
| `claude-guard-006-unreadable-policy-denied` | `claude-guard-unreadable-policy` | remaining | pending | — |
| `claude-guard-007-payload-names-the-call` | `claude-guard-hook-payload` | remaining | pending | — |
| `claude-guard-writes-001-write-inside-root-allowed` | `claude-guard-write-inside-root` | remaining | pending | — |
| `claude-guard-writes-002-write-outside-roots-denied` | `claude-guard-write-outside-roots` | remaining | pending | — |
| `claude-guard-writes-003-symlink-escape-denied` | `claude-guard-symlink-escape` | remaining | pending | — |
| `claude-guard-writes-004-shared-prefix-denied` | `claude-guard-shared-prefix` | remaining | pending | — |
| `claude-guard-writes-006-unnamed-path-denied` | `claude-guard-unnamed-path` | remaining | pending | — |
| `claude-guard-reads-001-outside-project-denied` | `claude-guard-read-outside` | remaining | pending | — |
| `claude-guard-reads-002-relative-escape-denied` | `claude-guard-read-relative-escape` | remaining | pending | — |
| `claude-guard-reads-003-symlink-escape-denied` | `claude-guard-read-symlink-escape` | remaining | pending | — |
| `claude-guard-reads-004-unnamed-path-allowed` | `claude-guard-read-unnamed-path` | remaining | pending | — |
| `claude-guard-reads-005-missing-read-root-denied` | `claude-guard-read-missing-root` | remaining | pending | — |
| `claude-guard-reads-006-relative-path-allowed` | `claude-guard-read-relative-path` | remaining | pending | — |
| `claude-replay-001-whole-command` | `claude-replay-whole-command` | remaining | pending | — |
| `claude-replay-002-selected-command` | `claude-replay-selected-command` | remaining | pending | — |
| `claude-replay-003-shell-operator` | `claude-replay-shell-operator` | remaining | pending | — |
| `claude-replay-004-assignment` | `claude-replay-assignment` | remaining | pending | — |
| `claude-replay-005-another-program` | `claude-replay-another-program` | remaining | pending | — |
| `claude-replay-006-extra-flag` | `claude-replay-extra-flag` | remaining | pending | — |
| `claude-replay-007-double-quotes` | `claude-replay-double-quotes` | remaining | pending | — |
| `claude-replay-008-wrong-directory` | `claude-replay-wrong-directory` | remaining | pending | — |
| `claude-replay-009-no-command` | `claude-replay-no-command` | remaining | pending | — |
| `build-code-claude-007-unrepresentable` | `build-code-unrepresentable-replay` | remaining | pending | — |

Recorded and replayed: 20 of 55; remaining: 35; service CLI minutes: 70.64.
