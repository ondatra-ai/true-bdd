# Issue 203 tracked-file inventory

Baseline: `71d93858ec420d6df9e903f3587b8e4d9f5b3482`. Exact command: `git grep -il crush`. Each tracked hit is listed below. #204 exclusions are intentional; #205 confinement is deferred. Historical Q&A is immutable decision evidence, retained rather than rewritten.

| File | Disposition |
| --- | --- |
| `.alint.yml` | Remove only crush wrapper entry and no-root-crush-config; retain generic pkg/cli/** allowance. |
| `.claude/rules/bdd-harness.md` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `.claude/rules/engine-internals.md` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `.claude/rules/go-conventions.md` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `.claude/rules/markdown.md` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `.claude/skills/common-codex/SKILL.md` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `.golangci.yaml` | Remove only crush wrapper entry and no-root-crush-config; retain generic pkg/cli/** allowance. |
| `AGENTS.md` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `GLOSSARY.md` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `README.md` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/adr/0016-scripted-codex-delegation.md` | Retain ADR; mark 0017/0018/0020 superseded by 0023; remove current product claims in other ADRs, retain tooling history pending #204. |
| `docs/adr/0017-crush-reads-only-inside-the-project.md` | Retain ADR; mark 0017/0018/0020 superseded by 0023; remove current product claims in other ADRs, retain tooling history pending #204. |
| `docs/adr/0018-crush-writer-replay-command.md` | Retain ADR; mark 0017/0018/0020 superseded by 0023; remove current product claims in other ADRs, retain tooling history pending #204. |
| `docs/adr/0019-failing-set-repair-and-engine-verdict.md` | Retain ADR; mark 0017/0018/0020 superseded by 0023; remove current product claims in other ADRs, retain tooling history pending #204. |
| `docs/adr/0020-crush-writer-response-watch.md` | Retain ADR; mark 0017/0018/0020 superseded by 0023; remove current product claims in other ADRs, retain tooling history pending #204. |
| `docs/adr/0021-codex-writer-confined-to-the-host.md` | Retain ADR; mark 0017/0018/0020 superseded by 0023; remove current product claims in other ADRs, retain tooling history pending #204. |
| `docs/adr/0022-confined-claude-writer.md` | Retain ADR; mark 0017/0018/0020 superseded by 0023; remove current product claims in other ADRs, retain tooling history pending #204. |
| `docs/agents/drill.md` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `docs/architecture/architecture.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/doc-universe.md` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/product/product.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/product/stories/build-code-failing-set.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/product/stories/build-code-frameworks-jest-playwright-go.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/product/stories/build-code-missing-confirmation.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/product/stories/build-code-run-fix-loop.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/product/stories/build-code-writer-failures.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/product/stories/crush-guard-reads-project-root.yaml` | Delete crush Story claims; move surviving canonical-command invariant to retained/new Claude Story. |
| `docs/product/stories/crush-guard-tool-calls.yaml` | Delete crush Story claims; move surviving canonical-command invariant to retained/new Claude Story. |
| `docs/product/stories/crush-guard-writes-write-roots.yaml` | Delete crush Story claims; move surviving canonical-command invariant to retained/new Claude Story. |
| `docs/product/stories/crush-writer-test-command.yaml` | Delete crush Story claims; move surviving canonical-command invariant to retained/new Claude Story. |
| `docs/product/stories/engine-config-models-and-roots.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/product/stories/engine-configured-commands.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/product/stories/engine-routes-agent-clis.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `docs/qna/20261021-112516-self-spec-and-doc-validation-chain.yaml` | Retain historical decision evidence; reference from ADR-0023 (not a current dependency). |
| `docs/scenarios.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/cli/crush/crush.go` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `pkg/enginelog/enginelog.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/shell/doc.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/aiproxy/fsdiff_filter.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/aiproxy/main.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/aiproxy/normalize_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/aiproxy/record.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/aiproxy/stdin_collector.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/cmd/coverage/override_sync_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reporter/artifact.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reporter/artifact_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reporter/engine_log.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reporter/invocation.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reporter/invocation_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reporter/operation_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reporter/phase_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reporter/testdata_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reporter/vocabulary.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/reportserver/diff_key.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/runner/golden_compare_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/runner/judge_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/runner/judge_model_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `pkg/testkit/runner/required_clis.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `scripts/codexrun/attempt.go` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `scripts/codexrun/options.go` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `scripts/codexrun/processes_test.go` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `scripts/codexrun/retry_test.go` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `scripts/codexrun/runner_test.go` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `scripts/drill/audit.go` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `scripts/drill/audit_test.go` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `scripts/drill/runner.go` | Leave for #204 (tooling/instructions); excluded from #203 product cleanup. |
| `services/bdd-cli/adapters/ai/claude_guard_identity_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/claude_guard_policy.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/claude_guard_policy_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/claude_provider.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/claude_writer_config.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/cli_invocation.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/crush_config_internal_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_guard_escape_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_guard_policy.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_guard_policy_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_guard_probe.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_guard_reads.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_guard_reads_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_provider.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_provider_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_test_command_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/crush_tool_call.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/adapters/ai/execution_mode_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/provider.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/provider_args_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/result_schema.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/result_schema_internal_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/router.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/adapters/ai/router_watch_internal_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/cmd/claude_guard.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/cmd/crush_guard.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/cmd/crush_guard_internal_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/cmd/root.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/app/bootstrap/model_override_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/app/generators/validate/file_content_extractor.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/app/generators/validate/file_content_extractor_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/app/generators/validate/fix_applier.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/app/generators/validate/writer_template_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/app/runner/apply_confirmation_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/app/runner/writer_failure.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/domain/models/provider/execution_mode.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/domain/models/provider/model_ref.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/domain/models/provider/model_ref_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/domain/models/provider/registry_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/domain/models/provider/test_command.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/domain/models/provider/turn_options.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/infrastructure/cli/crush/crush.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/internal/infrastructure/cli/crush/crush_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/internal/infrastructure/cli/crush/response_watch.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/internal/infrastructure/cli/crush/response_watch_test.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/internal/infrastructure/cli/truebdd/truebdd.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/infrastructure/testrunner/replay_grant.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `services/bdd-cli/internal/pkg/errors/crush_turn_error.go` | Delete crush-specific implementation/tests; extract shared replay and path helpers under provider-neutral names; retain writer-routing checks. |
| `services/bdd-cli/internal/pkg/errors/errors.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/build_code_confirmation_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/build_code_set_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/build_code_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/build_code_writer_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/build_tests_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/coverage_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/crush_guard_test.go` | Delete crush-specific fixture/test/step; preserve shared Claude setup and surviving invariants per Scenario table. |
| `tests/bdd-cli/crush_replay_test.go` | Delete crush-specific fixture/test/step; preserve shared Claude setup and surviving invariants per Scenario table. |
| `tests/bdd-cli/fixtures/build-code-claude-failure/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-claude-replay/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-codex-missing-root/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-codex-nonzero/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-codex-nonzero/recording.md` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-codex-selection/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-cross-walk/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-cross-walk/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-directory-architecture/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-fix-named-test/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-go-package-does-not-compile/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-grouped/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-jest-fix-named-test/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-missing-member/cassettes/claude-002/stdout` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-missing-member/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-missing-member/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-mixed-build/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-no-confirmation-bound/cassettes/claude-003/stdout` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-no-confirmation-bound/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-no-confirmation-bound/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-no-confirmation-bound/cassettes/crush-003/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-no-confirmation-passes/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-no-confirmation-retries/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-no-confirmation-retries/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-not-applied-bound/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-not-applied-bound/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-not-applied-bound/cassettes/crush-003/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-not-applied/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-not-applied/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-partial/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-partial/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-playwright-failing-assertion/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-playwright-nextjs/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-shared-fix/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-shrinking-bound/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-shrinking-bound/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-spec-missing-product/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-spec-missing-product/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-spec-missing-registry/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-spec-missing-registry/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-spec-scenario/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-spec-scenario/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-unrepresentable-replay/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-bound/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-bound/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-bound/cassettes/crush-003/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-bound/record-bin/crush` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-writer-bound/recording.md` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-writer-empty/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-empty/record-bin/crush` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-writer-empty/recording.md` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-writer-nonzero/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-nonzero/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-nonzero/record-bin/crush` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-writer-nonzero/recording.md` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-writer-replay/cassettes/claude-001/stdout` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-replay/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-replay/cassettes/crush-001/stdin` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-code-writer-replay/input/replay_hook.py` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-code-writer-replay/input/templates/build-code.fix-applier.prompt.tpl` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-tests-fix-default-write-roots/cassettes/claude-002/stdout` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-fix-default-write-roots/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-tests-fix-from-spec/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-host-crush-config/cassettes/claude-002/stdout` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-host-crush-config/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-host-crush-config/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-host-crush-config/input/CLAUDE.md` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-tests-host-crush-config/input/docs/scenarios.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-tests-malformed-model-entries/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-tests-run-bound/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-bound/cassettes/crush-001/stdout` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-distinct/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-distinct/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-grouped/cassettes/claude-004/stdout` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-grouped/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-grouped/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-package/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-prebuilt/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-recovery/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-recovery/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-shared/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-run-shared/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/build-tests-unconfigured-registry/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-tests-undeclared-architecture/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/build-tests-unknown-cli-tier/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/crush-guard-hook-payload/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-missing-policy/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-read-missing-root/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-read-only-tool/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-read-outside/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-read-relative-escape/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-read-relative-path/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-read-symlink-escape/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-read-unnamed-path/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-relative-path/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-shared-prefix/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-shell-allowed/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-shell-denied/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-symlink-escape/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-unknown-tool/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-unnamed-path/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-unreadable-policy/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-write-inside-root/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-guard-write-outside-roots/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-another-program/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-assignment/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-double-quotes/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-extra-flag/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-job-kill/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-job-output/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-job-without-command/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-kill-without-command/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-no-command/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-selected-command/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-shell-operator/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-whole-command/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/crush-replay-wrong-directory/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/fix-loop-attempts-exhausted/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/fix-loop-exit-steps/cassettes/claude-002/stdout` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/fix-loop-own-words/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/fix-loop-refine-feedback/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/fix-loop-refix-stops/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/fix-loop-refix-stops/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/fix-loop-walks-exhausted-terms/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-checklist-missing/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/scen-check-claude-turn-fails/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/scen-check-crush-turn-fails/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-crush-turn-fails/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-crush-turn-fails/input/docs/scenarios.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/scen-check-crush-turn-fails/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/scen-check-fix-adds-measurable-criterion/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-adds-missing-step/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/cassettes/crush-003/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/cassettes/crush-004/fsdiff/after/tmp/{{RUN_DIR}}/scenarios_yaml.yaml` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/cassettes/crush-004/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/cassettes/crush-005/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/cassettes/crush-006/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/cassettes/golden.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/input/docs/scenarios.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/scen-check-fix-by-crush/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/scen-check-fix-merges-phrases/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-records-mapping/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-records-phrase/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-records-term/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-rewrites-description-qualifier/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-rewrites-description/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-rewrites-forbidden-verb/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-rewrites-implementation/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-rewrites-phrase-meaning/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-rewrites-phrase-use/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-rewrites-step-qualifier/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-fix-rewrites-subjective-outcome/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-prompt-names-its-tiers/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-prompt-names-its-tiers/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-prompt-names-its-tiers/cassettes/crush-003/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/scen-check-prompt-names-its-tiers/input/true-bdd/checklists/scen-check.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/scen-check-prompt-names-its-tiers/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/scen-check-prompt-tier-unconfigured/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/scen-check-undeclared-registry/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-apply-relocated-registry/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-apply-relocated-registry/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/us-apply-rewalk-converges/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-apply-rewalk-converges/cassettes/crush-002/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-apply-undeclared-registry/input/true-bdd/true-bdd.yaml` | Convert active model/transport/template/fixture wording to codex; retire crush-only routing fixture; re-record changed requests. |
| `tests/bdd-cli/fixtures/us-refine-fix-desc-qualifier/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-refine-fix-descriptions/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-refine-fix-forbidden-verb/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-refine-fix-keeps-feature/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-refine-fix-records-phrase/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-refine-fix-step-qualifier/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/fixtures/us-refine-fix-steps/cassettes/crush-001/meta.json` | Delete with retired fixture, or re-record affected retained fixture; never edit cassette bytes. See recordings manifest. |
| `tests/bdd-cli/scen_check_test.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/scenarios/scenarios.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/steps/cli_steps.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `tests/bdd-cli/steps/crush_guard_steps.go` | Delete crush-specific fixture/test/step; preserve shared Claude setup and surviving invariants per Scenario table. |
| `tests/bdd-cli/steps/state.go` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
| `true-bdd/true-bdd.yaml` | Remove product crush dependency/claims, retain engine behavior and update codex/default wording; tests and requests updated as required. |
