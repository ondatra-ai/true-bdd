# Specification diet, 2 November 2026 (#287)

This audit compares starting commit `8d877d19583887d579d7b9230504f86ac860adbf` with this change on `impl/287-spec-diet`. Counts use Unicode-whitespace-separated words in decoded prose values; sizes are UTF-8 file bytes. “Deleted” means removed, with no text relocated to another document. References below identify pre-existing contracts that already fix the same behaviour, not destinations for removed text.

The Owner’s later instruction on 2 November excludes **all architecture.prompt_data entries**, the Public interface document’s `format:` fields and the prompt templates. Their bytes are unchanged. No context builder or prompt template changed for #286. Concepts, including all 19 `means` values and their term lists, are unchanged.

## Measurements

| Document | Bytes before | Bytes after | Reduction |
| --- | ---: | ---: | ---: |
| `docs/product/product.yaml` | 63,544 | 19,214 | 69.8% |
| `docs/architecture/architecture.yaml` | 192,821 | 122,452 | 36.5% |

Glossary: 52 → 43 terms; 6,007 → 954 meaning words. All retained terms are declared by Concepts; all have exactly one Word mapping.

## Five largest prose cuts

| Piece | Words before → after | Why |
| --- | ---: | --- |
| glossary command | 1487 → 23 | Repeated procedural contracts already fixed by the command/Concept Stories, Registry and Public interface; retained the meaning or unique mechanics. |
| product description.summary | 940 → 113 | Repeated concepts, closure, linking and repair protocols already fixed by concepts.yaml, concept-link.yaml, concept-words.yaml and the shipped checklists; kept product purpose and added the refusal/counting contract. |
| glossary fix loop | 678 → 36 | Repeated procedural contracts already fixed by the command/Concept Stories, Registry and Public interface; retained the meaning or unique mechanics. |
| mapping fix loop.technical | 543 → 109 | Repeated procedural contracts already fixed by the command/Concept Stories, Registry and Public interface; retained the meaning or unique mechanics. |
| mapping command.technical | 421 → 63 | Repeated procedural contracts already fixed by the command/Concept Stories, Registry and Public interface; retained the meaning or unique mechanics. |

## Glossary audit

Code names below are packages under `services/true-bdd/internal/` unless stated otherwise: `commands`, `runner`, `checks` and `bootstrap` are under `app/`; `validate` under `app/generators/`; `product`, `architecture`, `story`, `registry`, `stepcoverage`, `stepskeleton`, `input`, `enginelog`, `buildinfo`, `cli/*` and the IO channels are under `infrastructure/`; `catalogue` and `provider` are domain models. `scenariogen` is `app/generators/scenariogen`; `ai router` and `ai guard` are `adapters/ai`. `steps` means `tests/true-bdd/steps/`, and `testkit` means `pkg/testkit/`. Story filenames resolve under `docs/product/stories/`. The retained meanings also feed `product.LoadGlossary`, structured link/word judgement and the concept context builders.

