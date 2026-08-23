// Package claudecli runs one headless `claude -p` turn.
//
// Shared by the ClickUp interface and the merge loop because both depend on
// the same two argument-order facts, and one of them cost a hang to find:
//
//   - `--allowedTools` must come BEFORE `-p`. The other order makes claude
//     read the prompt as the flag's argument and then block forever on stdin.
//   - the tool allowlist is mandatory for anything touching MCP. Without it a
//     headless run answers "permission not granted" and does nothing.
//
// The environment is handled differently from the plain subprocess helpers on
// purpose: CLAUDECODE is REMOVED here rather than blanked, because a nested
// headless run has to look like it was not launched from inside a session.
package claudecli
