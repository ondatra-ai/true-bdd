// Package history captures the conversation into docs/history/.
//
// Entry points:
//
//	prompt-submit  — wired to BOTH the UserPromptSubmit and Stop hooks with
//	                 the same command.
//	new-task       — invoked from /task-start, via `history roll`.
//	bind <id>      — invoked from /task-start, once a Ticket is chosen.
//	bound          — invoked from /task-done and /task-fail; prints the id.
//	unbind         — invoked from all three, after the status write lands.
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
// tail hasn't been flushed yet. A per-session cursor records how many blocks
// of the current turn are already logged, so a blocking Stop hook that forces
// the turn to continue doesn't make the next Stop re-append the whole turn —
// only the continuation's new blocks land.
//
// State: scripts/state, one append-only docs/history/state.jsonl.
//
//	This package owns no state file. It reads `task` for the stem it appends
//	to — shared across sessions, so a new session continues the same file —
//	and `cursor:<session8>` for its own turn progress. `ticket` and `mandate`
//	live in the same file and belong to the /task-* skills.
//
// History file: docs/history/<task>.md, derived from the stem rather than
// stored, and opened O_APPEND|O_CREATE by whichever writer arrives first.
//
// Off switch: CLAUDE_HISTORY_ROLE=0 skips all logging.
//
// Rollover: /task-start removes the state file so the next prompt opens a
// fresh Task. Its own UserPromptSubmit (prompt == "/task-start") is filtered
// so it doesn't recreate the state it just deleted. One file holds all four
// keys, so `new-task` clears the Ticket binding and the mandate with them.
//
// One thing changed in the port. The Python found the repository root from
// its own `__file__` when CLAUDE_PROJECT_DIR was unset; a `go run` binary
// lives in a temporary directory, so that fallback cannot survive. The shim
// exports CLAUDE_PROJECT_DIR from its own location, and RepoRoot falls back
// to `git rev-parse --show-toplevel` — which answers the same question the
// `__file__` walk was asking.
package history