| Term | Words before | Words after | Code affected | Deleted and why |
| --- | ---: | ---: | --- | --- |
| Public interface | 207 | 27 | architecture; checks | public-interface-declared.yaml and true-bdd-cli.yaml fix document kinds, validation, startup and repair; retained the boundary meaning. |
| Concept | 70 | 39 | product; catalogue; validate | concepts.yaml, concept-link.yaml and concept-words.yaml fix shape, linking, closure and fixes; retained the closed meaning. |
| Concept binding | 113 | 25 | architecture; checks; validate | concepts.yaml and concept-link.yaml fix uniqueness, references and judgement; retained the connection and UI intent. |
| engine | 152 | 21 | bootstrap; infrastructure IO | services.patterns.owned_io fixes channel ownership, locks, writes and logs; kept program identity. |
| build identity | 115 | 16 | buildinfo; bootstrap | engine-entry-command-line.yaml and build-tests-files-suite-on-disk.yaml pin output/start logging; precedence stays in its mapping. |
| command | 1487 | 23 | commands; cmd | true-bdd-cli.yaml and the build-*, concept-*, init.yaml Stories fix execution. Deleted the embedded pipeline implementation manual. |
| model override | 117 | 0 | bootstrap | No Concept uses it; engine-model-override.yaml and CLI model options already fix overrides, validation and logging. Deleted. |
| project tree | 42 | 23 | steps; testkit | The binding fixes fixture assembly; deleted examples and rationale, kept the starting-tree meaning. |
| exit code | 87 | 30 | cmd; runner | true-bdd-cli.yaml responses and its mapping fix per-command statuses; deleted the repeated command list. |
| stdout | 29 | 12 | console; logging; steps | Kept stream meaning; output details already in true-bdd-cli.yaml and the mapping. |
| stderr | 47 | 22 | cmd; logging; steps | Kept stream meaning; deleted redundant examples, retaining the pre-logging distinction. |
| log record | 38 | 21 | enginelog; steps | Kept level/message/fields as output contracts; true-bdd-cli.yaml fixes named records. |
| pattern | 25 | 16 | steps | Kept RE2 matching meaning; dropped explanatory filler. |
| file change | 31 | 19 | testkit/fstree; steps | Kept pre/post-run created/modified/deleted meaning; deleted value commentary. |
| checklist | 179 | 23 | checklist; runner | us-refine-shipped-checklist.yaml, us-refine-quality-fix-prompts.yaml and concept-link.yaml already fix criteria counts and fixes. |
| Prompt | 39 | 22 | checklist; runner | Kept Q/D, rationale, docs and fix meaning; counting is fixed by engine log Scenarios. |
| AI turn | 28 | 21 | ai router; enginelog | Kept request and role meaning; cost commentary affects no generated code. |
| AI CLI | 65 | 20 | provider; ai router | dependencies[].invocation and engine-config-models-and-roots.yaml fix tiers, overrides, outputs and boundaries. |
| fix loop | 678 | 36 | runner; input; ai router | fix-loop-*.yaml, build-code-*.yaml and codex-apply-confinement.yaml already fix menus, EOF, retries and confinement; deleted their repeated manual. |
| failing set | 166 | 0 | commands; testrunner | No Concept uses it; build-code-failing-set.yaml, framework Stories and prompt_data already fix members, synthetic failures, reruns and evidence. Deleted. |
| test dependencies | 55 | 18 | testkit; steps | Kept required packages/tools; binding fixes prep and browser requirements. |
| glob | 28 | 21 | steps | Kept wildcard meaning; its mapping fixes syntax, component boundaries and count semantics. |
| Story | 39 | 21 | story; steps | Kept who/what/why and Feature relation; removed explanatory rationale. |
| Feature | 29 | 13 | story; steps | Kept Story label; deleted rationale already apparent from the data shape. |
| Acceptance Criterion | 45 | 24 | story; registry; steps | Kept authored id, steps and Scenario transition; schema/mapping already fix the exact representation. |
| step | 51 | 32 | registry; catalogue; steps | Kept Given/When/Then and concept/model-run meanings; mapping fixes serialized shape. |
| Scenario | 38 | 23 | registry; scenariogen | Kept behaviour and executable-test meaning; deleted persuasion about opinions. |
| Registry | 29 | 9 | registry; scenariogen | Kept single YAML collection; deleted rationale affecting no code. |
| lineage | 35 | 22 | registry; steps | Kept origin fields and id meaning; mapping fixes serialization and lookup. |
| Scenario key | 35 | 25 | registry; scenariogen | Kept lineage key and test-name relationship; removed repeated explanation. |
| step definition | 158 | 30 | stepcoverage; stepskeleton; scenariogen | concept-definitions.yaml and concept-steps.yaml fix records, skeletons, closure and writing; kept boundary and matching meaning. |
| fixture | 99 | 21 | commands; testkit | Kept named inputs and recordings distinction; prompt_data and concept-bodies.yaml already fix body/critique context. |
| Item | 28 | 0 | commands; runner | No Concept uses it; concept-grouping.yaml, concept-steps.yaml and build-steps-writing.yaml fix grouping, proposals and retries. Deleted. |
| Suite wiring | 77 | 0 | scenariogen; stepskeleton | No Concept uses it; build-steps-writing.yaml criteria 008/009, testing.testkit and Public interface fix deterministic wiring and marker rules. Deleted. |
| Scenario run | 47 | 0 | scenariogen; stepskeleton; testkit | No Concept uses it; build-steps-writing.yaml criterion 009 fixes embedded ScenarioRun and State preservation; library API remains supplied. Deleted. |
| Step coverage | 87 | 0 | stepcoverage; commands | No Concept uses it; build-steps-coverage.yaml, concept-grouping.yaml and concept-definitions.yaml fix scanning, ambiguity and gaps. Deleted. |
| red without the target | 96 | 0 | commands | No Concept uses it; build-steps-writing.yaml and build-steps-critique.yaml fix the absent-service run and executed-definition failures. Deleted. |
| test runner | 128 | 21 | testrunner; commands | build-code-failing-set.yaml and framework Stories fix selection/reruns; build-steps-writing.yaml fixes compilation/list/red checks. |
| shell command | 58 | 20 | steps; testkit | Kept separate after-run process; mapping/binding fix cwd, environment, timeout and status. |
| claude | 158 | 21 | cli/claude; ai router | dependencies[claude].invocation, build-code-claude-writer.yaml and engine-io.yaml fix guard, stream, environment and failure handling. |
| codex | 64 | 25 | cli/codex; ai router | Kept CLI identity and per-apply home lifecycle; dependencies[codex] and writer-lifecycle.yaml fix exact mechanics. |
| write guard | 88 | 21 | ai guard; cmd | claude-guard-*.yaml and claude-replay-preserved-invariants.yaml fix tools, roots and commands; kept allow/deny purpose. |
| write policy | 115 | 17 | ai guard; ai router | true-bdd-cli.yaml write-policy format and replay Stories fix fields, canonicalization and grants; kept policy meaning. |
| tool call | 51 | 25 | ai guard; steps | Kept name, inputs and Bash distinction; hook-payload mapping/binding fix exact fields. |
| hook payload | 34 | 16 | ai guard; steps | Kept stdin JSON shape; guard Stories/mapping already fix failure handling. |
| symbolic link | 31 | 20 | disk; ai guard; steps | Kept target/path meaning; mapping/bindings fix setup and safety checks. |
| Library | 168 | 27 | architecture; ai router | libraries.yaml and writer-config.yaml plus dependency invocation already fix downloading and confinement. Kept exact-pin meaning. |
| writable paths | 164 | 0 | architecture; commands; ai router | No Concept uses it; build-code-writable.yaml, codex-apply-confinement.yaml and dependencies fix grants/protected paths. Deleted. |
| impossible repair | 90 | 0 | commands; runner | No Concept uses it; build-code-writable.yaml and true-bdd-cli.yaml fix verdict, retained edits, rerun and stop. Deleted. |
| hard link | 38 | 23 | disk; steps | Kept shared inode and rename distinction; deleted redundant wording. |
| hold | 65 | 23 | disk; steps | Kept parent-directory contention and ten-second limit; patterns.owned_io and engine-io.yaml fix all mechanics. |
| staging file | 64 | 20 | disk; steps | Kept atomic replacement and stale-symlink meaning; patterns.owned_io and engine-io.yaml fix name/order/modes. |

