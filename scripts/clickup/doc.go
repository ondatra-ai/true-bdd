// Package clickup files deferred findings as ClickUp tasks, through
// `claude -p` and MCP.
//
// There is no ClickUp REST token in this repo's `.env`, and adding one would
// be a second credential to hold for something the session can already do:
// `claude -p` inherits the configured MCP servers, so it can create the tasks
// itself. Two details make that work headlessly, both verified:
//
//   - the tool allowlist is mandatory — without `--allowedTools` a headless
//     run answers "permission not granted" and files nothing;
//   - `--allowedTools` must come BEFORE `-p`, or the prompt is read as a flag
//     argument and claude asks for input on stdin instead.
//
// Both now live in scripts/internal/claudecli, which is the only place either
// order is spelled out.
//
// The queue is rendered to a markdown file first, and that file is the
// artifact: it is what a person reads before anything is uploaded, and it is
// what the model is asked to transcribe rather than invent.
//
// The command is the single ClickUp interface for anything running OUTSIDE a
// Claude session, which has no MCP server to inherit — task-handle included,
// since it is a Go command now. The /task-* skills call MCP directly:
//
//	clickup render --queue tmp/merge/defer-queue.json --tag fix-now --pr 76
//	clickup file   --queue tmp/merge/defer-queue.json --tag fix-now --pr 76
//	clickup list   --tag fix-now
package clickup
