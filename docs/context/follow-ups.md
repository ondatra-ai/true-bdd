# Follow-ups — conversational ledger

Work explicitly deferred or requested but not done in its task, extracted from
task transcripts by the context archivist (`.claude/hooks/context.py`).
Append-only; newest at the bottom. Strike through or remove items once done.

## 2026-07-21 — The task added and merged repository workflow/history hygiene changes, deliberately kept BDD fixtures outside the commit gate, and ended with an unanswered request about the original new-task workflow.

_transcript: 20260721-170249-ae17ad87-test.md_

- Locate the `new-task` skill or command in `/Users/peterovchinnikov/work/awesome/awesome-claude-mcp/` and report where it lives; the transcript ends before this request is addressed (2026-07-21T19:03:29Z).

## 2026-07-21 — The task investigated BDD failures and Opus model selection, then reverted the unanalysed changes while retaining the restored `bdd-cli/` directory.

_transcript: 20260721-190544-9858af28-let-s-start-from-fixing-bdd-tests-let-s.md_

- ~~Complete the requested BDD test run with Opus and analyze whether different workflow states need different models; the attempted run was stopped when the speculative changes were reverted, so this remained unfinished (requested 2026-07-21T19:05:44Z; incomplete as of 2026-07-21T19:36:10Z).~~ _(superseded 2026-07-21)_

## 2026-07-21 — This task added configurable model selection, validated Opus with the BDD fixtures, fixed fixture write permissions, and committed and pushed the changes.

_transcript: 20260721-195954-481d7431-now-let-s-make-possible-to-set-model-thr.md_

- Analyze whether different workflow states need different models; the requested Opus-configured BDD run was completed, but no comparative per-state model analysis was performed (run completion verified 2026-07-21T22:21:16Z; still unaddressed at task end 2026-07-22T06:55:12Z).

## 2026-07-25 — The task committed context-ledger updates and opened a pull request, but the merge remained pending on the required CI gate.

_transcript: 20260725-082150-3c6ab062-mmit-and-merge.md_

- ~~Complete Peter's requested merge of the context-ledger changes; the transcript ended while the required `gates` check was still pending, before the merge completed (requested 2026-07-25T08:21:50Z; still pending 2026-07-25T08:23:11Z).~~ _(superseded 2026-07-25)_
- The previously pending merge of the context-ledger changes completed after the required `gates` check passed, so no merge follow-up remains (verified 2026-07-25T08:23:34Z).

## 2026-07-25 — This task began the requested, research-led rename of `bdd-cli` to `true-bdd`, with Codex critiques required for both the plan and implementation.

_transcript: 20260725-111029-40e36181-research-me-where-we-use-true-bdd-and-wh.md_

- ~~Complete the requested rename from `bdd-cli` to `true-bdd`; this transcript chunk ends before implementation or the required post-implementation review is recorded (requested 2026-07-25T11:18:13Z; outstanding as of 2026-07-25T11:31:56Z).~~ _(superseded 2026-07-25)_
- The requested rename from `bdd-cli` to `true-bdd`, including the required post-implementation Codex review and resulting fixes, was completed, so no rename follow-up remains (verified 2026-07-25T11:51:17Z).

## 2026-07-25 — The task began designing broader BDD coverage for every checklist branch by comparing independent Claude and Codex proposals before implementation.

_transcript: 20260725-120421-f9fa25d3-task-is-to-write-more-bdd-tests-the-idea.md_