## Word mapping audit

Each retained technical statement is the implementation choice for the code named in its row. Each removed `observed_by` duplicates the already existing Concept binding for that observation, or a cited Story/Scenario. Distinct path-confinement, pattern-escaping and static-inspection instructions remain. The schema and its init copy now make `observed_by` optional; the loader already accepts its absence.

| Term | technical words before → after | observed_by words before → after | Code affected; deleted and why |
| --- | ---: | ---: | --- |
| Concept | 118 → 62 | 56 → 0 | product; catalogue; validate. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. concepts.yaml, concept-link.yaml and concept-words.yaml fix shape, linking, closure and fixes; retained the closed meaning. |
| Concept binding | 129 → 61 | 152 → 0 | architecture; checks; validate. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. concepts.yaml and concept-link.yaml fix uniqueness, references and judgement; retained the connection and UI intent. |
| Public interface | 83 → 57 | 37 → 0 | architecture; checks. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. public-interface-declared.yaml and true-bdd-cli.yaml fix document kinds, validation, startup and repair; retained the boundary meaning. |
| build identity | 142 → 69 | 52 → 0 | buildinfo; bootstrap. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. engine-entry-command-line.yaml and build-tests-files-suite-on-disk.yaml pin output/start logging; precedence stays in its mapping. |
| Library | 260 → 78 | 55 → 0 | architecture; ai router. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. libraries.yaml and writer-config.yaml plus dependency invocation already fix downloading and confinement. Kept exact-pin meaning. |
| writable paths | 216 → 0 | 24 → 0 | architecture; commands; ai router. No Concept uses it; build-code-writable.yaml, codex-apply-confinement.yaml and dependencies fix grants/protected paths. Deleted. |
| impossible repair | 123 → 0 | 27 → 0 | commands; runner. No Concept uses it; build-code-writable.yaml and true-bdd-cli.yaml fix verdict, retained edits, rerun and stop. Deleted. |
| engine | 48 → 26 | 121 → 0 | bootstrap; infrastructure IO. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. services.patterns.owned_io fixes channel ownership, locks, writes and logs; kept program identity. |
| Item | 350 → 0 | 66 → 0 | commands; runner. No Concept uses it; concept-grouping.yaml, concept-steps.yaml and build-steps-writing.yaml fix grouping, proposals and retries. Deleted. |
| red without the target | 96 → 0 | 60 → 0 | commands. No Concept uses it; build-steps-writing.yaml and build-steps-critique.yaml fix the absent-service run and executed-definition failures. Deleted. |
| Suite wiring | 220 → 0 | 118 → 0 | scenariogen; stepskeleton. No Concept uses it; build-steps-writing.yaml criteria 008/009, testing.testkit and Public interface fix deterministic wiring and marker rules. Deleted. |
| Scenario run | 30 → 0 | 38 → 0 | scenariogen; stepskeleton; testkit. No Concept uses it; build-steps-writing.yaml criterion 009 fixes embedded ScenarioRun and State preservation; library API remains supplied. Deleted. |
| Step coverage | 92 → 0 | 13 → 0 | stepcoverage; commands. No Concept uses it; build-steps-coverage.yaml, concept-grouping.yaml and concept-definitions.yaml fix scanning, ambiguity and gaps. Deleted. |
| command | 421 → 63 | 186 → 0 | commands; cmd. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. true-bdd-cli.yaml and the build-*, concept-*, init.yaml Stories fix execution. Deleted the embedded pipeline implementation manual. |
| project tree | 127 → 39 | 319 → 49 | steps; testkit. Kept distinct observation instructions; compacted technical mechanics. Removed repeated examples and lifecycle prose already fixed by the bindings and cited contracts. The binding fixes fixture assembly; deleted examples and rationale, kept the starting-tree meaning. |
| exit code | 69 → 38 | 25 → 0 | cmd; runner. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. true-bdd-cli.yaml responses and its mapping fix per-command statuses; deleted the repeated command list. |
| stdout | 83 → 27 | 32 → 0 | console; logging; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept stream meaning; output details already in true-bdd-cli.yaml and the mapping. |
| stderr | 62 → 20 | 29 → 0 | cmd; logging; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept stream meaning; deleted redundant examples, retaining the pre-logging distinction. |
| log record | 279 → 89 | 31 → 0 | enginelog; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept level/message/fields as output contracts; true-bdd-cli.yaml fixes named records. |
| pattern | 20 → 18 | 100 → 43 | steps. Kept distinct observation instructions; compacted technical mechanics. Removed repeated examples and lifecycle prose already fixed by the bindings and cited contracts. Kept RE2 matching meaning; dropped explanatory filler. |
| file change | 53 → 31 | 46 → 0 | testkit/fstree; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept pre/post-run created/modified/deleted meaning; deleted value commentary. |
| checklist | 108 → 54 | 56 → 0 | checklist; runner. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. us-refine-shipped-checklist.yaml, us-refine-quality-fix-prompts.yaml and concept-link.yaml already fix criteria counts and fixes. |
| Prompt | 26 → 21 | 17 → 0 | checklist; runner. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept Q/D, rationale, docs and fix meaning; counting is fixed by engine log Scenarios. |
| AI turn | 30 → 22 | 25 → 0 | ai router; enginelog. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept request and role meaning; cost commentary affects no generated code. |
| model override | 146 → 0 | 41 → 0 | bootstrap. No Concept uses it; engine-model-override.yaml and CLI model options already fix overrides, validation and logging. Deleted. |
| AI CLI | 63 → 32 | 62 → 0 | provider; ai router. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. dependencies[].invocation and engine-config-models-and-roots.yaml fix tiers, overrides, outputs and boundaries. |
| failing set | 106 → 0 | 27 → 0 | commands; testrunner. No Concept uses it; build-code-failing-set.yaml, framework Stories and prompt_data already fix members, synthetic failures, reruns and evidence. Deleted. |
| fix loop | 543 → 109 | 89 → 0 | runner; input; ai router. Kept distinct observation instructions; compacted technical mechanics. Removed repeated examples and lifecycle prose already fixed by the bindings and cited contracts. fix-loop-*.yaml, build-code-*.yaml and codex-apply-confinement.yaml already fix menus, EOF, retries and confinement; deleted their repeated manual. |
| test dependencies | 45 → 26 | 37 → 0 | testkit; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept required packages/tools; binding fixes prep and browser requirements. |
| fixture | 69 → 34 | 34 → 0 | commands; testkit. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept named inputs and recordings distinction; prompt_data and concept-bodies.yaml already fix body/critique context. |
| glob | 29 → 57 | 178 → 0 | steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept wildcard meaning; its mapping fixes syntax, component boundaries and count semantics. |
| Story | 21 → 30 | 53 → 0 | story; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept who/what/why and Feature relation; removed explanatory rationale. |
| Feature | 9 → 9 | 11 → 0 | story; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept Story label; deleted rationale already apparent from the data shape. |
| Acceptance Criterion | 34 → 30 | 31 → 0 | story; registry; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept authored id, steps and Scenario transition; schema/mapping already fix the exact representation. |
| step | 49 → 38 | 60 → 0 | registry; catalogue; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept Given/When/Then and concept/model-run meanings; mapping fixes serialized shape. |
| Scenario | 35 → 24 | 35 → 0 | registry; scenariogen. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept behaviour and executable-test meaning; deleted persuasion about opinions. |
| Registry | 23 → 22 | 100 → 0 | registry; scenariogen. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept single YAML collection; deleted rationale affecting no code. |
| lineage | 38 → 31 | 63 → 0 | registry; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept origin fields and id meaning; mapping fixes serialization and lookup. |
| Scenario key | 37 → 24 | 21 → 0 | registry; scenariogen. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept lineage key and test-name relationship; removed repeated explanation. |
| step definition | 177 → 69 | 115 → 25 | stepcoverage; stepskeleton; scenariogen. Kept distinct observation instructions; compacted technical mechanics. Removed repeated examples and lifecycle prose already fixed by the bindings and cited contracts. concept-definitions.yaml and concept-steps.yaml fix records, skeletons, closure and writing; kept boundary and matching meaning. |
| test runner | 283 → 91 | 51 → 0 | testrunner; commands. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. build-code-failing-set.yaml and framework Stories fix selection/reruns; build-steps-writing.yaml fixes compilation/list/red checks. |
| shell command | 40 → 44 | 132 → 0 | steps; testkit. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept separate after-run process; mapping/binding fix cwd, environment, timeout and status. |
| claude | 161 → 35 | 50 → 0 | cli/claude; ai router. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. dependencies[claude].invocation, build-code-claude-writer.yaml and engine-io.yaml fix guard, stream, environment and failure handling. |
| codex | 55 → 23 | 49 → 0 | cli/codex; ai router. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept CLI identity and per-apply home lifecycle; dependencies[codex] and writer-lifecycle.yaml fix exact mechanics. |
| write guard | 93 → 69 | 47 → 0 | ai guard; cmd. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. claude-guard-*.yaml and claude-replay-preserved-invariants.yaml fix tools, roots and commands; kept allow/deny purpose. |
| write policy | 91 → 62 | 43 → 0 | ai guard; ai router. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. true-bdd-cli.yaml write-policy format and replay Stories fix fields, canonicalization and grants; kept policy meaning. |
| tool call | 38 → 25 | 37 → 0 | ai guard; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept name, inputs and Bash distinction; hook-payload mapping/binding fix exact fields. |
| hook payload | 32 → 20 | 24 → 0 | ai guard; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept stdin JSON shape; guard Stories/mapping already fix failure handling. |
| symbolic link | 19 → 34 | 73 → 0 | disk; ai guard; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept target/path meaning; mapping/bindings fix setup and safety checks. |
| hard link | 36 → 19 | 33 → 0 | disk; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept shared inode and rename distinction; deleted redundant wording. |
| hold | 60 → 18 | 68 → 0 | disk; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept parent-directory contention and ten-second limit; patterns.owned_io and engine-io.yaml fix all mechanics. |
| staging file | 38 → 19 | 46 → 0 | disk; steps. Removed observed_by: its observation is already fixed by the corresponding binding/Story. Kept technical mechanics; removed repeated procedures from the cited contract. Kept atomic replacement and stale-symlink meaning; patterns.owned_io and engine-io.yaml fix name/order/modes. |

