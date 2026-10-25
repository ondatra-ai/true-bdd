# Issue 205 implementation and continuation

Branch: `impl/205-confine-codex-apply`; starting commit: `6e061cf75bbb946a7a6417fe86e30bf6e63c3a3d`. The code run commits locally; it never pushes or writes to GitHub.

## Write-root contract

| Kind | Grant | Safe creation |
| --- | --- | --- |
| Directory / terminal `/**` | Exact subtree, with new descendants allowed | Root and missing literal parents via `disk.DirWithin` |
| Existing single file | Exact file, without a parent write grant | File kept intact until the turn writes it |
| Missing single file | Exact absent filename | Only its literal parents are created |
| Configured glob | Existing wildcard anchors expand to exact files or recursive directory roots | Missing literal directory suffixes may be created; wildcard filenames are frozen for that turn |
| Scratch | Host tmp plus controlled HOME/TMPDIR/caches | Rooted creation; outside paths refuse |

SourceWriteRoots and Write/Edit patterns are combined, preserving #201 callers that supply source roots alongside scratch tools. Escaping roots, dangling outside links, a Host-project-wide root and roots reaching generated homes are refused before spawning; case variants cannot override generated-home protection. No profile entry silently grants an arbitrary wildcard’s fixed prefix.

## Runtime and lifetime

Go and ADR-0021’s OS/toolchain exceptions remain. Optional installed node, npm and Docker clients are explicit read grants; npm additionally reads its package and Mach-O runtimes read their enumerated libraries, library directories and installation links. Homebrew opt directories supply link metadata; unrelated canonical targets remain restricted. Commands inherit no environment; npm is offline and uses scratch cache/config, Docker uses scratch config without daemon/socket/network access, and `OPENSSL_CONF=/dev/null` disables outside configuration. Each apply uses a generated sign-in-only home, JSON events and cleanup. Plans/validation remain read-only. Only build code receives replay forms and started-failure recovery. Configured context remains 1050000, not a verified 1M prompt.

## TDD and evidence

Owner-confirmed seams were role dispatch, profile grants for each root kind, escaping roots, read-only planning/validation, retained build-code forms and runtime availability. Recorded red/green loops cover apply-role eligibility, file/glob grants, installed runtimes, existing single-file modes, dangling links, protected-home overrides, explicit source roots alongside scratch patterns, and an absent literal filename in configured test patterns. Tests exercise provider/applier and CLI seams; no sub-agent was used. `probes.md` lists live commands, calibration failures and final results.

All four requested fixtures were recorded individually and successfully, in the required order; no rejected recording was promoted and no cassette was hand-edited. `recordings.md` is the copied, updated 108-row ledger and names all 97 deferred migration Scenarios. Ignored helpers and JSON status remain in this worktree at `tmp/205/`. Continuation records the other migration fixtures on this branch, not in #205. No usage limit occurred.

The recording byte audit compares every blob with the starting commit: 1563 baseline files, 1528 byte-identical, 1597 current files, 74 changed/new/deleted paths, all within the four authorized fixtures. The seven unaffected fixtures among #203’s eight completed ones remain byte-identical. Generated-authentication scan after recordings: zero files remain. An optional `git diff --check` reports one trailing blank line in real codex stderr (`build-tests-fix-happy-path/cassettes/codex-001/stderr`); the cassette is preserved verbatim, as required, and the recording/alint gates accept it.

The recorded subset replay selected and passed all 155 available Registry Scenarios; the remaining 97 are listed in the ledger. Full replay awaits their #203 continuation. Required validation commands are `go run ./scripts/cmd/alint <changed files>`, `go build ./...`, `go test ./...` and `go run ./scripts/cmd/gates run --changed main`; command logs and the final report carry results.

## Specification divergence review

Reviewed product, architecture, Registry, Story, glossary meanings, README and matching area rules. Closed the stale default-coder document-edit blocker claims, writer-only confinement claims, source-root file/directory mismatch and snapshot treatment of generated state. The new Story and two generated tests state the implemented behavior using existing Catalogue phrases and Word mappings; no new domain term is introduced. ADR-0024 amends 0021/0023 and links ADR-0017’s guarantee to codex applies. Its boundary explicitly describes frozen glob matches, Docker-client-only access and unchanged read-only planning, so documents do not promise broader permissions or recovery. No Specification divergence remains within #205’s scope.

Out of scope: the other #203 recordings, #197, #198 and the drill. Non-macOS runtime dependencies and 1M capacity were not verified. No unresolved question requires an Owner answer.

## File-by-file map