- ~~Complete the Claude-versus-Codex design comparison and implement the expanded BDD branch-coverage suite; this transcript chunk ends with both proposals still running and no comparison or implementation recorded (requested 2026-07-25T12:04:21Z; outstanding as of 2026-07-25T12:07:07Z).~~ _(superseded 2026-07-25)_
- ~~Implement the expanded BDD branch-coverage suite; the independent Claude and Codex proposals were completed and compared, but this transcript chunk ends after the architecture recommendation and before implementation is recorded (comparison verified and work still outstanding 2026-07-25T12:25:56Z).~~ _(superseded 2026-07-25)_
- ~~Complete the renewed Claude-versus-Codex design comparison, implement code-coverage-like checklist-branch measurement, and then use uncovered branches to drive incremental fixture additions; this transcript chunk ends with both new proposals still running, before synthesis or implementation (requested 2026-07-25T12:45:33Z; outstanding as of 2026-07-25T12:47:08Z).~~ _(superseded 2026-07-25)_
- ~~Complete and reconcile the independent Claude/Codex source-level branch inventories, then continue with measurement design and the already-requested incremental fixture work; this transcript chunk ends while the verification phase remains unfinished (requested 2026-07-25T12:53:45Z; outstanding as of 2026-07-25T12:55:43Z).~~ _(superseded 2026-07-25)_
- ~~Finish the still-running Codex source-level branch inventory, reconcile it with the completed Claude inventory, then continue with coverage-measurement design and incremental fixture additions; the Claude inventory completed while Codex and the downstream work remained outstanding (verified 2026-07-25T13:09:15Z and 2026-07-25T13:09:39Z).~~ _(superseded 2026-07-25)_
- ~~Finish Phase 2 by comparing and synthesizing the independent coverage-measurement designs, then implement the coverage report and use its uncovered targets to drive incremental fixture additions; the source-level inventories are now complete and reconciled, while measurement design and implementation remain outstanding (verified 2026-07-25T13:32:16Z).~~ _(superseded 2026-07-25)_
- ~~Implement the checklist-branch coverage report and use its uncovered targets to drive incremental BDD fixture additions; the independent Phase-2 coverage-measurement designs were completed, compared, and synthesized, but implementation is not recorded in this transcript chunk (verified 2026-07-25T13:59:42Z).~~ _(superseded 2026-07-25)_
- ~~Complete the newly requested bootstrap collector and coverage baseline, including the unfinished pre-implementation critique and the post-results Codex verification and relevant-gap fixes; this transcript chunk records no completed implementation or results review (requested 2026-07-25T14:46:26Z; outstanding as of 2026-07-25T14:51:00Z).~~ _(superseded 2026-07-25)_
- Use the implemented checklist-branch coverage report's uncovered targets to drive incremental BDD fixture additions; this chunk records the collector, report, and baseline implementation but no fixture additions (implementation recorded and fixture work still outstanding 2026-07-25T15:34:39Z).
- ~~Finish the in-progress Codex verification of the bootstrap collector and coverage baseline, triage its findings, and apply only fixes relevant to the bootstrap goal; implementation and the pre-implementation critique completed, but the final audit had not finished (critique completion verified 2026-07-25T15:09:06Z; still outstanding 2026-07-25T15:34:39Z).~~ _(superseded 2026-07-25)_
- The Codex verification of the bootstrap collector and coverage baseline completed, its relevant findings were fixed and verified, so no final-audit follow-up remains (verified 2026-07-25T16:05:05Z).

## 2026-07-25 — The task was implementing expanded BDD fixtures, with a later model experiment deferred until that work finishes.

_transcript: 20260725-185231-8244ea0b-now-i-want-you-to-implement-bdd-tests-to.md_