## Concept binding audit

All 19 bindings retain their concept, block, service and Public interface references unchanged. Their `does` text retains every admitted setup/call/observation variation, including the controlled-fixture contracts needed by rebuilt definitions.

| Binding | does words before | does words after | Code affected | Deleted and why |
| --- | ---: | ---: | --- | --- |
| given: project tree | 397 | 101 | steps/cli_steps.go; testkit | Deleted repeated lifecycle narrative and shell-command examples; kept overlay/modes, Given variants and atomic fixture contract. |
| given: machine environment | 134 | 46 | steps/library_steps.go; steps/io_engine_run.go | Removed explanation of fixture assembly already fixed by project tree; kept prep checks, proxy and Git/Go variables. |
| given: fixture AI CLIs | 271 | 98 | steps/fixture_cli.go; steps/io_claude_steps.go; steps/concurrent_validation_steps.go | Deleted repeated mode/proxy explanation; kept every controlled fixture allowlist, PATH order, poisoned variable and admission-order rule. |
| given: scripted answers | 48 | 29 | steps/answer_steps.go | Deleted repetition; kept 64 answers, slash splitting, trimming and EOF. |
| given: write policy | 140 | 56 | steps/claude_guard_steps.go; steps/guard_call.go | Deleted per-field narrative repetition; kept exact replay payload, path handling and missing/invalid cases. |
| given: tool call | 168 | 56 | steps/guard_call.go | Deleted repeated JSON prose; kept payload shape, precedence, defaults and path/cwd resolution. |
| when: a role runs a command | 53 | 41 | steps/command_steps.go; testkit | Deleted restated explanation of role; kept one process, queued stdin, CWD, evidence and recorder handoff. |
| when: a command runs under interference | 298 | 117 | steps/io_hold_steps.go; steps/io_atomic_steps.go; steps/io_cancel_steps.go; steps/grouping_binding.go | Deleted repeated normal-run lifecycle; kept launch ordering, all timing thresholds, probes, signals and binding-change wrapper. |
| when: the write guard is asked | 57 | 31 | steps/claude_guard_steps.go | Deleted repeated assembly explanation; kept pre-snapshot policy, argv, stdin and retained result. |
| then: exit code | 14 | 11 | steps/cli_steps.go | Compacted comparison; kept numeric/no semantics. |
| then: console output | 43 | 23 | steps/cli_steps.go | Deleted matching prose repeated by pattern mapping; kept named stream and positive/negative assertion. |
| then: file changes | 165 | 51 | steps/file_steps.go; testkit/fstree | Deleted examples and wildcard tutorial already in glob mapping; kept all variations and unchanged-file existence. |
| then: file content | 175 | 76 | steps/file_match_steps.go; steps/file_pattern.go; steps/io_atomic_steps.go | Deleted escaping details already in pattern mapping; kept all assertions, nonempty glob, counts and inode/mode checks. |
| then: Story content | 104 | 53 | steps/story_steps.go | Deleted repeated read lifecycle; kept unique-file/criterion requirements and every comparison. |
| then: Registry content | 131 | 60 | steps/registry_steps.go; steps/registry_origin_steps.go | Deleted repeated YAML-read prose; kept origin uniqueness, whole-entry equality and static registration coverage. |
| then: engine activity | 219 | 84 | steps/engine_log_steps.go; steps/build_identity_steps.go; steps/validation_observation.go | Deleted per-record narrative already fixed by interface/mappings; kept every filter, missing-log refusal and overlap/order invariant. |
| then: after-run shell check | 38 | 25 | steps/command_steps.go | Compacted unchanged cwd/environment/status contract. |
| then: write guard's answer | 33 | 20 | steps/claude_guard_steps.go | Compacted allow/deny status and reason matching. |
| then: AI CLI observations | 198 | 66 | steps/writer_lifecycle_steps.go; steps/io_cancel_steps.go; steps/concurrent_validation_steps.go | Deleted repeated mode explanation; kept disappearance window, exact receipt, one-home cleanup and replay-only reverse order. |

