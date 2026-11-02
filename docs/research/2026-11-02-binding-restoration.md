# Binding restoration, 2 November 2026

Issue #290, under #249. Branch `impl/290-bindings`, starting at `be013647bacb36ee118079935ac56c11b6f678a6`. Only the five requested `architecture.concepts[].does` values change. References, all fourteen other bindings, Concept meanings, glossary, Registry and engine code remain unchanged.

## Words and isolated verdicts

Words are whitespace-separated words in the YAML-decoded `does` scalar, as in #287's audit. Before is `d6d0243c`; after is this run's starting commit, after #287. Costs are native `total_cost_usd`, including cache charges and every preliminary call.

| Binding | Before #287 | After #287 | Now | Final isolated verdict | Final call USD | Preliminary USD | Total USD |
|---|---:|---:|---:|---|---:|---:|---:|
| `given:fixture AI CLIs` | 271 | 98 | 172 | unambiguous | 0.2989165 | 0.3133365 | 0.6122530 |
| `given:machine environment` | 134 | 46 | 102 | unambiguous | 0.2802460 | 0.2830725 | 0.5633185 |
| `given:project tree` | 397 | 101 | 170 | unambiguous | 0.4748595 | 0.5322410 | 1.0071005 |
| `then:AI CLI observations` | 198 | 66 | 117 | unambiguous | 0.2496380 | 0.2508775 | 0.5005155 |
| `when:a command runs under interference` | 298 | 117 | 215 | unambiguous | 0.3283910 | 0.4169105 | 0.7453015 |

### given:fixture AI CLIs

The round's judgement asked what supplies the proxies and modes, how fixture paths resolve, which invalid reply each step selects, which environment values to set, and who coordinates concurrent arrival. Restored the suite's mode source and proxy behaviour, absolute fixture/run roots, ordered PATH insertion per variation, every poisoned environment value, and the fixture-consumed control variables. Invalid link/grouping Givens deliberately share setup: the selected fixture supplies the reply; the closed allowlist and absent-program failure remain explicit. Existing front launchers own request ordering. These instructions affect `fixture_cli.go`, `io_claude_steps.go`, `writer_lifecycle_steps.go` and `concurrent_validation_steps.go`, without asking rebuilt definitions to reproduce the library's proxy machinery.

### given:machine environment

The judgement asked for GOPROXY's variable, URL and run-root location, fixture lookup and preparation timing, exact Git paths, and TRUE_BDD_REAL_GO's consumer and resolution. Restored each value, next-command environment scope, prep.sh verification with the literal browser marker, and delegation from fixture Go programs after checking Git variables. This matches `library_steps.go`, `io_hold_steps.go` and the dependency Givens in `cli_steps.go`. The empty proxy is a local file source, with no network fallback; an absent source and an empty directory both refuse downloads.

### given:project tree

The judgement asked who actually narrows a checklist, the snippet matching rule and file anchor, and whether symbolic-link confinement includes its target. Restored fixture selection retained for later Givens, fixture-root checklist-prompts.yaml, exact equality of the one selected snippet, and the harness's subsequent substring filtering during assembly. Confine the link path while preserving its literal target, including escaping targets; name the snapshot-excluded directories. Retained optional preparation, atomic probes, modes and teardown. These are `prepareTree`, `assertChecklistNarrowed`, `plantLink`, omission and atomic-probe setup; no new checklist mutation occurs in a Given.

### then:AI CLI observations

The judgement asked for the replay receipt path and data shape, the mode discriminator, exact writer receipt bytes and the basis for the private-authentication claim. Restored suite -mode as the source, run-root paths, consumed recording-name array with first name claude-002, and exact newline-terminated writer receipt. State what the existing observing proxy verifies before issuing that receipt, including private file/home permissions and protected roots; the Then reads the receipt and checks authentication removal. This matches `concurrent_validation_steps.go`, `writer_lifecycle_steps.go`, `io_cancel_steps.go` and the existing `pkg/testkit/aiproxy/writer_spawn.go` verifier. Real credentials are never inspected.

### when:a command runs under interference

The judgement asked for TRUE_BDD_GROUPING_ENGINE's value and consumer, wrapper arguments and replacement paths, SIGINT's recipient, the directory held during init, and the kill target on failed observations. Restored the absolute engine executable and fixture-wrapper contract, scen check's zero-exit prerequisite, input-overlay source and architecture destination; signal only the engine PID. The base overlay already supplies init's true-bdd directory; its Given removes only the configuration file. Name SIGKILL's target, retain all hold timings and barriers, and distinguish the binding-change wrapper from the IO launcher. This matches `grouping_binding.go`, its fixture's `input/run-after-check`, and `io_{hold,atomic,cancel,engine_run}*.go`.

## Other fourteen

Each received a fresh isolated judgement, over all distinct steps from the round's passing links, with its wording unchanged.