- ~~After the current BDD fixture implementation finishes, try running the tests with Sonnet (requested 2026-07-25T19:21:44Z; not yet addressed in this transcript chunk).~~ _(superseded 2026-07-25)_
- ~~Finish the in-progress Sonnet BDD comparison and deliver the promised full comparison table; partial results were available, but the remaining fixtures were still running (originally requested 2026-07-25T19:21:44Z; outstanding 2026-07-25T20:53:01Z).~~ _(superseded 2026-07-25)_
- ~~Peter canceled the in-progress Sonnet BDD comparison and requested reverting its use, so the promised full Sonnet comparison table should not be resumed (requested 2026-07-25T21:00:49Z; revert confirmed 2026-07-25T21:02:25Z).~~ _(superseded 2026-07-25)_
- ~~Finish verifying the single-step checklist-override fixtures in isolation and in the full coverage union, then update the coverage baseline; the transcript ends while fixture execution is still in progress (outstanding 2026-07-25T22:04:25Z).~~ _(superseded 2026-07-25)_
- ~~Complete the still-running final Codex verification of the single-step checklist-override implementation and results, triage any goal-relevant findings, and apply relevant fixes; fixture isolation, full-union coverage verification, and the baseline update are complete, but the final verification pass had not finished (verified 2026-07-25T22:29:18Z).~~ _(superseded 2026-07-25)_
- The final Codex verification of the single-step checklist-override implementation completed and all reported findings were fixed and verified, so no final-verification follow-up remains (verified 2026-07-25T22:43:32Z).
- ~~Complete the requested fixture-time checklist generation and staged Codex process; this transcript chunk ends with the initial Codex plan critique still pending, before implementation or the post-implementation review (requested 2026-07-26T06:47:47Z; outstanding as of 2026-07-26T06:49:20Z).~~ _(superseded 2026-07-25)_
- The requested fixture-time checklist generation and staged Codex critique/review process completed, including verification and fixes for all goal-relevant review findings, so no follow-up remains (verified 2026-07-26T08:01:33Z).
- ~~Complete Peter's renewed request to measure how long the tests take when run with Sonnet; the transcript ends after the run was started and before timing results were reported (requested 2026-07-26T11:09:19Z; outstanding as of 2026-07-26T11:10:45Z).~~ _(superseded 2026-07-25)_
- Peter's renewed request to measure Sonnet fixture runtime was completed and reported, and the engine was reverted to Opus, so no timing-run follow-up remains (verified 2026-07-26T11:49:43Z).
- ~~Complete Peter's requested merge of PR #20; the commit and push completed, but the merge remained blocked pending CodeRabbit re-review when the transcript ended (requested 2026-07-27T19:29:48Z; outstanding 2026-07-27T19:34:23Z).~~ _(superseded 2026-07-25)_
- Peter's requested merge of PR #20 completed successfully, so no merge follow-up remains (verified 2026-07-27T19:35:32Z).
- ~~Clarify which documents should remain in the configured document set; Peter said to keep only a specified set, but the message ended before the list, so no removals should be inferred yet (requested 2026-07-27T20:16:09Z; unresolved as of 2026-07-27T20:16:16Z).~~ _(superseded 2026-07-25)_
- ~~Finish the clarified host-document restructuring: the retained set is `docs/architecture/architecture.yaml`, `docs/prd/epics/*.yaml`, `docs/prd/stories/*.yaml`, `docs/prd/prd.yaml`, and the scenario YAML path Peter supplied; live BDD validation was still running, with coverage-baseline regeneration, full gates, and committing the remaining migration still outstanding (clarified 2026-07-27T20:23:15Z; outstanding 2026-07-27T21:33:12Z).~~ _(superseded 2026-07-25)_
- The clarified host-document restructuring, live BDD validation, coverage-baseline regeneration, full gates, and remaining commits were completed, so no restructuring follow-up remains (verified 2026-07-27T21:58:31Z).
- ~~Finish the affected BDD fixture validation, rescan coverage, and regenerate the coverage baseline after removing `bdd_guidelines`; the rerun was still underway when this transcript chunk ended (requested 2026-07-28T06:41:47Z; outstanding 2026-07-28T06:43:49Z).~~ _(superseded 2026-07-25)_
- The affected BDD fixture validation, coverage rescan, and coverage-baseline regeneration after removing `bdd_guidelines` were completed, so no follow-up remains (verified 2026-07-28T07:03:00Z).

## 2026-07-28 — The task removed the Historian MCP entry and then requested a Playwright inspection of the latest HTML relationship visualization.

_transcript: 20260728-102315-7b460a15-ok-let-s-try-to-visualyse-relatiotiship.md_

