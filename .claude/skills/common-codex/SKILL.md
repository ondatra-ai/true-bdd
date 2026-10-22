---
name: common-codex
description: Hand code changes, fixes, plan readings, reviews and focused research to Codex through the Go runner, then weigh its report.
---

# Codex runs

Codex takes the code work and readings Claude would otherwise spend its subscription on, and reads Claude's drafts as a second model. This session writes the brief, starts the command, waits and reads the summary itself. No Claude sub-agent stands between Claude and Codex.

## What goes to Codex

- **code**: a ticket or described change, through green checks and a local commit, in a new worktree on its own branch. `mpc_implement-spec`'s implementers are code runs.
- **fix**: accepted review findings, failing checks or a merge conflict on a code run's branch, in that run's worktree.
- **plan-read**: a spec draft before publication, a diagnosis before its fix, or a delegated decision whose mistake would be costly.
- **review**: a diff along one axis. `mpc_code-review`'s standards and spec reviewers are two review runs, started together.
- **research**: one precise question about the code or public sources; use `--search` when it needs the web.

Readers (plan-read, review, research) run read-only without network. Writers (code, fix) have full access at high reasoning effort in worktrees the command owns. There is no complexity exception: Claude takes a code task itself only after two failed attempts. An edit or question Claude settles in two or three calls on files already held stays with Claude; a handover costs more.

Claude keeps the conversation with the Owner, decisions, specs and tickets, briefs, reading reports and diffs, the Owner's texts, and research through the Owner's connectors, which Codex lacks. Findings are evidence to check: sort each as valid, invalid or needs a test, and explain which you took and why.

## Brief and start

Copy the relevant template to `tmp/codex-briefs/<run>.md`: [code](briefs/code.md), [fix](briefs/fix.md), [plan-read](briefs/plan-read.md), [review](briefs/review.md), or [research](briefs/research.md). Fill every `{{…}}` placeholder. Name documents by path, issue number or command instead of pasting them; give a criterion of done and the report's shape.

From the repository root, start the command as a background Bash task with `run_in_background: true` and `timeout: 7200000`, so the session's task list wakes you at its end:

```bash
go run ./scripts/cmd/codex start --kind <kind> --name <run> \
  --brief tmp/codex-briefs/<run>.md [--search] [--issue <n>]...
```

For **code**, add `--branch impl/<n>-<slug> --base <branch-or-commit>`. The new checkout is `tmp/codex-worktrees/<run>/`, containing the whole repository. For **fix**, add `--branch <code-run-branch>` and no `--base`: the command requires a code run's record of that branch and checkout, and fixes must add a commit beyond the tip at which they start. Writers never run in Claude's checkout or an unrelated worktree. At most four writers run at once; a worktree is held by one running command.

`--name` is new, 1–64 lowercase letters, digits, dots, underscores or dashes, starting with a letter or digit. Repeat `--issue` for every issue the brief names. Before starting the agent, the command saves issue bodies, labels and comments under the run folder and appends their paths to the brief. Readers cannot call `gh` themselves.

The command checks sign-in and reports weekly usage before every attempt. It finds the newest weekly token-count event in `$CODEX_HOME/sessions/` (`~/.codex/sessions/` by default), by event timestamp, with a future reset and `window_minutes: 10080`. No usable event means `unknown`. Past 90% it warns and still starts. Before starting a batch past 90%, tell the Owner. Tell the Owner the log path from the command's first record.

## Wait, read, retry

Wait for the task notification; runs started together are waited for together. Read the single closing summary: status, reason, agent, weekly limit and UTC reset, Codex availability when relevant, attempt, exit code, session id, writer branch and worktree, current log, and final message.

- `done`: weigh the report. For a writer, also read `git diff <base>...<branch>` and its check evidence; Git completion alone cannot prove it meets the brief.
- `incomplete`: an agent left a report, but Git could not prove a clean checkout on the recorded branch, retaining the starting commit with at least one new commit. Treat it as failure.
- `failed`, `stalled` (no log growth for 12 minutes), `timed-out` (115 minutes total across Codex and takeover): read the reason and log tail.
- `unavailable` for a reader: Claude does the reading itself and tells the Owner. Do not resume it or hand it to crush, which has no enforced read-only mode.

After any other result than `done` or a reader's `unavailable`, resume once as the same kind of background task:

```bash
go run ./scripts/cmd/codex resume --name <run> --note '<what went wrong>'
```

Resume names the recorded session id, never the last session, and keeps workspace, sandbox, search and original commit baseline. With no session id it starts fresh using the saved brief and note, fetching issues preparation did not save. A refusal consumes no attempt: exit 2 means invalid input, an unknown/done/exhausted run, a live command, four busy writers, a held worktree, or a branch moved from its recorded checkout. A dead command recorded as running may be retried. Exit 0 means `done`; exit 1 means an attempted run ended otherwise. After the second failure Claude takes the work and tells the Owner, naming both logs.

When Codex is signed out, its sign-in check is missing, broken or stalled, its access token expires, or it hits its usage limit, crush takes code and fix work in the same workspace and attempt. Other execution failures use the normal retry path. The takeover brief says to read `git status` and the diff before continuing partial work. The command invokes `CRUSH_CLIENT_SERVER=false crush --cwd <worktree> run --quiet --reasoning-effort high`, keeping crush's configured model; `run` auto-approves, and no `--yolo` flag is used. A crush retry stays on crush in a fresh session with the whole brief and retry note.

## Files and shutdown

`tmp/codex/<run>/` retains `brief.md` with the command's `## This run`, `issues/<n>.md`, `run.json`, `run.log` and `final.md`. Attempt two keeps `resume.md`, `run-2.log` and `final-2.md`. Takeover also keeps `crush.md` or `crush-2.md`; both agents' streams remain in that attempt's log, and crush's stdout becomes the report. Records are published atomically under an allocation hold.

Every agent and preparation subprocess has its own process group. Cancellation sends SIGTERM, waits up to 10 seconds, then sends SIGKILL; descendants are stopped even after a successful agent exit. Output draining is bounded, including when an escaped descendant retains a pipe. `CODEX_RUN_STALL_MINUTES`, `CODEX_RUN_LIMIT_MINUTES` and `CODEX_RUN_GRACE_SECONDS` can shorten the windows for testing. Offline tests use fake CLIs: `go test ./scripts/codexrun ./pkg/cli/codex ./pkg/shell`.

## Implementing a spec

Keep `mpc_implement-spec`'s flow: one code run per ticket on the frontier, cut from the integration branch. Claude merges a completed branch with `git merge` when authorized. On conflict, abort and start a fix run on the ticket branch to merge the integration branch in, resolve and re-check; then merge again. Resolve conflicts on ticket branches. The closing review is two review runs. Once finished and authorized, remove worktrees with `git worktree remove tmp/codex-worktrees/<run>`.
