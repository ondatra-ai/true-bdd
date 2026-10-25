# Bound one crush writer response and let the tests judge failed applies

**Superseded by [ADR-0023](0023-remove-crush-from-product.md).** Historical rationale follows; current writer recovery and confinement follow ADR-0021, ADR-0022 and ADR-0023. [ADR-0025](0025-unbounded-ai-turns.md) removes the router deadline for every role and CLI, preserving caller cancellation.

Only a `build code --fix` crush apply watches its own generated data directory's `logs/crush.log`. A `ModelProvider called` JSON record opens a model response. The next record of any kind closes it; `Hook completed` for `PreToolUse` opens tool execution. Only an open model response is timed: at ten minutes the engine stops the process group and logs one WARN carrying the turn label, elapsed minutes and log path. Without a model record nothing is timed. Tool execution, including replay and `job_output`, is never timed by this watch.

This is a deliberate exception to #181's “every AI turn works unbounded”. The Owner chose “Guard + next attempt”; the option as offered named a per-response `max_tokens` cap. Claude then chose a ten-minute log watch from the measured response durations, and the plan-read required state tracking to exclude tool execution. It bounds one model response that streams without completing, not the turn, which may run as long as its responses keep ending. No prompt, checklist or template states the bound. The build-code applier requests it explicitly through turn options and the router's Request.WatchWriterResponse field; every other turn is unwatched. At the time, other Claude turns kept a 20-minute bound and ADR-0022 exempted only its build-code writer. ADR-0025 removes that bound for every turn.

## Evidence and alternatives

The second verification on `d0a5cdff` degenerated at 438,126 prompt tokens and 239 messages: gibberish and unfinished empty bash calls streamed for over 22 minutes, while the crush log stopped after `ModelProvider called`. Larger sessions at 604,905 and 556,999 tokens did not degenerate, so a context-size cutoff would not identify the failure. Across 594 completed responses the three sessions' medians were 11/12/12 seconds, 95th percentiles 57/81/63 seconds, and maxima 135/161/112 seconds. Ten minutes is more than three times the longest observed response.

A whole-turn bound would stop legitimate multi-stage work. A log-silence bound without state would stop long test runs and tool waits. A per-response `max_tokens` cap could cut a legitimate file write and would not prevent degeneration in the next response. The state-aware watch uses the installed crush v0.97.1 JSON log, verified at its generated `options.data_directory/logs/crush.log`, without changing argv, stdin or configuration.

## Consequences

A started crush writer stopped by the watch, exiting non-zero or answering empty/whitespace-only reports a distinct outcome. Only build code's apply treats it as completed: files remain, PostNotApplied reruns the remaining failing set, and the tests decide. The watch warns once for a stall; the runner warns once for the other failed outcomes. The next plan receives the rerun's remaining set and rebuilt grouped evidence, including a package build failure if the writer left incomplete code. The existing attempt counter includes every such apply and the terminal attempt; a passing terminal rerun still cannot bypass the non-converging bound.

A process that cannot start or an engine refusal before spawning stays fatal. Other commands and roles retain their existing behavior. ADR-0021 and ADR-0022 extend started-writer recovery to the codex and Claude build-code appliers. Provider argv, stdin composition and shipped prompt templates are unchanged; existing recordings remain byte-identical.

The guard and recovery were chosen by the Owner in #196 after #181's iteration 2 verification; the ten-minute mechanism was chosen by Claude and revised by the plan-read. No grilling entry records this exception; the parent drill decision is recorded in `docs/qna/20261022-201524-rebuild-from-spec-drill.yaml`, Q5.
