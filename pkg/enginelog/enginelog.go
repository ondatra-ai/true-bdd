// Package enginelog is the wire contract between the engine's log records and
// the reader that folds them back into turns.
//
// The engine emits these messages and keys through pkg/logging; the BDD
// report reads them out of true-bdd.log.json. The two live in different roots
// and cannot import each other, so before this package existed the reporter
// duplicated the strings privately: renaming one broke the report silently,
// with no gate going red. Referring to a constant from both sides makes the
// compiler the enforcer instead.
//
// Adding a message or a key is free. Renaming one is a breaking change to a
// consumer, which is the point.
package enginelog

// The messages the report folds a run into turns on. Everything else in the
// log is DEBUG noise from the provider's message stream.
const (
	MsgDispatch  = "Dispatching AI turn"
	MsgAssistant = "AssistantMessage received"
	MsgResult    = "ResultMessage received"
	// MsgToolUse carries a tool-use block as {name, input} on the content
	// key. It is the only record of what a turn actually DID.
	MsgToolUse = "ToolUseBlock details"
	MsgUsage   = "AI turn usage"
	// MsgAnswerUnusable marks a model answer the engine refused to grade.
	MsgAnswerUnusable = "Model answer unusable"
	MsgReturned       = "AI turn returned"
	MsgFailed         = "AI turn failed"
	// MsgTranscriptSaved is crush's and codex's only result boundary: they
	// stream nothing, so the archived transcript is the first evidence the
	// turn produced anything.
	MsgTranscriptSaved = "CLI transcript saved"
)

// The attribute keys the report decodes. Kept snake_case to match the JSON
// tags on the reader's side, which tagliatelle holds to the same case.
const (
	KeyAction           = "action"
	KeyAllowedTools     = "allowed_tools"
	KeyArgs             = "args"
	KeyAttempt          = "attempt"
	KeyBinary           = "binary"
	KeyCLI              = "cli"
	KeyCommand          = "command"
	KeyContent          = "content"
	KeyCostUSD          = "cost_usd"
	KeyDir              = "dir"
	KeyDisallowedTools  = "disallowed_tools"
	KeyDocs             = "docs"
	KeyDurationMs       = "duration_ms"
	KeyError            = "error"
	KeyExitCode         = "exit_code"
	KeyFile             = "file"
	KeyFramework        = "framework"
	KeyFrom             = "from"
	KeyItems            = "items"
	KeyIteration        = "iteration"
	KeyMaxApplyAttempts = "max_apply_attempts"
	KeyMaxAttempts      = "max_attempts"
	KeyModel            = "model"
	KeyPath             = "path"
	KeyPhase            = "phase"
	KeyPrompts          = "prompts"
	KeyRole             = "role"
	KeySection          = "section"
	KeyServices         = "services"
	KeyStderrBytes      = "stderr_bytes"
	KeyStderrFile       = "stderr_file"
	KeyStdoutBytes      = "stdout_bytes"
	KeyStdoutFile       = "stdout_file"
	KeySystemLength     = "system_length"
	KeyTo               = "to"
	KeyTurn             = "turn"
	KeyUserLength       = "user_length"
)
