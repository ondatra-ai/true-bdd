# Issue 203 Scenario dispositions

Baseline Registry: 56 affected Scenarios: 35 `crush-*`, 21 other mentions. Table is the pre-edit decision record. Every original Then clause is listed as an invariant, alongside the description; provider-specific transports and diagnostics retire only where stated. New equivalents exercise the Claude hook, not a fictional codex guard. #205 owns confinement of other codex apply turns.

| Original Scenario | Every invariant (description and Then clauses) | Disposition / afterwards |
| --- | --- | --- |
| `engine-004-unknown-model-cli-refused` | A command must be refused at startup when an &#96;engine.models&#96; tier names an AI CLI the engine does not drive, naming the tier, that CLI and the CLIs it supports on stdout, rather than run the tier on another model; the command exits with code 1; stdout matches level=ERROR msg="Refusing to start" command="build tests"; stdout matches engine\.models\.coder: model reference names an unknown cli: \\"aider\\" \(supported: claude, crush, codex\); stdout does not match BUILD TESTS; no file outside "tmp" changed; the file "docs/scenarios.yaml" is unchanged; the engine dispatched no AI turns | Retain all engine invariants and mechanical assertions in &#96;engine-004-unknown-model-cli-refused&#96;; writer/apply CLI and malformed-model examples become codex. The unknown CLI input becomes crush; same startup refusal and no dispatch. |
| `scen-check-fix-001-unmatched-step-gets-phrase` | &#96;scen check --fix&#96; must propose a catalogue entry for a step that matches none, record the entry the user applies in the product document and never in the Scenario, and pass the re-walk; the command exits with code 0; stdout matches CHECK FAILED; stdout matches E2E-001: then step "the service answers within 200 milliseconds" matches no template; stdout matches msg="AI needs clarification"; stdout matches FIX PROMPT GENERATED; stdout matches msg="Fix recorded in the document" document=product; stdout matches check=step-phrases subjectID=E2E-001 promptIndex=1 answer=pass; stdout matches ALL CHECKS PASSED; the only file changed outside "tmp" is "docs/product/product.yaml"; the file "docs/product/product.yaml" matches the service answers within; the engine dispatched 3 AI turns; every file matching "tmp/&#42;/crush-config-&#42;/crush.json" does not match "bash"; every file matching "tmp/&#42;/crush-config-&#42;/crush.json" does not match "job_output" | Retain all engine invariants and mechanical assertions in &#96;scen-check-fix-001-unmatched-step-gets-phrase&#96;; writer/apply CLI and malformed-model examples become codex. Crush configuration inventory assertions retire; writer-only replay form isolation is pinned by writer_template and apply_confirmation unit seams. |
| `engine-routes-002-crush-asks-before-fixing` | &#96;scen check --fix&#96; with every tier naming crush, a CLI that enforces no answer schema, must still read crush's verdict and its clarifying question, ask that question, record in the Registry the fix crush then writes, and pass the re-walk; the command exits with code 0; stdout matches CHECK FAILED; stdout matches msg="AI needs clarification" questionCount=1; stdout matches FIX PROMPT GENERATED; stdout matches msg="Fix recorded in the document" document=scenarios_yaml; stdout matches ALL CHECKS PASSED; the engine dispatched 3 validation AI turns to "crush"; the engine dispatched 2 fix AI turns to "crush"; the engine dispatched 1 apply AI turn to "crush"; the engine dispatched 6 AI turns; the only file changed outside "tmp" is "docs/scenarios.yaml" | Retire schema-less crush routing and six-turn shape. Both supported providers enforce result schemas. Clarification, document commit and re-walk remain pinned by engine-routes-003-prompt-tiers-win, scen-check-fix-001 and fix-loop-dialogue-001 (fixture fix-loop-own-words). |
| `engine-routes-003-prompt-tiers-win` | A Prompt naming its own &#96;model:&#96;, &#96;fix_model:&#96; and &#96;apply_model:&#96; tiers must send its validation, fix and apply turns to the AI CLIs those tiers name, and none to the AI CLI its checklist's tier names for that kind of turn; the command exits with code 0; stdout matches ALL CHECKS PASSED; the engine dispatched 3 validation AI turns to "crush"; the engine dispatched 1 fix AI turn to "codex"; the engine dispatched 1 apply AI turn to "claude"; the engine dispatched no validation AI turns to "claude"; the engine dispatched no fix AI turns to "claude"; the engine dispatched no apply AI turns to "crush"; the engine dispatched 5 AI turns; the only file changed outside "tmp" is "docs/scenarios.yaml" | Retain all engine invariants and mechanical assertions in &#96;engine-routes-003-prompt-tiers-win&#96;; writer/apply CLI and malformed-model examples become codex. Validation and fix both use codex; checklist uses claude for those roles and codex for apply, so each role still proves precedence and the negative dispatch assertions. |
| `engine-routes-005-failed-crush-turn-stops-run` | A validation turn whose crush CLI exits non-zero, here on a model id the CLI does not know, must stop &#96;scen check&#96; with exit code 1 and a log record naming the turn, its AI CLI and its model, before any verdict is reported; the command exits with code 1; stdout matches msg="AI turn failed" turn=1 label=crush-turn001 role=prompt cli=crush model=zhipu-coding/no-such-model; stdout does not match CHECK FAILED&#124;ALL CHECKS PASSED; the engine dispatched 1 AI turn; no file outside "tmp" changed | Retain all engine invariants and mechanical assertions in &#96;engine-routes-005-failed-codex-turn-stops-run&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `crush-guard-001-read-only-tool-allowed` | A tool call to a tool that only reads must be allowed for any file inside the project, inside the turn's write roots or not; the guard allows the tool call | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-001-read-only-tool-allowed&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-002-shell-denied-without-grant` | A tool call to the shell must be denied when the turn's write policy does not let the turn run it, since a shell command can write anywhere; the guard denies the tool call; the guard's reason matches shell is denied in this mode | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-002-shell-denied-without-grant&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-003-shell-allowed-with-grant` | A tool call to the shell must be allowed when the turn's write policy lets the turn run it; the guard allows the tool call | Retire unrestricted-shell grant: no surviving writer grants an unrestricted shell; exact replay only, pinned by build-code-claude-001/002/003/004. |
| `crush-guard-004-unknown-tool-denied` | A tool call to a tool the guard does not know must be denied as a writer, even when it targets a file inside a write root; the guard denies the tool call; the guard's reason matches tool rename is not on the allowlist \(unknown tools are treated as writers\) | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-004-unknown-tool-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-005-missing-policy-denied` | A tool call must be denied when crush hands the guard no write policy, even one to a tool that only reads; the guard denies the tool call; the guard's reason matches crush guard policy is not set | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-005-missing-policy-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-006-unreadable-policy-denied` | A tool call must be denied when the guard cannot read the write policy crush hands it, even one to a tool that only reads; the guard denies the tool call; the guard's reason matches crush guard policy is malformed | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-006-unreadable-policy-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-007-payload-names-the-call` | When crush names neither the tool nor its file but in the hook payload, the guard must judge the tool call the payload names; the guard allows the tool call | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-007-payload-names-the-call&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-writes-001-write-inside-root-allowed` | A write to a file inside one of the turn's write roots must be allowed, even when the file does not exist yet; the guard allows the tool call | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-writes-001-write-inside-root-allowed&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-writes-002-write-outside-roots-denied` | A write to a file outside every one of the turn's write roots must be denied, naming the file and the roots; the guard denies the tool call; the guard's reason matches write target \S&#42;/docs/notes\.md is outside \S&#42;/tests, \S&#42;/services | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-writes-002-write-outside-roots-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-writes-003-symlink-escape-denied` | A write through a symbolic link inside a write root that points out of it must be denied, judged by where the link leads rather than by the path crush names; the guard denies the tool call; the guard's reason matches write target \S&#42;/docs/README\.md is outside \S&#42;/tests | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-writes-003-symlink-escape-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-writes-004-shared-prefix-denied` | A write to a sibling directory whose name starts with a write root's name must be denied; the guard denies the tool call; the guard's reason matches write target \S&#42;/tests-old/notes\.md is outside \S&#42;/tests | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-writes-004-shared-prefix-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-writes-005-relative-path-denied` | A write naming a relative path must be denied, since a relative path does not say where the write lands; the guard denies the tool call; the guard's reason matches write target must be an absolute path, got tests/notes\.md | Retire absolute-only crush write protocol: Claude intentionally resolves paths against cwd (TestClaudeFileToolsUseDirectoryAfterReplay); root confinement is pinned by claude-guard-writes-002/003/004. |
| `crush-guard-writes-006-unnamed-path-denied` | A write that names no file must be denied; the guard denies the tool call; the guard's reason matches write tool did not name a target path | Retire the unconditional nameless-write denial: Claude resolves a missing target to cwd. Pin the surviving root restriction in claude-guard-writes-006-unnamed-path-denied, whose cwd is outside the write roots; the denial reason follows Claude's protocol. |
| `crush-guard-writes-007-host-crush-config-reported` | A host project's own crush config declaring permissions or hooks must be reported before a crush turn, and the turn must still write only inside its write roots; the command exits with code 0; stdout matches level=WARN msg="Host crush config merges over the engine's generated config" file=\S&#42;/\.crush\.json declares="\&#91;permissions hooks\&#93;" note="engine write-guard hook still applies"; the file "tests/mcp/mcp_test.go" is created; every step of scenario "INT-900" in "docs/scenarios.yaml" is matched by a step definition under "tests/mcp/steps"; no file outside "tests" and "tmp" changed | Retire crush configuration merge diagnostics and hook: no product crush configuration discovery remains; test generation remains pinned by build-tests-steps-001 and build-tests-steps-006. |
| `engine-config-002-malformed-model-entries-refused` | A command must be refused at startup when &#96;engine.models&#96; holds entries it cannot parse, naming in one run every such entry and why, a tier name the engine does not know, a model reference with no colon, one with an empty CLI and one with an empty model, and none of the sound entries; the command exits with code 1; stdout matches level=ERROR msg="Refusing to start" command="build tests"; stdout matches engine\.models: unknown model tier: \\"medium\\" \(known: xhigh, high, coder, quick\); stdout matches engine\.models\.xhigh: model reference must be \\"&lt;cli&gt;:&lt;model&gt;\\": got \\"claude-fable-5\\"; stdout matches engine\.models\.high: model reference has an empty cli: got \\":claude-opus-5\\"; stdout matches engine\.models\.coder: model reference has an empty model: got \\"crush:\\"; stdout does not match engine\.models\.quick; stdout does not match BUILD TESTS; no file outside "tmp" changed; the engine dispatched no AI turns | Retain all engine invariants and mechanical assertions in &#96;engine-config-002-malformed-model-entries-refused&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-run-006-shared-fix-skips-green-peer` | Two failing tests receive one plan and one writer for both, with one selected rerun and no validation turn; the command exits with code 0; stdout matches BUILD CODE COMPLETE; stdout matches msg="Loaded prompts" command="build code" items=1; stdout matches msg="Check answered" check=tests-pass.&#42;answer=pass; stdout matches msg="Suite failures" failures=2 framework=go-test; the engine dispatched no validation AI turns to "claude"; the engine dispatched 1 fix AI turn to "claude"; the engine dispatched 1 apply AI turn to "crush"; the engine dispatched 2 AI turns; the engine spawned exactly 3 test runners; the engine spawned a test runner with arguments matching -run \^TestAAdd\$\&#124;\^TestBAdd\$; every file matching "tmp/&#42;/apply-&#42;-user.txt" matches Whole suite:; every file matching "tmp/&#42;/apply-&#42;-user.txt" matches Selection:; every file matching "tmp/&#42;/apply-&#42;-user.txt" matches TestAAdd; every file matching "tmp/&#42;/apply-&#42;-user.txt" matches TestBAdd; the file "tests/suite/calc_test.go" is unchanged; the shell command "go test -json -count=1 ./tests/suite/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-run-006-shared-fix-skips-green-peer&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-run-007-next-walk-finds-regression` | A repair that breaks a test passing at discovery must cause the next suite run to discover and repair that regression before the command exits successfully; the command exits with code 0; stdout matches BUILD CODE COMPLETE; stdout matches msg="Suite failures" failures=0 framework=go-test; stdout matches msg=test index=1 total=1 id=.&#42;TestBCompatibility; the engine dispatched no validation AI turns to "claude"; the engine dispatched 2 fix AI turns to "claude"; the engine dispatched 2 apply AI turns to "crush"; the engine dispatched 4 AI turns; the engine spawned exactly 5 test runners; the file "tests/suite/calc_test.go" is unchanged; the shell command "go test -json -count=1 ./tests/suite/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-run-007-next-walk-finds-regression&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-frameworks-005-not-applied-revalidated` | A completed apply turn reporting applied false while its Go test still fails must keep its files and revalidate that test before another repair completes the walk; the command exits with code 0; stdout matches BUILD CODE COMPLETE; stdout matches level=WARN msg="Fix was NOT applied"; stdout does not match msg="Walk stopped"; the engine dispatched no validation AI turns to "claude"; the engine dispatched 2 fix AI turns to "claude"; the engine dispatched 2 apply AI turns to "crush"; the engine dispatched 4 AI turns; the engine spawned exactly 4 test runners; the file "services/calc/prepared.txt" matches ^prepared; the file "tests/suite/calc_test.go" is unchanged; the shell command "go test -json -count=1 ./tests/suite/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-frameworks-005-not-applied-revalidated&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-frameworks-006-not-applied-hits-fix-bound` | Not-applied turns in build code --fix must consume the configured per-set fix allowance and stop at the unchanged non-converging fix bound; the command exits with code 1; stdout matches level=WARN msg="Fix was NOT applied"; stdout matches level=ERROR msg="Walk stopped".&#42;gave up after 3 applied fix; the engine dispatched no validation AI turns to "claude"; the engine dispatched 3 fix AI turns to "claude"; the engine dispatched 3 apply AI turns to "crush"; the engine dispatched 6 AI turns; the engine spawned exactly 4 test runners; the file "services/calc/prepared-first.txt" matches ^prepared; the file "services/calc/prepared-second.txt" matches ^prepared; the file "tests/suite/calc_test.go" is unchanged | Retain all engine invariants and mechanical assertions in &#96;build-code-frameworks-006-not-applied-hits-fix-bound&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `crush-guard-reads-001-outside-project-denied` | A view of an absolute path outside the project must be denied, naming the path and the read root; the guard denies the tool call; the guard's reason matches read target /true-bdd-neighbour/secrets\.txt is outside read root \S&#42;/crush-guard-read-outside | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-reads-001-outside-project-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-reads-002-relative-escape-denied` | A glob whose path leaves the project through ../ must be denied; the guard denies the tool call; the guard's reason matches read target \S&#42;/neighbour is outside read root \S&#42;/crush-guard-read-relative-escape | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-reads-002-relative-escape-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-reads-003-symlink-escape-denied` | A read through a symbolic link inside the project pointing outside it must be denied; the guard denies the tool call; the guard's reason matches read target \S&#42;/notes\.md is outside read root \S&#42;/crush-guard-read-symlink-escape | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-reads-003-symlink-escape-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-reads-004-unnamed-path-allowed` | An ls call naming no path must be allowed to read the project; the guard allows the tool call | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-reads-004-unnamed-path-allowed&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-reads-005-missing-read-root-denied` | A tool call must be denied when crush hands the guard a write policy with no read root, naming what is missing; the guard denies the tool call; the guard's reason matches crush guard policy names no read root | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-reads-005-missing-read-root-denied&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-guard-reads-006-relative-path-allowed` | A relative read path inside the project must be allowed outside every write root; the guard allows the tool call | Delete old protocol Scenario; pin surviving file/hook invariant in &#96;claude-guard-reads-006-relative-path-allowed&#96;; crush environment and tool names retire. Claude uses its own payload/policy and existing guard reasons. |
| `crush-replay-001-whole-command` | The write guard allows whole command; the guard allows the tool call | Delete old hook protocol; pin every exact-command/selector/directory decision and refusal reason in &#96;claude-replay-001-whole-command&#96; through Claude Bash. |
| `crush-replay-002-selected-command` | The write guard allows selected command; the guard allows the tool call | Delete old hook protocol; pin every exact-command/selector/directory decision and refusal reason in &#96;claude-replay-002-selected-command&#96; through Claude Bash. |
| `crush-replay-003-shell-operator` | The write guard denies shell operator; the guard denies the tool call; the guard's reason matches allowed forms: .&#42; OR .&#42; '&lt;selection&gt;' | Delete old hook protocol; pin every exact-command/selector/directory decision and refusal reason in &#96;claude-replay-003-shell-operator&#96; through Claude Bash. |
| `crush-replay-004-assignment` | The write guard denies assignment; the guard denies the tool call; the guard's reason matches allowed forms: .&#42; OR .&#42; '&lt;selection&gt;' | Delete old hook protocol; pin every exact-command/selector/directory decision and refusal reason in &#96;claude-replay-004-assignment&#96; through Claude Bash. |
| `crush-replay-005-another-program` | The write guard denies another program; the guard denies the tool call; the guard's reason matches allowed forms: .&#42; OR .&#42; '&lt;selection&gt;' | Delete old hook protocol; pin every exact-command/selector/directory decision and refusal reason in &#96;claude-replay-005-another-program&#96; through Claude Bash. |
| `crush-replay-006-extra-flag` | The write guard denies extra flag; the guard denies the tool call; the guard's reason matches allowed forms: .&#42; OR .&#42; '&lt;selection&gt;' | Delete old hook protocol; pin every exact-command/selector/directory decision and refusal reason in &#96;claude-replay-006-extra-flag&#96; through Claude Bash. |
| `crush-replay-007-double-quotes` | The write guard denies double quotes; the guard denies the tool call; the guard's reason matches allowed forms: .&#42; OR .&#42; '&lt;selection&gt;' | Delete old hook protocol; pin every exact-command/selector/directory decision and refusal reason in &#96;claude-replay-007-double-quotes&#96; through Claude Bash. |
| `crush-replay-008-wrong-directory` | The write guard denies wrong directory; the guard denies the tool call; the guard's reason matches allowed forms: .&#42; OR .&#42; '&lt;selection&gt;' | Delete old hook protocol; pin every exact-command/selector/directory decision and refusal reason in &#96;claude-replay-008-wrong-directory&#96; through Claude Bash. |
| `crush-replay-009-no-command` | The write guard denies no command; the guard denies the tool call; the guard's reason matches allowed forms: .&#42; OR .&#42; '&lt;selection&gt;' | Delete old hook protocol; pin every exact-command/selector/directory decision and refusal reason in &#96;claude-replay-009-no-command&#96; through Claude Bash. |
| `crush-replay-010-job-output` | The write guard allows job output; the guard allows the tool call | Retire crush-specific job_output/job_kill tools and their grant dependency: neither surviving provider exposes those tools. |
| `crush-replay-011-job-without-command` | The write guard denies job without command; the guard denies the tool call | Retire crush-specific job_output/job_kill tools and their grant dependency: neither surviving provider exposes those tools. |
| `crush-replay-012-job-kill` | The write guard allows job kill; the guard allows the tool call | Retire crush-specific job_output/job_kill tools and their grant dependency: neither surviving provider exposes those tools. |
| `crush-replay-013-kill-without-command` | The write guard denies kill without command; the guard denies the tool call | Retire crush-specific job_output/job_kill tools and their grant dependency: neither surviving provider exposes those tools. |
| `crush-replay-014-writer-background-result` | The build code crush writer runs its declared replay selection through bash and reads a test lasting over a minute through job_output; the command exits with code 0; stdout matches BUILD CODE COMPLETE; every file matching "tmp/&#42;/crush-config-&#42;/crush.json" matches "bash"; every file matching "tmp/&#42;/crush-config-&#42;/crush.json" matches "job_output"; every file matching "tmp/&#42;/crush-config-&#42;/crush.json" matches "job_kill"; the file "services/calc/guard-calls.jsonl" matches "tool": "bash"; the file "services/calc/guard-calls.jsonl" matches "tool": "job_output"; the file "services/calc/guard-calls.jsonl" matches "elapsed_seconds": (&#91;6-9&#93;&#91;0-9&#93;&#124;&#91;1-9&#93;&#91;0-9&#93;{2,}); every file matching "tmp/&#42;/crush-turn&#42;-crush.log" matches job_output; every file matching "tmp/&#42;/crush-turn&#42;-crush.log" matches SELECTION COMPLETED; the file "services/calc/replayed.txt" matches ^SELECTION COMPLETED; the file "tests/suite/replay_test.go" is unchanged; the engine dispatched 1 apply AI turn to "crush" | Retire crush bash/job_output log protocol and minute-long job threshold. Writer executes granted selections and engine reruns them in build-code-codex-001-selection and build-code-claude-005-replay; codex polls native process sessions. |
| `crush-replay-015-unrepresentable-command` | A replay argument containing a single quote withholds the writer grant with one warning; the command exits with code 0; stdout matches level=WARN msg="Replay command grant unavailable"; stdout matches argument contains a single quote, newline or NUL; the file matching "tmp/true-bdd.log.json" has exactly 1 match for Replay command grant unavailable; every file matching "tmp/&#42;/crush-config-&#42;/crush.json" does not match "bash"; every file matching "tmp/&#42;/crush-config-&#42;/crush.json" does not match "job_output" | Delete old Scenario; pin canonical argument refusal, exactly one warning, and absent replay grant in &#96;build-code-claude-007-unrepresentable&#96;. Crush bash/job tool inventory assertions retire. |
| `build-code-set-001-partial-repair` | A partial repair removes the passing member from the next plan; the command exits with code 0; stdout matches BUILD CODE COMPLETE; the engine dispatched no validation AI turns to "claude"; the engine dispatched 2 fix AI turns to "claude"; the engine dispatched 2 apply AI turns to "crush"; the engine spawned exactly 4 test runners; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches TestB; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" does not match TestA; the engine spawned a test runner with arguments matching -run \^TestA\$\&#124;\^TestB\$ | Retain all engine invariants and mechanical assertions in &#96;build-code-set-001-partial-repair&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-set-002-shared-evidence` | Shared failure output is shown once with both ids in the set request; the command exits with code 0; the engine dispatched 1 fix AI turn to "claude"; the engine dispatched 1 apply AI turn to "crush"; the engine spawned exactly 3 test runners; the file matching "tmp/&#42;/&#42;fix-iter1-user.txt" has exactly 1 match for SHARED-FAILURE-EVIDENCE; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches Tests: .&#42;TestA, .&#42;TestB; every file matching "tmp/&#42;/apply-&#42;-user.txt" matches Whole suite:; every file matching "tmp/&#42;/apply-&#42;-user.txt" matches Selection: | Retain all engine invariants and mechanical assertions in &#96;build-code-set-002-shared-evidence&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-set-003-mixed-synthetic` | A build failure and a named test form one set whose rerun is unfiltered; the command exits with code 0; stdout matches msg="Suite failures" failures=2 framework=go-test; the engine spawned exactly 3 test runners; the engine dispatched 1 fix AI turn to "claude"; the engine dispatched 1 apply AI turn to "crush"; stdout does not match Spawning test runner.&#42;-run; the shell command "go test -json -count=1 ./tests/suite/... ./tests/broken/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-set-003-mixed-synthetic&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-set-004-missing-result` | A skipped selected member stays failing until its own passing result; the command exits with code 0; the engine dispatched 2 fix AI turns to "claude"; the engine dispatched 2 apply AI turns to "crush"; the engine spawned exactly 4 test runners; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches did not run; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches TestA | Retain all engine invariants and mechanical assertions in &#96;build-code-set-004-missing-result&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-set-005-shrinking-counter` | Shrinking the set does not reset its existing attempt counter and the last apply is rerun; the command exits with code 1; stdout matches gave up after 2 applied fix; the engine dispatched 2 apply AI turns to "crush"; the engine spawned exactly 3 test runners; the file "services/calc/attempted.txt" matches attempted; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches TestB; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" does not match TestA | Retain all engine invariants and mechanical assertions in &#96;build-code-set-005-shrinking-counter&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-confirmation-001-passes` | A completed writer response without confirmation preserves its repair, warns once and reruns the set to convergence; the command exits with code 0; stdout matches level=WARN msg="Writer confirmation missing".&#42;apply-failing-set-iter1-result.yaml; the file matching "tmp/true-bdd.log.json" has exactly 1 match for "level":"WARN","msg":"Writer confirmation missing"; the engine dispatched no validation AI turns to "claude"; the engine dispatched 1 fix AI turn to "claude"; the engine dispatched 1 apply AI turn to "crush"; the engine spawned exactly 3 test runners; the file "services/calc/calc.go" is modified; the file "tests/suite/calc_test.go" is unchanged; stdout matches BUILD CODE COMPLETE; stdout does not match msg="Walk stopped"; the shell command "go test -json -count=1 ./tests/suite/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-confirmation-001-passes&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-confirmation-002-retries` | A missing confirmation with failures remaining replans only the remaining tests; the command exits with code 0; stdout matches level=WARN msg="Writer confirmation missing".&#42;apply-failing-set-iter1-result.yaml; the file matching "tmp/true-bdd.log.json" has exactly 2 matches for "level":"WARN","msg":"Writer confirmation missing"; the engine dispatched no validation AI turns to "claude"; the engine dispatched 2 fix AI turns to "claude"; the engine dispatched 2 apply AI turns to "crush"; the engine spawned exactly 4 test runners; the file "services/calc/calc.go" is modified; the file "tests/suite/calc_test.go" is unchanged; stdout matches BUILD CODE COMPLETE; stdout does not match msg="Walk stopped"; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches TestB; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" does not match TestA; the shell command "go test -json -count=1 ./tests/suite/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-confirmation-002-retries&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-confirmation-003-bound` | Missing confirmations consume the same allowance and rerun the terminal repair before the non-converging bound even when it passes; the command exits with code 1; stdout matches level=WARN msg="Writer confirmation missing".&#42;apply-failing-set-iter1-result.yaml; the file matching "tmp/true-bdd.log.json" has exactly 3 matches for "level":"WARN","msg":"Writer confirmation missing"; the engine dispatched no validation AI turns to "claude"; the engine dispatched 3 fix AI turns to "claude"; the engine dispatched 3 apply AI turns to "crush"; the engine spawned exactly 4 test runners; the file "services/calc/calc.go" is modified; the file "tests/suite/calc_test.go" is unchanged; stdout matches level=ERROR msg="Walk stopped".&#42;gave up after 3 applied fix; the file "services/calc/prepared.txt" matches ^prepared; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches TestB; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" does not match TestA; the shell command "go test -json -count=1 ./tests/suite/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-confirmation-003-bound&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-writer-001-nonzero` | A non-zero writer preserves its files, reruns a build failure and replans from the new evidence to convergence; the command exits with code 0; stdout matches level=WARN msg="Writer turn failed" outcome="non-zero exit"; the file matching "tmp/true-bdd.log.json" has exactly 2 matches for "level":"WARN","msg":"Writer turn failed"; the engine dispatched no validation AI turns to "claude"; the engine dispatched 2 fix AI turns to "claude"; the engine dispatched 2 apply AI turns to "crush"; the engine spawned exactly 4 test runners; the file "services/calc/calc.go" is modified; the file "tests/suite/calc_test.go" is unchanged; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches closureGlossary; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches build failed; stdout matches BUILD CODE COMPLETE; stdout does not match msg="Walk stopped"; the shell command "go test -json -count=1 ./tests/suite/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-writer-001-nonzero&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-writer-002-empty` | An empty writer answer preserves its repair, warns once and reruns the set to convergence; the command exits with code 0; stdout matches level=WARN msg="Writer turn failed" outcome="empty output"; the file matching "tmp/true-bdd.log.json" has exactly 1 match for "level":"WARN","msg":"Writer turn failed"; the engine dispatched no validation AI turns to "claude"; the engine dispatched 1 fix AI turn to "claude"; the engine dispatched 1 apply AI turn to "crush"; the engine spawned exactly 3 test runners; the file "services/calc/calc.go" is modified; the file "tests/suite/calc_test.go" is unchanged; stdout matches BUILD CODE COMPLETE; stdout does not match msg="Walk stopped"; the shell command "go test -json -count=1 ./tests/suite/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-writer-002-empty&#96;; writer/apply CLI and malformed-model examples become codex.  |
| `build-code-writer-003-bound` | Failed writers consume the existing allowance and rerun the terminal repair before the bound even when it passes; the command exits with code 1; stdout matches level=WARN msg="Writer turn failed" outcome="non-zero exit"; the file matching "tmp/true-bdd.log.json" has exactly 3 matches for "level":"WARN","msg":"Writer turn failed"; the engine dispatched no validation AI turns to "claude"; the engine dispatched 3 fix AI turns to "claude"; the engine dispatched 3 apply AI turns to "crush"; the engine spawned exactly 4 test runners; the file "services/calc/calc.go" is modified; the file "tests/suite/calc_test.go" is unchanged; stdout matches level=ERROR msg="Walk stopped".&#42;gave up after 3 applied fix; the file "services/calc/prepared.txt" matches ^prepared; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" matches TestB; every file matching "tmp/&#42;/&#42;fix-iter1-user.txt" does not match TestA; the shell command "go test -json -count=1 ./tests/suite/..." run in "." exits with code 0 | Retain all engine invariants and mechanical assertions in &#96;build-code-writer-003-bound&#96;; writer/apply CLI and malformed-model examples become codex.  |

## Original steps

These exact original steps make later rewritten-step review possible. The continuation ledger will pair them with the final Registry steps.

### engine-004-unknown-model-cli-refused

```json
{
  "given": [
    "the \"build-tests-unknown-cli-tier\" project tree"
  ],
  "when": [
    "the System Architect runs \"build tests\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches level=ERROR msg=\"Refusing to start\" command=\"build tests\"",
    {
      "and": "stdout matches engine\\.models\\.coder: model reference names an unknown cli: \\\\\"aider\\\\\" \\(supported: claude, crush, codex\\)"
    },
    {
      "but": "stdout does not match BUILD TESTS"
    },
    {
      "and": "no file outside \"tmp\" changed"
    },
    {
      "and": "the file \"docs/scenarios.yaml\" is unchanged"
    },
    {
      "and": "the engine dispatched no AI turns"
    }
  ]
}
```

### scen-check-fix-001-unmatched-step-gets-phrase

```json
{
  "given": [
    "the \"scen-check-fix-records-phrase\" project tree",
    "the Quality Engineer answers \"1\" to every question"
  ],
  "when": [
    "the Quality Engineer runs \"scen check --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches CHECK FAILED",
    {
      "and": "stdout matches E2E-001: then step \"the service answers within 200 milliseconds\" matches no template"
    },
    {
      "and": "stdout matches msg=\"AI needs clarification\""
    },
    {
      "and": "stdout matches FIX PROMPT GENERATED"
    },
    {
      "and": "stdout matches msg=\"Fix recorded in the document\" document=product"
    },
    {
      "and": "stdout matches check=step-phrases subjectID=E2E-001 promptIndex=1 answer=pass"
    },
    {
      "and": "stdout matches ALL CHECKS PASSED"
    },
    {
      "and": "the only file changed outside \"tmp\" is \"docs/product/product.yaml\""
    },
    {
      "and": "the file \"docs/product/product.yaml\" matches the service answers within"
    },
    {
      "and": "the engine dispatched 3 AI turns"
    },
    {
      "and": "every file matching \"tmp/*/crush-config-*/crush.json\" does not match \"bash\""
    },
    {
      "and": "every file matching \"tmp/*/crush-config-*/crush.json\" does not match \"job_output\""
    }
  ]
}
```

### engine-routes-002-crush-asks-before-fixing

```json
{
  "given": [
    "the \"scen-check-fix-by-crush\" project tree",
    {
      "and": "the \"scen-check\" checklist is narrowed to its \"whether this scenario includes at least one specific\" prompt"
    },
    "the Quality Engineer answers \"1\" to every question"
  ],
  "when": [
    "the Quality Engineer runs \"scen check --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches CHECK FAILED",
    {
      "and": "stdout matches msg=\"AI needs clarification\" questionCount=1"
    },
    {
      "and": "stdout matches FIX PROMPT GENERATED"
    },
    {
      "and": "stdout matches msg=\"Fix recorded in the document\" document=scenarios_yaml"
    },
    {
      "and": "stdout matches ALL CHECKS PASSED"
    },
    {
      "and": "the engine dispatched 3 validation AI turns to \"crush\""
    },
    {
      "and": "the engine dispatched 2 fix AI turns to \"crush\""
    },
    {
      "and": "the engine dispatched 1 apply AI turn to \"crush\""
    },
    {
      "and": "the engine dispatched 6 AI turns"
    },
    {
      "and": "the only file changed outside \"tmp\" is \"docs/scenarios.yaml\""
    }
  ]
}
```

### engine-routes-003-prompt-tiers-win

```json
{
  "given": [
    "the \"scen-check-prompt-names-its-tiers\" project tree",
    "the Quality Engineer answers \"1\" to every question"
  ],
  "when": [
    "the Quality Engineer runs \"scen check --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches ALL CHECKS PASSED",
    {
      "and": "the engine dispatched 3 validation AI turns to \"crush\""
    },
    {
      "and": "the engine dispatched 1 fix AI turn to \"codex\""
    },
    {
      "and": "the engine dispatched 1 apply AI turn to \"claude\""
    },
    {
      "but": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched no fix AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched no apply AI turns to \"crush\""
    },
    {
      "and": "the engine dispatched 5 AI turns"
    },
    {
      "and": "the only file changed outside \"tmp\" is \"docs/scenarios.yaml\""
    }
  ]
}
```

### engine-routes-005-failed-crush-turn-stops-run

```json
{
  "given": [
    "the \"scen-check-crush-turn-fails\" project tree",
    {
      "and": "the \"scen-check\" checklist is narrowed to its \"whether this scenario includes at least one specific\" prompt"
    }
  ],
  "when": [
    "the Quality Engineer runs \"scen check\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches msg=\"AI turn failed\" turn=1 label=crush-turn001 role=prompt cli=crush model=zhipu-coding/no-such-model",
    {
      "but": "stdout does not match CHECK FAILED|ALL CHECKS PASSED"
    },
    {
      "and": "the engine dispatched 1 AI turn"
    },
    {
      "and": "no file outside \"tmp\" changed"
    }
  ]
}
```

### crush-guard-001-read-only-tool-allowed

```json
{
  "given": [
    "the \"crush-guard-read-only-tool\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"view\"",
    "crush names the file \"docs/README.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-guard-002-shell-denied-without-grant

```json
{
  "given": [
    "the \"crush-guard-shell-denied\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"bash\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches shell is denied in this mode"
    }
  ]
}
```

### crush-guard-003-shell-allowed-with-grant

```json
{
  "given": [
    "the \"crush-guard-shell-allowed\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    {
      "and": "crush's write policy lets the turn run the shell"
    },
    "crush names the tool \"bash\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-guard-004-unknown-tool-denied

```json
{
  "given": [
    "the \"crush-guard-unknown-tool\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"rename\"",
    "crush names the file \"tests/notes.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches tool rename is not on the allowlist \\(unknown tools are treated as writers\\)"
    }
  ]
}
```

### crush-guard-005-missing-policy-denied

```json
{
  "given": [
    "the \"crush-guard-missing-policy\" project tree",
    "crush hands the guard no write policy",
    "crush names the tool \"view\"",
    "crush names the file \"docs/README.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches crush guard policy is not set"
    }
  ]
}
```

### crush-guard-006-unreadable-policy-denied

```json
{
  "given": [
    "the \"crush-guard-unreadable-policy\" project tree",
    "crush hands the guard an unreadable write policy",
    "crush names the tool \"view\"",
    "crush names the file \"docs/README.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches crush guard policy is malformed"
    }
  ]
}
```

### crush-guard-007-payload-names-the-call

```json
{
  "given": [
    "the \"crush-guard-hook-payload\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush pipes the hook payload naming the tool \"write\" and the file \"tests/notes.md\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-guard-writes-001-write-inside-root-allowed

```json
{
  "given": [
    "the \"crush-guard-write-inside-root\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"write\"",
    "crush names the file \"tests/notes.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-guard-writes-002-write-outside-roots-denied

```json
{
  "given": [
    "the \"crush-guard-write-outside-roots\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    {
      "and": "crush's write policy lets the turn write under \"services\""
    },
    "crush names the tool \"write\"",
    "crush names the file \"docs/notes.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches write target \\S*/docs/notes\\.md is outside \\S*/tests, \\S*/services"
    }
  ]
}
```

### crush-guard-writes-003-symlink-escape-denied

```json
{
  "given": [
    "the \"crush-guard-symlink-escape\" project tree",
    "the project tree's \"tests/node_modules/shared\" is a symbolic link to \"../../docs\"",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"write\"",
    "crush names the file \"tests/node_modules/shared/README.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches write target \\S*/docs/README\\.md is outside \\S*/tests"
    }
  ]
}
```

### crush-guard-writes-004-shared-prefix-denied

```json
{
  "given": [
    "the \"crush-guard-shared-prefix\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"write\"",
    "crush names the file \"tests-old/notes.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches write target \\S*/tests-old/notes\\.md is outside \\S*/tests"
    }
  ]
}
```

### crush-guard-writes-005-relative-path-denied

```json
{
  "given": [
    "the \"crush-guard-relative-path\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"write\"",
    "crush names the path \"tests/notes.md\" as written"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches write target must be an absolute path, got tests/notes\\.md"
    }
  ]
}
```

### crush-guard-writes-006-unnamed-path-denied

```json
{
  "given": [
    "the \"crush-guard-unnamed-path\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"write\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches write tool did not name a target path"
    }
  ]
}
```

### crush-guard-writes-007-host-crush-config-reported

```json
{
  "given": [
    "the \"build-tests-host-crush-config\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build tests --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Host crush config merges over the engine's generated config\" file=\\S*/\\.crush\\.json declares=\"\\[permissions hooks\\]\" note=\"engine write-guard hook still applies\"",
    {
      "and": "the file \"tests/mcp/mcp_test.go\" is created"
    },
    {
      "and": "every step of scenario \"INT-900\" in \"docs/scenarios.yaml\" is matched by a step definition under \"tests/mcp/steps\""
    },
    {
      "but": "no file outside \"tests\" and \"tmp\" changed"
    }
  ]
}
```

### engine-config-002-malformed-model-entries-refused

```json
{
  "given": [
    "the \"build-tests-malformed-model-entries\" project tree"
  ],
  "when": [
    "the System Architect runs \"build tests\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches level=ERROR msg=\"Refusing to start\" command=\"build tests\"",
    {
      "and": "stdout matches engine\\.models: unknown model tier: \\\\\"medium\\\\\" \\(known: xhigh, high, coder, quick\\)"
    },
    {
      "and": "stdout matches engine\\.models\\.xhigh: model reference must be \\\\\"<cli>:<model>\\\\\": got \\\\\"claude-fable-5\\\\\""
    },
    {
      "and": "stdout matches engine\\.models\\.high: model reference has an empty cli: got \\\\\":claude-opus-5\\\\\""
    },
    {
      "and": "stdout matches engine\\.models\\.coder: model reference has an empty model: got \\\\\"crush:\\\\\""
    },
    {
      "but": "stdout does not match engine\\.models\\.quick"
    },
    {
      "and": "stdout does not match BUILD TESTS"
    },
    {
      "and": "no file outside \"tmp\" changed"
    },
    {
      "and": "the engine dispatched no AI turns"
    }
  ]
}
```

### build-code-run-006-shared-fix-skips-green-peer

```json
{
  "given": [
    "the \"build-code-shared-fix\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches BUILD CODE COMPLETE",
    "stdout matches msg=\"Loaded prompts\" command=\"build code\" items=1",
    "stdout matches msg=\"Check answered\" check=tests-pass.*answer=pass",
    {
      "and": "stdout matches msg=\"Suite failures\" failures=2 framework=go-test"
    },
    {
      "and": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 1 fix AI turn to \"claude\""
    },
    {
      "and": "the engine dispatched 1 apply AI turn to \"crush\""
    },
    {
      "and": "the engine dispatched 2 AI turns"
    },
    {
      "and": "the engine spawned exactly 3 test runners"
    },
    {
      "and": "the engine spawned a test runner with arguments matching -run \\^TestAAdd\\$\\|\\^TestBAdd\\$"
    },
    "every file matching \"tmp/*/apply-*-user.txt\" matches Whole suite:",
    "every file matching \"tmp/*/apply-*-user.txt\" matches Selection:",
    "every file matching \"tmp/*/apply-*-user.txt\" matches TestAAdd",
    "every file matching \"tmp/*/apply-*-user.txt\" matches TestBAdd",
    {
      "but": "the file \"tests/suite/calc_test.go\" is unchanged"
    },
    {
      "and": "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
    }
  ]
}
```

### build-code-run-007-next-walk-finds-regression

```json
{
  "given": [
    "the \"build-code-cross-walk\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches BUILD CODE COMPLETE",
    {
      "and": "stdout matches msg=\"Suite failures\" failures=0 framework=go-test"
    },
    {
      "and": "stdout matches msg=test index=1 total=1 id=.*TestBCompatibility"
    },
    {
      "and": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 2 fix AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 2 apply AI turns to \"crush\""
    },
    {
      "and": "the engine dispatched 4 AI turns"
    },
    {
      "and": "the engine spawned exactly 5 test runners"
    },
    {
      "but": "the file \"tests/suite/calc_test.go\" is unchanged"
    },
    {
      "and": "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
    }
  ]
}
```

### build-code-frameworks-005-not-applied-revalidated

```json
{
  "given": [
    "the \"build-code-not-applied\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches BUILD CODE COMPLETE",
    {
      "and": "stdout matches level=WARN msg=\"Fix was NOT applied\""
    },
    {
      "but": "stdout does not match msg=\"Walk stopped\""
    },
    {
      "and": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 2 fix AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 2 apply AI turns to \"crush\""
    },
    {
      "and": "the engine dispatched 4 AI turns"
    },
    {
      "and": "the engine spawned exactly 4 test runners"
    },
    {
      "and": "the file \"services/calc/prepared.txt\" matches ^prepared"
    },
    {
      "but": "the file \"tests/suite/calc_test.go\" is unchanged"
    },
    {
      "and": "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
    }
  ]
}
```

### build-code-frameworks-006-not-applied-hits-fix-bound

```json
{
  "given": [
    "the \"build-code-not-applied-bound\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 1",
    {
      "and": "stdout matches level=WARN msg=\"Fix was NOT applied\""
    },
    {
      "and": "stdout matches level=ERROR msg=\"Walk stopped\".*gave up after 3 applied fix"
    },
    {
      "and": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 3 fix AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 3 apply AI turns to \"crush\""
    },
    {
      "and": "the engine dispatched 6 AI turns"
    },
    {
      "and": "the engine spawned exactly 4 test runners"
    },
    {
      "and": "the file \"services/calc/prepared-first.txt\" matches ^prepared"
    },
    {
      "and": "the file \"services/calc/prepared-second.txt\" matches ^prepared"
    },
    {
      "but": "the file \"tests/suite/calc_test.go\" is unchanged"
    }
  ]
}
```

### crush-guard-reads-001-outside-project-denied

```json
{
  "given": [
    "the \"crush-guard-read-outside\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"view\"",
    "crush names the path \"/true-bdd-neighbour/secrets.txt\" as written"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches read target /true-bdd-neighbour/secrets\\.txt is outside read root \\S*/crush-guard-read-outside"
    }
  ]
}
```

### crush-guard-reads-002-relative-escape-denied

```json
{
  "given": [
    "the \"crush-guard-read-relative-escape\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush pipes the hook payload naming the tool \"glob\" and the path \"../neighbour\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches read target \\S*/neighbour is outside read root \\S*/crush-guard-read-relative-escape"
    }
  ]
}
```

### crush-guard-reads-003-symlink-escape-denied

```json
{
  "given": [
    "the \"crush-guard-read-symlink-escape\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "the project tree's \"node_modules/shared\" is a symbolic link to \"../../\"",
    "crush names the tool \"grep\"",
    "crush names the file \"node_modules/shared/notes.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches read target \\S*/notes\\.md is outside read root \\S*/crush-guard-read-symlink-escape"
    }
  ]
}
```

### crush-guard-reads-004-unnamed-path-allowed

```json
{
  "given": [
    "the \"crush-guard-read-unnamed-path\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"ls\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-guard-reads-005-missing-read-root-denied

```json
{
  "given": [
    "the \"crush-guard-read-missing-root\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush hands the guard a write policy with no read root",
    "crush names the tool \"view\"",
    "crush names the file \"docs/README.md\" of the project tree"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches crush guard policy names no read root"
    }
  ]
}
```

### crush-guard-reads-006-relative-path-allowed

```json
{
  "given": [
    "the \"crush-guard-read-relative-path\" project tree",
    "crush's write policy lets the turn write under \"tests\"",
    "crush names the tool \"diagnostics\"",
    "crush names the path \"docs/README.md\" as written"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-replay-001-whole-command

```json
{
  "given": [
    "the \"crush-replay-whole-command\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush pipes a shell call in directory \".\" with command go test ./..."
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-replay-002-selected-command

```json
{
  "given": [
    "the \"crush-replay-selected-command\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush pipes a shell call in directory \".\" with command go test ./... -run '^TestSlow$'"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-replay-003-shell-operator

```json
{
  "given": [
    "the \"crush-replay-shell-operator\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush pipes a shell call in directory \".\" with command go test ./...; cat /etc/passwd"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-004-assignment

```json
{
  "given": [
    "the \"crush-replay-assignment\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush pipes a shell call in directory \".\" with command X=1 go test ./..."
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-005-another-program

```json
{
  "given": [
    "the \"crush-replay-another-program\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush pipes a shell call in directory \".\" with command cat /etc/passwd"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-006-extra-flag

```json
{
  "given": [
    "the \"crush-replay-extra-flag\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush pipes a shell call in directory \".\" with command go test ./... -v"
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-007-double-quotes

```json
{
  "given": [
    "the \"crush-replay-double-quotes\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush pipes a shell call in directory \".\" with command go test ./... -run \"^TestSlow$\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-008-wrong-directory

```json
{
  "given": [
    "the \"crush-replay-wrong-directory\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush pipes a shell call in directory \"tests\" with command go test ./..."
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-009-no-command

```json
{
  "given": [
    "the \"crush-replay-no-command\" project tree",
    "crush pipes a shell call in directory \".\" with command go test ./..."
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-010-job-output

```json
{
  "given": [
    "the \"crush-replay-job-output\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush names the tool \"job_output\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-replay-011-job-without-command

```json
{
  "given": [
    "the \"crush-replay-job-without-command\" project tree",
    "crush names the tool \"job_output\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call"
  ]
}
```

### crush-replay-012-job-kill

```json
{
  "given": [
    "the \"crush-replay-job-kill\" project tree",
    "crush's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "crush names the tool \"job_kill\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-replay-013-kill-without-command

```json
{
  "given": [
    "the \"crush-replay-kill-without-command\" project tree",
    "crush names the tool \"job_kill\""
  ],
  "when": [
    "crush asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call"
  ]
}
```

### crush-replay-014-writer-background-result

```json
{
  "given": [
    "the \"build-code-writer-replay\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches BUILD CODE COMPLETE",
    "every file matching \"tmp/*/crush-config-*/crush.json\" matches \"bash\"",
    "every file matching \"tmp/*/crush-config-*/crush.json\" matches \"job_output\"",
    "every file matching \"tmp/*/crush-config-*/crush.json\" matches \"job_kill\"",
    "the file \"services/calc/guard-calls.jsonl\" matches \"tool\": \"bash\"",
    "the file \"services/calc/guard-calls.jsonl\" matches \"tool\": \"job_output\"",
    "the file \"services/calc/guard-calls.jsonl\" matches \"elapsed_seconds\": ([6-9][0-9]|[1-9][0-9]{2,})",
    "every file matching \"tmp/*/crush-turn*-crush.log\" matches job_output",
    "every file matching \"tmp/*/crush-turn*-crush.log\" matches SELECTION COMPLETED",
    "the file \"services/calc/replayed.txt\" matches ^SELECTION COMPLETED",
    "the file \"tests/suite/replay_test.go\" is unchanged",
    "the engine dispatched 1 apply AI turn to \"crush\""
  ]
}
```

### crush-replay-015-unrepresentable-command

```json
{
  "given": [
    "the \"build-code-unrepresentable-replay\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Replay command grant unavailable\"",
    "stdout matches argument contains a single quote, newline or NUL",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 1 match for Replay command grant unavailable",
    "every file matching \"tmp/*/crush-config-*/crush.json\" does not match \"bash\"",
    "every file matching \"tmp/*/crush-config-*/crush.json\" does not match \"job_output\""
  ]
}
```

### build-code-set-001-partial-repair

```json
{
  "given": [
    "the \"build-code-partial\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches BUILD CODE COMPLETE",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 2 fix AI turns to \"claude\"",
    "the engine dispatched 2 apply AI turns to \"crush\"",
    "the engine spawned exactly 4 test runners",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA",
    "the engine spawned a test runner with arguments matching -run \\^TestA\\$\\|\\^TestB\\$"
  ]
}
```

### build-code-set-002-shared-evidence

```json
{
  "given": [
    "the \"build-code-grouped\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "the engine dispatched 1 fix AI turn to \"claude\"",
    "the engine dispatched 1 apply AI turn to \"crush\"",
    "the engine spawned exactly 3 test runners",
    "the file matching \"tmp/*/*fix-iter1-user.txt\" has exactly 1 match for SHARED-FAILURE-EVIDENCE",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches Tests: .*TestA, .*TestB",
    "every file matching \"tmp/*/apply-*-user.txt\" matches Whole suite:",
    "every file matching \"tmp/*/apply-*-user.txt\" matches Selection:"
  ]
}
```

### build-code-set-003-mixed-synthetic

```json
{
  "given": [
    "the \"build-code-mixed-build\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches msg=\"Suite failures\" failures=2 framework=go-test",
    "the engine spawned exactly 3 test runners",
    "the engine dispatched 1 fix AI turn to \"claude\"",
    "the engine dispatched 1 apply AI turn to \"crush\"",
    "stdout does not match Spawning test runner.*-run",
    "the shell command \"go test -json -count=1 ./tests/suite/... ./tests/broken/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-set-004-missing-result

```json
{
  "given": [
    "the \"build-code-missing-member\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "the engine dispatched 2 fix AI turns to \"claude\"",
    "the engine dispatched 2 apply AI turns to \"crush\"",
    "the engine spawned exactly 4 test runners",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches did not run",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestA"
  ]
}
```

### build-code-set-005-shrinking-counter

```json
{
  "given": [
    "the \"build-code-shrinking-bound\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches gave up after 2 applied fix",
    "the engine dispatched 2 apply AI turns to \"crush\"",
    "the engine spawned exactly 3 test runners",
    "the file \"services/calc/attempted.txt\" matches attempted",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA"
  ]
}
```

### build-code-confirmation-001-passes

```json
{
  "given": [
    "the \"build-code-no-confirmation-passes\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Writer confirmation missing\".*apply-failing-set-iter1-result.yaml",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 1 match for \"level\":\"WARN\",\"msg\":\"Writer confirmation missing\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 1 fix AI turn to \"claude\"",
    "the engine dispatched 1 apply AI turn to \"crush\"",
    "the engine spawned exactly 3 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches BUILD CODE COMPLETE",
    "stdout does not match msg=\"Walk stopped\"",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-confirmation-002-retries

```json
{
  "given": [
    "the \"build-code-no-confirmation-retries\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Writer confirmation missing\".*apply-failing-set-iter1-result.yaml",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 2 matches for \"level\":\"WARN\",\"msg\":\"Writer confirmation missing\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 2 fix AI turns to \"claude\"",
    "the engine dispatched 2 apply AI turns to \"crush\"",
    "the engine spawned exactly 4 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches BUILD CODE COMPLETE",
    "stdout does not match msg=\"Walk stopped\"",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-confirmation-003-bound

```json
{
  "given": [
    "the \"build-code-no-confirmation-bound\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches level=WARN msg=\"Writer confirmation missing\".*apply-failing-set-iter1-result.yaml",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 3 matches for \"level\":\"WARN\",\"msg\":\"Writer confirmation missing\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 3 fix AI turns to \"claude\"",
    "the engine dispatched 3 apply AI turns to \"crush\"",
    "the engine spawned exactly 4 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches level=ERROR msg=\"Walk stopped\".*gave up after 3 applied fix",
    "the file \"services/calc/prepared.txt\" matches ^prepared",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-writer-001-nonzero

```json
{
  "given": [
    "the \"build-code-writer-nonzero\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Writer turn failed\" outcome=\"non-zero exit\"",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 2 matches for \"level\":\"WARN\",\"msg\":\"Writer turn failed\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 2 fix AI turns to \"claude\"",
    "the engine dispatched 2 apply AI turns to \"crush\"",
    "the engine spawned exactly 4 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches closureGlossary",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches build failed",
    "stdout matches BUILD CODE COMPLETE",
    "stdout does not match msg=\"Walk stopped\"",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-writer-002-empty

```json
{
  "given": [
    "the \"build-code-writer-empty\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Writer turn failed\" outcome=\"empty output\"",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 1 match for \"level\":\"WARN\",\"msg\":\"Writer turn failed\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 1 fix AI turn to \"claude\"",
    "the engine dispatched 1 apply AI turn to \"crush\"",
    "the engine spawned exactly 3 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches BUILD CODE COMPLETE",
    "stdout does not match msg=\"Walk stopped\"",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-writer-003-bound

```json
{
  "given": [
    "the \"build-code-writer-bound\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches level=WARN msg=\"Writer turn failed\" outcome=\"non-zero exit\"",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 3 matches for \"level\":\"WARN\",\"msg\":\"Writer turn failed\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 3 fix AI turns to \"claude\"",
    "the engine dispatched 3 apply AI turns to \"crush\"",
    "the engine spawned exactly 4 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches level=ERROR msg=\"Walk stopped\".*gave up after 3 applied fix",
    "the file \"services/calc/prepared.txt\" matches ^prepared",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

## Final steps after replacement

### engine-004-unknown-model-cli-refused → engine-004-unknown-model-cli-refused

```json
{
  "given": [
    "the \"build-tests-unknown-cli-tier\" project tree"
  ],
  "when": [
    "the System Architect runs \"build tests\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches level=ERROR msg=\"Refusing to start\" command=\"build tests\"",
    {
      "and": "stdout matches engine\\.models\\.coder: model reference names an unknown cli: \\\\\"crush\\\\\" \\(supported: claude, codex\\)"
    },
    {
      "but": "stdout does not match BUILD TESTS"
    },
    {
      "and": "no file outside \"tmp\" changed"
    },
    {
      "and": "the file \"docs/scenarios.yaml\" is unchanged"
    },
    {
      "and": "the engine dispatched no AI turns"
    }
  ]
}
```

### scen-check-fix-001-unmatched-step-gets-phrase → scen-check-fix-001-unmatched-step-gets-phrase

```json
{
  "given": [
    "the \"scen-check-fix-records-phrase\" project tree",
    "the Quality Engineer answers \"1\" to every question"
  ],
  "when": [
    "the Quality Engineer runs \"scen check --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches CHECK FAILED",
    {
      "and": "stdout matches E2E-001: then step \"the service answers within 200 milliseconds\" matches no template"
    },
    {
      "and": "stdout matches msg=\"AI needs clarification\""
    },
    {
      "and": "stdout matches FIX PROMPT GENERATED"
    },
    {
      "and": "stdout matches msg=\"Fix recorded in the document\" document=product"
    },
    {
      "and": "stdout matches check=step-phrases subjectID=E2E-001 promptIndex=1 answer=pass"
    },
    {
      "and": "stdout matches ALL CHECKS PASSED"
    },
    {
      "and": "the only file changed outside \"tmp\" is \"docs/product/product.yaml\""
    },
    {
      "and": "the file \"docs/product/product.yaml\" matches the service answers within"
    },
    {
      "and": "the engine dispatched 3 AI turns"
    }
  ]
}
```

### engine-routes-003-prompt-tiers-win → engine-routes-003-prompt-tiers-win

```json
{
  "given": [
    "the \"scen-check-prompt-names-its-tiers\" project tree",
    "the Quality Engineer answers \"1\" to every question"
  ],
  "when": [
    "the Quality Engineer runs \"scen check --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches ALL CHECKS PASSED",
    {
      "and": "the engine dispatched 3 validation AI turns to \"codex\""
    },
    {
      "and": "the engine dispatched 1 fix AI turn to \"codex\""
    },
    {
      "and": "the engine dispatched 1 apply AI turn to \"claude\""
    },
    {
      "but": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched no fix AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched no apply AI turns to \"codex\""
    },
    {
      "and": "the engine dispatched 5 AI turns"
    },
    {
      "and": "the only file changed outside \"tmp\" is \"docs/scenarios.yaml\""
    }
  ]
}
```

### engine-routes-005-failed-crush-turn-stops-run → engine-routes-005-failed-codex-turn-stops-run

```json
{
  "given": [
    "the \"scen-check-codex-turn-fails\" project tree",
    {
      "and": "the \"scen-check\" checklist is narrowed to its \"whether this scenario includes at least one specific\" prompt"
    }
  ],
  "when": [
    "the Quality Engineer runs \"scen check\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches msg=\"AI turn failed\" turn=1 label=codex-turn001 role=prompt cli=codex model=codex-no-such-model",
    {
      "but": "stdout does not match CHECK FAILED|ALL CHECKS PASSED"
    },
    {
      "and": "the engine dispatched 1 AI turn"
    },
    {
      "and": "no file outside \"tmp\" changed"
    }
  ]
}
```

### crush-guard-001-read-only-tool-allowed → claude-guard-001-read-only-tool-allowed

```json
{
  "given": [
    "the \"claude-guard-read-only-tool\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"Read\"",
    "claude names the file \"docs/README.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-guard-002-shell-denied-without-grant → claude-guard-002-shell-denied-without-grant

```json
{
  "given": [
    "the \"claude-guard-shell-denied\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"Bash\""
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches shell is denied in this mode"
    }
  ]
}
```

### crush-guard-004-unknown-tool-denied → claude-guard-004-unknown-tool-denied

```json
{
  "given": [
    "the \"claude-guard-unknown-tool\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"rename\"",
    "claude names the file \"tests/notes.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches tool is not on the claude writer allowlist"
    }
  ]
}
```

### crush-guard-005-missing-policy-denied → claude-guard-005-missing-policy-denied

```json
{
  "given": [
    "the \"claude-guard-missing-policy\" project tree",
    "claude hands the guard no write policy",
    "claude names the tool \"Read\"",
    "claude names the file \"docs/README.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches cannot read claude guard policy"
    }
  ]
}
```

### crush-guard-006-unreadable-policy-denied → claude-guard-006-unreadable-policy-denied

```json
{
  "given": [
    "the \"claude-guard-unreadable-policy\" project tree",
    "claude hands the guard an unreadable write policy",
    "claude names the tool \"Read\"",
    "claude names the file \"docs/README.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches invalid claude guard"
    }
  ]
}
```

### crush-guard-007-payload-names-the-call → claude-guard-007-payload-names-the-call

```json
{
  "given": [
    "the \"claude-guard-hook-payload\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude pipes the hook payload naming the tool \"Write\" and the file \"tests/notes.md\""
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-guard-writes-001-write-inside-root-allowed → claude-guard-writes-001-write-inside-root-allowed

```json
{
  "given": [
    "the \"claude-guard-write-inside-root\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"Write\"",
    "claude names the file \"tests/notes.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-guard-writes-002-write-outside-roots-denied → claude-guard-writes-002-write-outside-roots-denied

```json
{
  "given": [
    "the \"claude-guard-write-outside-roots\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    {
      "and": "claude's write policy lets the turn write under \"services\""
    },
    "claude names the tool \"Write\"",
    "claude names the file \"docs/notes.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches write target is outside the claude writer roots"
    }
  ]
}
```

### crush-guard-writes-003-symlink-escape-denied → claude-guard-writes-003-symlink-escape-denied

```json
{
  "given": [
    "the \"claude-guard-symlink-escape\" project tree",
    "the project tree's \"tests/node_modules/shared\" is a symbolic link to \"../../docs\"",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"Write\"",
    "claude names the file \"tests/node_modules/shared/README.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches write target is outside the claude writer roots"
    }
  ]
}
```

### crush-guard-writes-004-shared-prefix-denied → claude-guard-writes-004-shared-prefix-denied

```json
{
  "given": [
    "the \"claude-guard-shared-prefix\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"Write\"",
    "claude names the file \"tests-old/notes.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches write target is outside the claude writer roots"
    }
  ]
}
```

### crush-guard-writes-006-unnamed-path-denied → claude-guard-writes-006-unnamed-path-denied

```json
{
  "given": [
    "the \"claude-guard-unnamed-path\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"Write\""
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches write target is outside the claude writer roots"
    }
  ]
}
```

### engine-config-002-malformed-model-entries-refused → engine-config-002-malformed-model-entries-refused

```json
{
  "given": [
    "the \"build-tests-malformed-model-entries\" project tree"
  ],
  "when": [
    "the System Architect runs \"build tests\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches level=ERROR msg=\"Refusing to start\" command=\"build tests\"",
    {
      "and": "stdout matches engine\\.models: unknown model tier: \\\\\"medium\\\\\" \\(known: xhigh, high, coder, quick\\)"
    },
    {
      "and": "stdout matches engine\\.models\\.xhigh: model reference must be \\\\\"<cli>:<model>\\\\\": got \\\\\"claude-fable-5\\\\\""
    },
    {
      "and": "stdout matches engine\\.models\\.high: model reference has an empty cli: got \\\\\":claude-opus-5\\\\\""
    },
    {
      "and": "stdout matches engine\\.models\\.coder: model reference has an empty model: got \\\\\"codex:\\\\\""
    },
    {
      "but": "stdout does not match engine\\.models\\.quick"
    },
    {
      "and": "stdout does not match BUILD TESTS"
    },
    {
      "and": "no file outside \"tmp\" changed"
    },
    {
      "and": "the engine dispatched no AI turns"
    }
  ]
}
```

### build-code-run-006-shared-fix-skips-green-peer → build-code-run-006-shared-fix-skips-green-peer

```json
{
  "given": [
    "the \"build-code-shared-fix\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches BUILD CODE COMPLETE",
    "stdout matches msg=\"Loaded prompts\" command=\"build code\" items=1",
    "stdout matches msg=\"Check answered\" check=tests-pass.*answer=pass",
    {
      "and": "stdout matches msg=\"Suite failures\" failures=2 framework=go-test"
    },
    {
      "and": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 1 fix AI turn to \"claude\""
    },
    {
      "and": "the engine dispatched 1 apply AI turn to \"codex\""
    },
    {
      "and": "the engine dispatched 2 AI turns"
    },
    {
      "and": "the engine spawned exactly 3 test runners"
    },
    {
      "and": "the engine spawned a test runner with arguments matching -run \\^TestAAdd\\$\\|\\^TestBAdd\\$"
    },
    "every file matching \"tmp/*/apply-*-user.txt\" matches Whole suite:",
    "every file matching \"tmp/*/apply-*-user.txt\" matches Selection:",
    "every file matching \"tmp/*/apply-*-user.txt\" matches TestAAdd",
    "every file matching \"tmp/*/apply-*-user.txt\" matches TestBAdd",
    {
      "but": "the file \"tests/suite/calc_test.go\" is unchanged"
    },
    {
      "and": "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
    }
  ]
}
```

### build-code-run-007-next-walk-finds-regression → build-code-run-007-next-walk-finds-regression

```json
{
  "given": [
    "the \"build-code-cross-walk\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches BUILD CODE COMPLETE",
    {
      "and": "stdout matches msg=\"Suite failures\" failures=0 framework=go-test"
    },
    {
      "and": "stdout matches msg=test index=1 total=1 id=.*TestBCompatibility"
    },
    {
      "and": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 2 fix AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 2 apply AI turns to \"codex\""
    },
    {
      "and": "the engine dispatched 4 AI turns"
    },
    {
      "and": "the engine spawned exactly 5 test runners"
    },
    {
      "but": "the file \"tests/suite/calc_test.go\" is unchanged"
    },
    {
      "and": "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
    }
  ]
}
```

### build-code-frameworks-005-not-applied-revalidated → build-code-frameworks-005-not-applied-revalidated

```json
{
  "given": [
    "the \"build-code-not-applied\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches BUILD CODE COMPLETE",
    {
      "and": "stdout matches level=WARN msg=\"Fix was NOT applied\""
    },
    {
      "but": "stdout does not match msg=\"Walk stopped\""
    },
    {
      "and": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 2 fix AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 2 apply AI turns to \"codex\""
    },
    {
      "and": "the engine dispatched 4 AI turns"
    },
    {
      "and": "the engine spawned exactly 4 test runners"
    },
    {
      "and": "the file \"services/calc/prepared.txt\" matches ^prepared"
    },
    {
      "but": "the file \"tests/suite/calc_test.go\" is unchanged"
    },
    {
      "and": "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
    }
  ]
}
```

### build-code-frameworks-006-not-applied-hits-fix-bound → build-code-frameworks-006-not-applied-hits-fix-bound

```json
{
  "given": [
    "the \"build-code-not-applied-bound\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 1",
    {
      "and": "stdout matches level=WARN msg=\"Fix was NOT applied\""
    },
    {
      "and": "stdout matches level=ERROR msg=\"Walk stopped\".*gave up after 3 applied fix"
    },
    {
      "and": "the engine dispatched no validation AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 3 fix AI turns to \"claude\""
    },
    {
      "and": "the engine dispatched 3 apply AI turns to \"codex\""
    },
    {
      "and": "the engine dispatched 6 AI turns"
    },
    {
      "and": "the engine spawned exactly 4 test runners"
    },
    {
      "and": "the file \"services/calc/prepared-first.txt\" matches ^prepared"
    },
    {
      "and": "the file \"services/calc/prepared-second.txt\" matches ^prepared"
    },
    {
      "but": "the file \"tests/suite/calc_test.go\" is unchanged"
    }
  ]
}
```

### crush-guard-reads-001-outside-project-denied → claude-guard-reads-001-outside-project-denied

```json
{
  "given": [
    "the \"claude-guard-read-outside\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"Read\"",
    "claude names the path \"/true-bdd-neighbour/secrets.txt\" as written"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches (read target is outside the claude writer roots|search target is outside|glob)"
    }
  ]
}
```

### crush-guard-reads-002-relative-escape-denied → claude-guard-reads-002-relative-escape-denied

```json
{
  "given": [
    "the \"claude-guard-read-relative-escape\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude pipes the hook payload naming the tool \"Glob\" and the path \"../neighbour\""
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches (read target is outside the claude writer roots|search target is outside|glob)"
    }
  ]
}
```

### crush-guard-reads-003-symlink-escape-denied → claude-guard-reads-003-symlink-escape-denied

```json
{
  "given": [
    "the \"claude-guard-read-symlink-escape\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "the project tree's \"node_modules/shared\" is a symbolic link to \"../../\"",
    "claude names the tool \"Grep\"",
    "claude names the file \"node_modules/shared/notes.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches (read target is outside the claude writer roots|search target is outside|glob)"
    }
  ]
}
```

### crush-guard-reads-004-unnamed-path-allowed → claude-guard-reads-004-unnamed-path-allowed

```json
{
  "given": [
    "the \"claude-guard-read-unnamed-path\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"Read\""
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-guard-reads-005-missing-read-root-denied → claude-guard-reads-005-missing-read-root-denied

```json
{
  "given": [
    "the \"claude-guard-read-missing-root\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude hands the guard a write policy with no read root",
    "claude names the tool \"Read\"",
    "claude names the file \"docs/README.md\" of the project tree"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    {
      "and": "the guard's reason matches invalid claude guard"
    }
  ]
}
```

### crush-guard-reads-006-relative-path-allowed → claude-guard-reads-006-relative-path-allowed

```json
{
  "given": [
    "the \"claude-guard-read-relative-path\" project tree",
    "claude's write policy lets the turn write under \"tests\"",
    "claude names the tool \"Read\"",
    "claude names the path \"docs/README.md\" as written"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-replay-001-whole-command → claude-replay-001-whole-command

```json
{
  "given": [
    "the \"claude-replay-whole-command\" project tree",
    "claude's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "claude pipes a \"foreground\" shell call in directory \".\" with command go test ./..."
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-replay-002-selected-command → claude-replay-002-selected-command

```json
{
  "given": [
    "the \"claude-replay-selected-command\" project tree",
    "claude's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "claude pipes a \"foreground\" shell call in directory \".\" with command go test ./... -run '^TestSlow$'"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard allows the tool call"
  ]
}
```

### crush-replay-003-shell-operator → claude-replay-003-shell-operator

```json
{
  "given": [
    "the \"claude-replay-shell-operator\" project tree",
    "claude's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "claude pipes a \"foreground\" shell call in directory \".\" with command go test ./...; cat /etc/passwd"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-004-assignment → claude-replay-004-assignment

```json
{
  "given": [
    "the \"claude-replay-assignment\" project tree",
    "claude's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "claude pipes a \"foreground\" shell call in directory \".\" with command X=1 go test ./..."
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-005-another-program → claude-replay-005-another-program

```json
{
  "given": [
    "the \"claude-replay-another-program\" project tree",
    "claude's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "claude pipes a \"foreground\" shell call in directory \".\" with command cat /etc/passwd"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-006-extra-flag → claude-replay-006-extra-flag

```json
{
  "given": [
    "the \"claude-replay-extra-flag\" project tree",
    "claude's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "claude pipes a \"foreground\" shell call in directory \".\" with command go test ./... -v"
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-007-double-quotes → claude-replay-007-double-quotes

```json
{
  "given": [
    "the \"claude-replay-double-quotes\" project tree",
    "claude's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "claude pipes a \"foreground\" shell call in directory \".\" with command go test ./... -run \"^TestSlow$\""
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-008-wrong-directory → claude-replay-008-wrong-directory

```json
{
  "given": [
    "the \"claude-replay-wrong-directory\" project tree",
    "claude's write policy grants the test command \"go test ./...\" with selector \"-run\" in \".\"",
    "claude pipes a \"foreground\" shell call in directory \"tests\" with command go test ./..."
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-009-no-command → claude-replay-009-no-command

```json
{
  "given": [
    "the \"claude-replay-no-command\" project tree",
    "claude pipes a \"foreground\" shell call in directory \".\" with command go test ./..."
  ],
  "when": [
    "claude asks the guard about the tool call"
  ],
  "then": [
    "the guard denies the tool call",
    "the guard's reason matches allowed forms: .* OR .* '<selection>'"
  ]
}
```

### crush-replay-015-unrepresentable-command → build-code-claude-007-unrepresentable

```json
{
  "given": [
    "the \"build-code-unrepresentable-replay\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Replay command grant unavailable\"",
    "stdout matches argument contains a single quote, newline or NUL",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 1 match for Replay command grant unavailable",
    "every file matching \"tmp/*/*-claude-config/policy.json\" does not match \"test_command\""
  ]
}
```

### build-code-set-001-partial-repair → build-code-set-001-partial-repair

```json
{
  "given": [
    "the \"build-code-partial\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches BUILD CODE COMPLETE",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 2 fix AI turns to \"claude\"",
    "the engine dispatched 2 apply AI turns to \"codex\"",
    "the engine spawned exactly 4 test runners",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA",
    "the engine spawned a test runner with arguments matching -run \\^TestA\\$\\|\\^TestB\\$"
  ]
}
```

### build-code-set-002-shared-evidence → build-code-set-002-shared-evidence

```json
{
  "given": [
    "the \"build-code-grouped\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "the engine dispatched 1 fix AI turn to \"claude\"",
    "the engine dispatched 1 apply AI turn to \"codex\"",
    "the engine spawned exactly 3 test runners",
    "the file matching \"tmp/*/*fix-iter1-user.txt\" has exactly 1 match for SHARED-FAILURE-EVIDENCE",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches Tests: .*TestA, .*TestB",
    "every file matching \"tmp/*/apply-*-user.txt\" matches Whole suite:",
    "every file matching \"tmp/*/apply-*-user.txt\" matches Selection:"
  ]
}
```

### build-code-set-003-mixed-synthetic → build-code-set-003-mixed-synthetic

```json
{
  "given": [
    "the \"build-code-mixed-build\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches msg=\"Suite failures\" failures=2 framework=go-test",
    "the engine spawned exactly 3 test runners",
    "the engine dispatched 1 fix AI turn to \"claude\"",
    "the engine dispatched 1 apply AI turn to \"codex\"",
    "stdout does not match Spawning test runner.*-run",
    "the shell command \"go test -json -count=1 ./tests/suite/... ./tests/broken/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-set-004-missing-result → build-code-set-004-missing-result

```json
{
  "given": [
    "the \"build-code-missing-member\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "the engine dispatched 2 fix AI turns to \"claude\"",
    "the engine dispatched 2 apply AI turns to \"codex\"",
    "the engine spawned exactly 4 test runners",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches did not run",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestA"
  ]
}
```

### build-code-set-005-shrinking-counter → build-code-set-005-shrinking-counter

```json
{
  "given": [
    "the \"build-code-shrinking-bound\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches gave up after 2 applied fix",
    "the engine dispatched 2 apply AI turns to \"codex\"",
    "the engine spawned exactly 3 test runners",
    "the file \"services/calc/attempted.txt\" matches attempted",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA"
  ]
}
```

### build-code-confirmation-001-passes → build-code-confirmation-001-passes

```json
{
  "given": [
    "the \"build-code-no-confirmation-passes\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Writer confirmation missing\".*apply-failing-set-iter1-result.yaml",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 1 match for \"level\":\"WARN\",\"msg\":\"Writer confirmation missing\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 1 fix AI turn to \"claude\"",
    "the engine dispatched 1 apply AI turn to \"codex\"",
    "the engine spawned exactly 3 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches BUILD CODE COMPLETE",
    "stdout does not match msg=\"Walk stopped\"",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-confirmation-002-retries → build-code-confirmation-002-retries

```json
{
  "given": [
    "the \"build-code-no-confirmation-retries\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Writer confirmation missing\".*apply-failing-set-iter1-result.yaml",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 2 matches for \"level\":\"WARN\",\"msg\":\"Writer confirmation missing\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 2 fix AI turns to \"claude\"",
    "the engine dispatched 2 apply AI turns to \"codex\"",
    "the engine spawned exactly 4 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches BUILD CODE COMPLETE",
    "stdout does not match msg=\"Walk stopped\"",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-confirmation-003-bound → build-code-confirmation-003-bound

```json
{
  "given": [
    "the \"build-code-no-confirmation-bound\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches level=WARN msg=\"Writer confirmation missing\".*apply-failing-set-iter1-result.yaml",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 3 matches for \"level\":\"WARN\",\"msg\":\"Writer confirmation missing\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 3 fix AI turns to \"claude\"",
    "the engine dispatched 3 apply AI turns to \"codex\"",
    "the engine spawned exactly 4 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches level=ERROR msg=\"Walk stopped\".*gave up after 3 applied fix",
    "the file \"services/calc/prepared.txt\" matches ^prepared",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-writer-001-nonzero → build-code-writer-001-nonzero

```json
{
  "given": [
    "the \"build-code-writer-nonzero\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Writer turn failed\" outcome=\"non-zero exit\"",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 2 matches for \"level\":\"WARN\",\"msg\":\"Writer turn failed\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 2 fix AI turns to \"claude\"",
    "the engine dispatched 2 apply AI turns to \"codex\"",
    "the engine spawned exactly 4 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches closureGlossary",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches build failed",
    "stdout matches BUILD CODE COMPLETE",
    "stdout does not match msg=\"Walk stopped\"",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-writer-002-empty → build-code-writer-002-empty

```json
{
  "given": [
    "the \"build-code-writer-empty\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 0",
    "stdout matches level=WARN msg=\"Writer turn failed\" outcome=\"empty output\"",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 1 match for \"level\":\"WARN\",\"msg\":\"Writer turn failed\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 1 fix AI turn to \"claude\"",
    "the engine dispatched 1 apply AI turn to \"codex\"",
    "the engine spawned exactly 3 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches BUILD CODE COMPLETE",
    "stdout does not match msg=\"Walk stopped\"",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

### build-code-writer-003-bound → build-code-writer-003-bound

```json
{
  "given": [
    "the \"build-code-writer-bound\" project tree",
    "the System Architect answers \"1\" to every question"
  ],
  "when": [
    "the System Architect runs \"build code --fix\""
  ],
  "then": [
    "the command exits with code 1",
    "stdout matches level=WARN msg=\"Writer turn failed\" outcome=\"non-zero exit\"",
    "the file matching \"tmp/true-bdd.log.json\" has exactly 3 matches for \"level\":\"WARN\",\"msg\":\"Writer turn failed\"",
    "the engine dispatched no validation AI turns to \"claude\"",
    "the engine dispatched 3 fix AI turns to \"claude\"",
    "the engine dispatched 3 apply AI turns to \"codex\"",
    "the engine spawned exactly 4 test runners",
    "the file \"services/calc/calc.go\" is modified",
    "the file \"tests/suite/calc_test.go\" is unchanged",
    "stdout matches level=ERROR msg=\"Walk stopped\".*gave up after 3 applied fix",
    "the file \"services/calc/prepared.txt\" matches ^prepared",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" matches TestB",
    "every file matching \"tmp/*/*fix-iter1-user.txt\" does not match TestA",
    "the shell command \"go test -json -count=1 ./tests/suite/...\" run in \".\" exits with code 0"
  ]
}
```

The absolute-only write-path rule and a nameless write being denied regardless of cwd retire with the old protocol. Claude resolves relative or absent file targets against cwd, then enforces its roots. The replacement nameless-write Scenario pins the denial when cwd lies outside the write roots. Historical Q&A text remains decision evidence; #204 tooling references are excluded from this product change.