- ~~Open the most recently created HTML relationship visualization in Playwright and inspect how it looks; the transcript ends before this request is addressed (requested 2026-07-28T15:50:03Z).~~ _(superseded 2026-07-28)_
- The requested Playwright inspection of the latest HTML relationship visualization was completed and reported, so no inspection follow-up remains (verified 2026-07-28T15:51:29Z).

## 2026-07-28 — The task scoped and began a test-first, Codex-reviewed Next.js web harness for inspecting TrueBDD projects and invoking existing CLI workflows.

_transcript: 20260728-193056-431e4f8a-next-task-would-be-to-create-web-interfa.md_

- ~~Continue the staged process from the in-flight first Codex risk-analysis round and complete the requested harness; this transcript chunk records no completed plan, Playwright tests, implementation, or final review (outstanding 2026-07-28T20:12:38Z).~~ _(superseded 2026-07-28)_
- ~~Complete the requested web harness from the now-running Go-side phase: finish the full inventory scanner, event channel, remote hardening, server and UI surfaces, AI pass, and final Codex critique/fix rounds; the spike was complete, but these later phases remained unfinished (outstanding 2026-07-29T00:33:23Z).~~ _(superseded 2026-07-28)_
- ~~Complete the requested web harness from the in-progress UI phase: finish the UI, run the full real-Claude A0–A9 AI pass serially, complete repository/CI hygiene, and perform the final Codex critique and relevant-fix rounds; the server phase was verified complete, while these later phases remained unfinished (server completion verified 2026-07-29T02:16:09Z; remaining work stated 2026-07-29T02:17:03Z).~~ _(superseded 2026-07-28)_
- ~~Complete the requested web harness from the in-progress real-Claude AI phase: finish the serial A0–A9 pass, complete repository/CI hygiene, and perform the final Codex critique and relevant-fix rounds; the UI phase was completed and verified before the AI pass began (verified 2026-07-29T02:34:48Z).~~ _(superseded 2026-07-28)_
- ~~Complete the in-flight final Codex critique and apply any goal-relevant fixes through the requested review rounds; the serial real-Claude A0–A9 pass and repository/CI hygiene were complete before that review began (verified 2026-07-29T03:27:57Z).~~ _(superseded 2026-07-28)_
- ~~For the web-harness task, wrap up as soon as the in-progress tests finish and do not run Codex paths 2 or 3 (requested 2026-07-29T05:13:34Z).~~ _(superseded 2026-07-28)_
- The web-harness task was wrapped up after the remaining test matrix passed, and Codex solution-review paths 2 and 3 were not run as requested, so no harness follow-up remains (verified 2026-07-29T05:14:17Z).
- ~~Complete the requested harness UI rework into an expandable epic/story hierarchy with popup story review; this transcript chunk ends with only a plan and the first Codex review in progress, before tests or implementation were completed (requested 2026-07-29T05:54:37Z; outstanding 2026-07-29T05:59:02Z).~~ _(superseded 2026-07-28)_
- ~~Complete the harness UI rework into an expandable epic/story hierarchy with popup story review, including the newly requested manual Playwright MCP scenario smoke pass and visual-smoothness assessment; this transcript chunk records the request but no execution (requested 2026-07-29T08:07:14Z).~~ _(superseded 2026-07-28)_
- ~~Finish the in-progress final Codex review of the harness UI rework and apply any must-fix, goal-relevant findings; the expandable epic/story hierarchy, story popup, full test matrix, manual Playwright scenario smoke pass, and visual-smoothness assessment were completed before the review began (verified 2026-07-29T08:38:34Z).~~ _(superseded 2026-07-28)_
- The final Codex review of the harness UI rework completed, all four must-fix and both polish findings were fixed, and the full verification matrix passed, so no final-review follow-up remains (verified 2026-07-29T09:58:07Z).
- ~~Complete the CLI-owned-state v2 harness rebuild: finish the in-progress implementation, then perform the queued manual Playwright MCP smoke and visual pass and the final Codex review with relevant fixes; Peter explicitly requested completion, and the transcript ended before these stages finished (requested 2026-07-29T12:54:56Z; outstanding 2026-07-29T12:56:43Z).~~ _(superseded 2026-07-28)_
- ~~Complete the in-flight final Codex review of the CLI-owned-state v2 harness and apply any must-fix, goal-relevant findings; implementation and the manual Playwright MCP smoke and visual pass were complete, while the review was still running at the end of this chunk (verified 2026-07-29T14:42:08Z).~~ _(superseded 2026-07-28)_
- ~~Complete the in-progress Opus fix round for the CLI-owned-state v2 harness and apply all seven must-fix Codex findings; the completed review reproduced issues including cross-session reply injection, queued mutations surviving re-registration, duplicate answer delivery, a four-reader status deadlock, control-event loss, and a reconciliation race, but no completed fixes were recorded in this chunk (review completion and fix launch reported 2026-07-29T15:12:20Z).~~ _(superseded 2026-07-28)_
- ~~Complete the whole-module lint cleanup for the CLI-owned-state v2 harness and rerun the full CI-equivalent gates; all seven must-fix Codex findings were independently verified as functionally closed, but new v2 production and test files still failed CI’s full `golangci-lint` invocation when this chunk ended (verified and outstanding 2026-07-29T16:29:25Z).~~ _(superseded 2026-07-28)_
- Complete the whole-module lint cleanup for the CLI-owned-state v2 harness, rerun the full CI-equivalent gates, and commit the work once all agents have finished (cleanup outstanding 2026-07-29T16:29:25Z; commit requested 2026-07-29T17:02:24Z).

