// Package history captures the conversation into docs/history/.
//
// Two entry points:
//
//	prompt-submit  — wired to BOTH the UserPromptSubmit and Stop hooks with
//	                 the same command.
//	new-task       — invoked from .claude/commands/new-task.sh.
//
// UserPromptSubmit (payload has a `prompt`): append it under a heading keyed
// on the writer's role. The main interactive session (default role "claude")
// logs a real human turn as "## user"; a headless worker
// (CLAUDE_HISTORY_ROLE=branch-name|commit-msg|pr-content ...) is claude
// prompting that worker via `claude -p`, so it logs as "## claude to @<role>".
//
// Stop (no prompt): append the whole assistant turn — every text block since
// the last user prompt, read from the transcript — under the writer's role
// heading (CLAUDE_HISTORY_ROLE, default "claude"). The payload's
// `last_assistant_message` backstops the final block in case the transcript's
// tail hasn't been flushed yet. A per-session cursor
// (tmp/history-cursor/<session8>.json, keyed by prompt_id) records how many
// blocks of the current turn are already logged, so a blocking Stop hook that
// forces the turn to continue doesn't make the next Stop re-append the whole
// turn — only the continuation's new blocks land.
//
// State file: docs/history/hook-state
//
//	A single line: the current task file's name. Nothing else.
//	Shared across sessions — a new session continues the same file.
//
// History file: docs/history/<UTC-ts>-<session8>-<slug>.md
//
// Off switch: CLAUDE_HISTORY_ROLE=0 skips all logging.
//
// Rollover: /new-task removes the state file so the next prompt opens a fresh
// task file. Its own UserPromptSubmit (prompt == "/new-task") is filtered so
// it doesn't recreate the state file it just deleted.
//
// One thing changed in the port. The Python found the repository root from
// its own `__file__` when CLAUDE_PROJECT_DIR was unset; a `go run` binary
// lives in a temporary directory, so that fallback cannot survive. The shim
// exports CLAUDE_PROJECT_DIR from its own location, and RepoRoot falls back
// to `git rev-parse --show-toplevel` — which answers the same question the
// `__file__` walk was asking.
package history