| Binding | Distinct steps | System plus user bytes | Verdict | USD |
|---|---:|---:|---|---:|
| `given:scripted answers` | 10 | 22,380 | unambiguous | 0.2639935 |
| `given:tool call` | 27 | 24,643 | unambiguous | 0.2825965 |
| `given:write policy` | 6 | 21,616 | unambiguous | 0.2762380 |
| `then:Registry content` | 21 | 25,049 | unambiguous | 0.3790630 |
| `then:Story content` | 34 | 26,993 | unambiguous | 0.3973980 |
| `then:after-run shell check` | 10 | 21,930 | unambiguous | 0.2400810 |
| `then:console output` | 501 | 93,269 | unambiguous | 0.6288410 |
| `then:engine activity` | 51 | 29,337 | unambiguous | 0.3830280 |
| `then:exit code` | 2 | 21,523 | unambiguous | 0.2777405 |
| `then:file changes` | 96 | 33,470 | unambiguous | 0.3206945 |
| `then:file content` | 187 | 66,035 | unambiguous | 0.6116230 |
| `then:write guard's answer` | 11 | 21,528 | unambiguous | 0.2353135 |
| `when:a role runs a command` | 68 | 29,859 | unambiguous | 0.3140835 |
| `when:the write guard is asked` | 1 | 20,532 | unambiguous | 0.2627895 |

All nineteen final judgements pass. **Total new model spend: $8.3019725 of $25**, including **$1.7964380** for five preliminary calls. The five restored scalars total 1298 → 428 → 776 words; each remains shorter than before #287.

## Invocation and evidence

Read #290, #287 and #249; the supplied round log and saved prompts/answers; #287's audit and #285's isolated-call report; the domain instructions, glossary, chronological Q&A decisions, ADR-0037 and its transport amendment ADR-0039; the shipped bindings Prompt, evaluator and CLI wrapper; and the affected definitions and fixture programs. The later Owner decisions reject compaction and digests and keep the complete product document readable by path. No evidence was moved elsewhere.

Raw data is ignored under `tmp/binding-restoration/`: `judge.py`, `restore.py`, `ledger.jsonl`, per-call system/user/argv/stream/result/answer files, count snapshots and verification logs. The original round files were read only. Each call runs synchronously in an isolated Host project inside this worktree containing the complete current product document at its original product_path. The concept, binding, complete Public interface and every supplied step stay embedded. The model read the product document through Read; no service source was supplied.

`judge.py` starts from each saved ADR-0037 per-binding request, replaces only `binding.Does`, and preserves Go field names, empty fields, reference shapes, indentation and HTML escaping. Its serializer reproduces all nineteen original payloads byte-for-byte before substitution. The system text is the saved shipped Prompt verbatim. The response schema is `bindingSchema`'s required object with that binding's exact key, boolean unambiguous and nonempty reason. Each final schema-valid response says true. System plus user bytes are checked against 150,000 before dispatch.

Flags match the recorded validation calls: `claude --setting-sources '' --strict-mcp-config --safe-mode --allowedTools 'Read(**),Write(./tmp/**),Glob(**),Grep(**)' --disallowed-tools 'Bash,Edit(**),MultiEdit(**),Agent,Task' --model claude-opus-5 --system-prompt <saved-system> --permission-mode acceptEdits --output-format stream-json --verbose --json-schema <per-binding-schema> -p`, with the user prompt on stdin. Remove CLAUDECODE; set CLAUDE_HISTORY_ROLE=prompt and CLAUDE_CODE_DISABLE_ADVISOR_TOOL=1. No effort downgrade, compaction, filtering, prompt truncation or reused verdict. Every command runs in the foreground and finishes before the report.

The five preliminary calls passed but serialized the binding with YAML field names rather than Go's JSON shape; they do not establish acceptance. Corrected the scratch driver and repeated all nineteen with the engine shape. Before the final calls, also clarified optional prep.sh and ordinary live mode, and simplified one mode-selection sentence. No ambiguous final verdict required another wording iteration.

## Specification divergence review

Checked the four categories in `docs/agents/issue-tracker.md` against the unchanged product Concepts and glossary, the Registry's steps, declared Public interface, current definitions, fixture scripts and test-library verifier. Every restored instruction affects selection, environment, launch, observation or comparison in the named definitions. Paths, values and process targets agree with the existing code; fixture programs and the library own the mechanisms the definitions only configure. No new engine option, Public interface reference or admitted variation is introduced. No contradiction, undeclared concept term, unrepresented behaviour or claim beyond the code was found in the changed scope. The removed pre-#287 explanatory prose stays deleted.

## Verification

- `go run ./scripts/cmd/alint docs/architecture/architecture.yaml docs/research/2026-11-02-binding-restoration.md`: passed.
- `go run ./scripts/cmd/gates build`: passed; it runs `GOWORK=off go build ./...` independently in `.`, `pkg/` and `services/true-bdd/`.
- `GOWORK=off go test ./...` in each of `.`, `pkg/` and `services/true-bdd/`: passed, with separate logs.
- `GOWORK=off go -C services/true-bdd build -o ../../bin/true-bdd .`: passed.
- `bin/true-bdd scen check` in the scratch verification copy, with only the five shipped D Prompts retained: passed, all 351 Scenarios × five built-in checks, zero AI turns. The copied architecture is byte-identical to the final file; tracked checklist and configuration files are untouched.
- `GOWORK=off go test -tags bdd ./tests/true-bdd/ -mode=services:replay,tests:replay`: passed (328.017 seconds).
- `go run ./scripts/cmd/gates run --changed main`: passed by hand before `git commit --no-verify`.
- `git diff --check`: passed.

Decisions: retain compact explicit instructions only where the failed judgements and existing definitions need them; preserve all other binding wording and all references. Open questions: none for #290. This verifies the binding stage and the existing offline suite, without asserting a completed cut-B rebuild for #249.