## 2026-07-29 — The task corrected the deployment source and explored Git-backed Vercel deployment for the local harness, but stopped pending Vercel authentication.

_transcript: 20260729-172938-d9329923-do-you-have-vercel-connection-if-yes-cou.md_

- ~~Complete the requested Vercel deployment of the local harness and configure repository-backed automatic deployment after Vercel CLI authentication or a scoped token is provided; no deployment was completed in this transcript chunk (requested 2026-07-29T17:49:12Z; outstanding 2026-07-29T18:01:18Z).~~ _(superseded 2026-07-29)_
- ~~After Peter chooses and validates either the temporary `npx` cache path or the global Homebrew path and explicitly says “go,” perform only that Vercel CLI installation; no installation occurred in this chunk (requested 2026-07-29T19:16:57Z; outstanding 2026-07-29T19:17:46Z).~~ _(superseded 2026-07-29)_
- Configure the originally requested repository-backed automatic deployment for `harness/`; the authenticated npx production deployment is complete, but no GitHub-to-Vercel link was configured and the verified Node 26 versus Vercel 24 mismatch must be resolved for Git builds to succeed (deployment verified and work still outstanding 2026-07-29T19:44:14Z).
- The standalone Vercel CLI installation is no longer outstanding: Peter selected the already-authenticated `npx vercel` path and deployment completed through it without a global Homebrew installation (chosen 2026-07-29T19:38:03Z; verified 2026-07-29T19:44:14Z).
- Investigate available internet guidance and fix the harness RPC origin/host policy so the remote agent can connect to the Vercel deployment; the request was not addressed in this transcript chunk (requested 2026-07-29T20:09:36Z).
- ~~Complete Peter's requested parallel Codex cross-check of the accidental-tracking and `.gitignore` audit and reconcile it with the completed local findings; the Codex process had produced no result when this transcript chunk ended (requested 2026-07-30T10:10:12Z; outstanding 2026-07-30T10:16:18Z).~~ _(superseded 2026-07-29)_
- The requested parallel Codex cross-check of the accidental-tracking and `.gitignore` audit completed and was reconciled with the local findings, so no cross-check follow-up remains (verified 2026-07-30T11:18:15Z).
