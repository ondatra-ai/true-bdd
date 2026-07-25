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