| Path | Change |
| --- | --- |
| `.claude/rules/bdd-harness.md` | Describe all-apply confinement, exact roots, runtime exceptions and unchanged plan/recovery boundaries. |
| `.claude/rules/engine-internals.md` | Describe all-apply confinement, exact roots, runtime exceptions and unchanged plan/recovery boundaries. |
| `README.md` | Describe all-apply confinement, exact roots, runtime exceptions and unchanged plan/recovery boundaries. |
| `docs/adr/0017-crush-reads-only-inside-the-project.md` | Record the confinement extension or link superseded/amended decisions to ADR-0024. |
| `docs/adr/0021-codex-writer-confined-to-the-host.md` | Record the confinement extension or link superseded/amended decisions to ADR-0024. |
| `docs/adr/0023-remove-crush-from-product.md` | Record the confinement extension or link superseded/amended decisions to ADR-0024. |
| `docs/adr/0024-confine-every-codex-apply.md` | Record the confinement extension or link superseded/amended decisions to ADR-0024. |
| `docs/architecture/architecture.yaml` | Describe all-apply confinement, exact roots, runtime exceptions and unchanged plan/recovery boundaries. |
| `docs/product/product.yaml` | Describe all-apply confinement, exact roots, runtime exceptions and unchanged plan/recovery boundaries. |
| `docs/product/stories/codex-apply-confinement.yaml` | Specify scratch-document and configured-test-root apply behavior. |
| `docs/scenarios.yaml` | Add the two real-codex confinement Scenarios without weakening existing Scenarios. |
| `pkg/testkit/aiproxy/record.go` | Skip generated homes/runtime scratch before per-call snapshot traversal. |
| `pkg/testkit/cmd/coverage/override_sync_test.go` | Declare the new closure-check fixture purpose without weakening prompt-parity checks. |
| `pkg/testkit/runner/runner.go` | Skip the same generated state before whole-run snapshot traversal. |
| `services/bdd-cli/adapters/ai/codex_apply_probe_test.go` | Regression/behavior coverage for apply confinement, exact roots, runtime grants or retained read-only/writer behavior. |
| `services/bdd-cli/adapters/ai/codex_apply_roots_test.go` | Regression/behavior coverage for apply confinement, exact roots, runtime grants or retained read-only/writer behavior. |
| `services/bdd-cli/adapters/ai/codex_provider.go` | Prepare/clean private homes and archive JSON events for every codex apply; retain writer-only recovery. |
| `services/bdd-cli/adapters/ai/codex_runtime.go` | Grant installed runtime files/libraries, create controlled runtime links and isolate npm/Docker/OpenSSL state. |
| `services/bdd-cli/adapters/ai/codex_runtime_libraries.go` | Read Mach-O dependencies through Disk access and enumerate runtime libraries without a Cellar-wide grant. |
| `services/bdd-cli/adapters/ai/codex_write_roots.go` | Combine explicit roots and tool patterns; grant exact files/subtrees, expand glob anchors and refuse escapes/protected homes. |
| `services/bdd-cli/adapters/ai/codex_writer.go` | Retain generated configuration and Go isolation; validate scratch, provision rooted directories and resolve dangling links. |
| `services/bdd-cli/adapters/ai/codex_writer_test.go` | Regression/behavior coverage for apply confinement, exact roots, runtime grants or retained read-only/writer behavior. |
| `services/bdd-cli/adapters/ai/execution_mode.go` | Keep service-directory normalization separate from configured patterns; preserve exact single-file roots. |
| `services/bdd-cli/adapters/ai/execution_mode_test.go` | Regression/behavior coverage for apply confinement, exact roots, runtime grants or retained read-only/writer behavior. |
| `services/bdd-cli/adapters/ai/provider.go` | Carry the apply-confinement flag separately from build-code recovery. |
| `services/bdd-cli/adapters/ai/provider_args_internal_test.go` | Regression/behavior coverage for apply confinement, exact roots, runtime grants or retained read-only/writer behavior. |
| `services/bdd-cli/adapters/ai/router.go` | Select codex confinement by apply role and CLI, independently of build-code options. |
| `services/bdd-cli/adapters/ai/router_writer_internal_test.go` | Regression/behavior coverage for apply confinement, exact roots, runtime grants or retained read-only/writer behavior. |
| `services/bdd-cli/adapters/ai/tmp/true-bdd.log.json` | Align engine implementation and permission contract. |
| `services/bdd-cli/internal/domain/models/provider/execution_mode.go` | Document shared write patterns and independent codex role confinement. |
| `services/bdd-cli/internal/domain/models/provider/turn_options.go` | Document that the explicit writer option remains build-code-only. |
| `services/bdd-cli/internal/infrastructure/cli/codex/codex.go` | Document all-apply profile selection; retain existing profile argv and polling behavior. |
| `services/bdd-cli/internal/infrastructure/cli/codex/runtime.go` | Resolve the optional node/npm/docker executables through the typed CLI wrapper. |
| `tests/bdd-cli/build_tests_test.go` | Generated by true-bdd build tests --fix from the two new Registry Scenarios. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/claude-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/claude-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/claude-002/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/claude-002/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/claude-003/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/claude-003/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/claude-004/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/claude-004/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/codex-001/fsdiff/after/tests/mcp/steps/definitions.go` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/codex-001/fsdiff/after/tests/mcp/steps/steps.go` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/codex-001/fsdiff/after/tmp/{{RUN_DIR}}/codex-turn003-codex-answer.txt` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/codex-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/codex-001/stderr` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/codex-001/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/codex-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/build-tests-fix-happy-path/cassettes/golden.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-001/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-002/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-002/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-002/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-003/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-003/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-003/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-004/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-004/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/claude-004/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/codex-001/fsdiff/after/tests/mcp/steps/definitions.go` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/codex-001/fsdiff/after/tmp/{{RUN_DIR}}/codex-turn003-codex-answer.txt` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/codex-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/codex-001/stderr` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/codex-001/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/codex-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/cassettes/golden.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/input/.claude/settings.json` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/input/CLAUDE.md` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/input/docs/architecture/architecture.yaml` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/input/docs/scenarios.yaml` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/input/go.mod` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/input/tests/mcp/scenarios/scenarios.go` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/input/tests/mcp/steps/state.go` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/input/true-bdd/true-bdd.yaml` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-build-tests/prep.sh` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/claude-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/claude-001/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/claude-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/claude-002/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/claude-002/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/claude-002/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/codex-001/fsdiff/after/tmp/{{RUN_DIR}}/codex-turn003-codex-answer.txt` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/codex-001/fsdiff/after/tmp/{{RUN_DIR}}/product.yaml` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/codex-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/codex-001/stderr` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/codex-001/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/codex-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/cassettes/golden.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/input/docs/product/product.yaml` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/input/docs/product/stories/doc-outline-catalogued.yaml` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/codex-apply-us-refine/input/true-bdd/checklists/us-refine.yaml` | New confinement fixture input/prep, preserving the source fixture’s behavior; build-tests config declares its exact recursive root. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-002/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-002/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-003/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-003/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-004/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-004/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-005/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-005/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-006/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-006/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-007/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/claude-007/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/codex-001/fsdiff/after/tmp/{{RUN_DIR}}/codex-turn004-codex-answer.txt` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/codex-001/fsdiff/after/tmp/{{RUN_DIR}}/scenarios.yaml` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/codex-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/codex-001/stderr` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/codex-001/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/codex-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/crush-001/fsdiff/after/tmp/{{RUN_DIR}}/scenarios.yaml` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/crush-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/crush-001/stdin` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/crush-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/judge/claude-001/meta.json` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/fixtures/us-apply-duplicate-collapse/cassettes/judge/claude-001/stdout` | Real-CLI recording, or retired transport file replaced by that recording; never hand-edited. |
| `tests/bdd-cli/us_refine_test.go` | Generated by true-bdd build tests --fix from the two new Registry Scenarios. |
| `tmp/205/implementation.md` | Local reviewable run evidence, file map, probes or copied recording ledger. |
| `tmp/205/probes.md` | Local reviewable run evidence, file map, probes or copied recording ledger. |
| `tmp/205/recordings.md` | Local reviewable run evidence, file map, probes or copied recording ledger. |
| `services/bdd-cli/internal/app/bootstrap/container.go` | Select exact configured-pattern mode for build-tests. |
| `services/bdd-cli/internal/app/generators/validate/fix_applier.go` | Keep configured patterns separate from service directory roots when selecting the execution mode. |

## Completed validation

- Alint passed over all 121 existing changed files; the final probe-test edit also passed scoped alint.
- `go build ./...` and `go build -o ./bin/true-bdd ./services/bdd-cli` passed on the final source.
- `go test ./...` passed on the final source.
- Recorded-subset replay passed 155/155, with 97 pending migration Scenarios explicitly excluded and listed.
- Final live profile suite passed, 91.920 seconds.
- Gates are run by hand before the local commit; `tmp/205/gates-final.log` preserves that result.