## Other pieces and retained large contracts

- Product title and empty vision/goals/releases/not_goals stay because the product schema requires their declared shape. Summary/problem lose persuasion and duplicated procedural contracts; purpose remains, and summary specifies the new counting/refusal rule. Role names stay unchanged for Story role judgements, with responsibilities/use/characteristics compacted. Vocabulary values stay unchanged because quality checklist questions consume them. All Concept prose stays unchanged: grouping must see every admitted variation (#281).
- Architecture testing keeps its framework, exact library pins, testkit choices and all three commands: these drive `architecture.Load`, `scenariogen` and `testrunner`. Deleted comments narrating the same commands and an obsolete cold-cache module pin; they affect no rebuilt engine code.
- Services keep source path, language, interface declaration, exact full library graph, frameworks and IO/resource rules. These are reconstruction inputs for manifests, init, channel ownership, directory locking, atomic replacement, process groups and stream handling. The complete graph is deliberately large: cut A deletes the engine manifests, so a rebuild needs the exact pins.
- Dependencies keep both exact CLI invocation contracts, configuration wire forms, sandbox/environment grants, result handling and cancellation. These remain large because replay hashes argv/stdin, and live-only config shape has independent Scenario assertions. Deleted only their descriptions of how writer-config/writer-lifecycle tests check those contracts; the existing Scenarios and test library already do that. Deleted the project_resources rule describing the init-defaults drift gate: that gate is kept tooling in cut A, not engine behaviour. The preceding rules still specify init and runtime resource handling. Kept the exact buildinfo.version and buildinfo.commit linker symbols because the retained release tooling stamps them by name.
- Deleted the architecture header’s release/drill/gate/test-bench narration. It affects kept `scripts/` tooling, which cut A does not rebuild, and duplicates README, ADRs and the drill ticket. Retained the three-module ownership and cut-A boundary in compact comments. Nothing was moved.
- `prompt_data` is **out of scope by the Owner’s decision**, including every field and rendering entry; all 32 entries remain unchanged. `docs/architecture/true-bdd-cli.yaml` is wholly unchanged, as are templates and context builders. No Specification key deletion forced a prompt change.

## Refusal and fixtures

`product.ValidateForm`, `product.Load` and `product.LoadGlossary` all reject invalid glossary prose through the shared document reader. Legacy unreadable/non-list glossary findings retain their previous command paths; a malformed sibling cannot conceal another entry’s oversized meaning. The error identifies the term, sentence count, longest sentence’s word count, paragraph count and the rule: one paragraph, at most three sentences, each at most 25 words. Empty meanings and multiple paragraphs refuse too. This applies at configured-command startup through `bootstrap.NewContainer`; configuration-free init/version and the hidden guard retain their existing paths.

A sentence ends at a run of `.`, `!` or `?` followed by Unicode whitespace or EOF; a trailing nonempty fragment counts as a sentence. Words are nonempty Unicode-whitespace-separated fields, so punctuation attached to a word, paths, decimals and hyphenated words do not split it. Blank lines delimit paragraphs; ordinary wrapped lines do not. The product summary is the Specification of this algorithm; README and engine rules agree. Concepts’ meanings are exempt.

RED: `TestGlossaryRefusesFourSentencesAtEveryReader` initially failed at all three readers. GREEN: shared reader validation passed it. Boundary tests pin 25/26 words, 3/4 sentences, trailing fragments, punctuation runs, decimals/paths, Unicode whitespace, wrapped lines, LF/CRLF paragraphs and empty prose; the empty-prose test was also observed red before adding that refusal.

Registry Scenario `glossary-form-001-startup-refused` names fixture `glossary-form-refused`. Its meaning deliberately has four sentences with a 26-word first sentence; it observes startup refusal, exact counts/rule, exit 1, zero AI turns and no changes outside tmp. It has no Story claim, as standalone Scenarios permit. The fixture inherits the shipped checklist, with no custom override or CLI stand-in. Its recording contains only the generated empty golden and was produced through the declared record command narrowed by `-run '^TestGlossaryForm001StartupRefused$'`; cost $0. No cassette was hand-edited.

Census: 131 parseable fixture product documents with glossary lists, 143 entries including the deliberate refusal fixture. Pre-existing input meanings already satisfy the bounds. Replay exposed oversized meanings in the recorded fix outputs of **fix-loop-refine-feedback** (28 words) and **fix-loop-walks-exhausted-terms** (26 words). Their product summaries now state the hard glossary rule, and both fixtures were re-recorded through the declared command; no recording was hand-edited. The init-empty golden was also re-recorded because init ships the updated architecture schema. A final audit of 159 fixture and recorded-output meanings finds only the new deliberate refusal entry outside the limits. Existing malformed-product/non-list fixtures remain intentional refusal inputs.

## Divergence review

Checked the four issue-tracker categories against the product loaders, architecture loaders/closure checks, `scen-check`’s five D Prompts, every edited mapping/binding, corresponding step definitions and the cited Stories/Registry. All 43 Concept terms remain defined and mapped exactly once; all 19 meanings and binding references remain unchanged. New behaviour is described in the product summary and pinned by the new Registry Scenario and reader-boundary tests. Removed text is redundant with retained Specification/checklist/interface contracts or describes kept tooling/test-library internals; no required engine behaviour was intentionally removed. The optional observation field closes the schema/loader mismatch exposed by removing redundant observations. The old binding mapping also claimed every concept required a binding, contradicting concept-link.yaml criterion 018 and checks.conceptBindingFindings; it now describes the actual linked-only requirement. The old claude mapping incorrectly claimed build-steps requested the restricted Claude writer; the invocation contract and current router limit it to build-code, and the compacted mapping now agrees.

The review is a document/code audit plus existing replay, not a fresh cut-A regeneration. Running the live rebuild drill is outside #287. No unresolved Specification divergence was found in the edited scope. The untouched prompt_data rendering anchor still contains its older build-steps binding-turn description; concept-grouping.yaml and the retained ADR-0037 contracts describe the current shared judgement. This pre-existing clause is left for the Owner’s separate prompt_data design; no open question blocks this change.

## Verification

Commands ran in the foreground from this worktree, with full logs in ignored `tmp/spec-diet/`:

- `go run ./scripts/cmd/alint <every changed file>`: passed for all 42 changed files.
- `GOWORK=off go build ./...` and `GOWORK=off go test ./...` in `.`, `pkg/` and `services/true-bdd/`: passed in all three modules, independently and through the gates.
- `GOWORK=off go -C services/true-bdd build -o ../../bin/true-bdd .`: passed.
- Declared record command, narrowed to the new Scenario: passed, zero AI turns, empty golden.
- `GOWORK=off go test -tags bdd ./tests/true-bdd/ -mode=services:replay,tests:replay`: passed, all 349 Scenarios (413.912 seconds under concurrent verification load).
- `bin/true-bdd scen check`, narrowed to the five shipped D Prompts with a temporary checklist copy restored byte-for-byte after the run: passed, 349 Scenarios × five checks, zero AI turns.
- `go run ./scripts/cmd/gates run --changed main`: passed by hand before `git commit --no-verify`.

During verification, one root immutability check observed this run staging files concurrently; the unchanged checkout/index rerun passed. One shared-library graceful-stop timing test failed under concurrent load and passed on rerun. No implementation workaround or gate configuration change was made.
