package ai

import "context"

// Request is one single-turn prompt handed to a provider.
type Request struct {
	// SystemPrompt is passed natively where the CLI supports it and
	// prepended to UserPrompt where it does not (crush, codex).
	SystemPrompt string
	// UserPrompt is the rendered checklist/fix template.
	UserPrompt string
	// Model is the provider-native model id — the right-hand side of
	// the `"<cli>:<model>"` reference, already stripped of the cli.
	Model string
	// Mode carries the tool permissions for this turn. Providers
	// without a native allowlist project it onto their own sandbox.
	Mode ExecutionMode
	// ResultSchema, when set, is the JSON Schema the answer must satisfy.
	// Only providers SupportsResultSchema accepts may receive one.
	ResultSchema string
	// WorkDir is the directory the turn runs in (the host project root).
	WorkDir string
	// TmpDir is the run's artifact directory. Providers write their
	// generated config and raw transcripts here, alongside the
	// prompt/response files the callers already save.
	TmpDir string
	// Label distinguishes concurrent artifacts within TmpDir.
	Label string
}

// Provider runs one prompt through a single agent CLI and returns the
// assistant's text. Implementations are stateless: a turn is a
// subprocess, never a session.
type Provider interface {
	// Name is the CLI's config name — the left-hand side of a model
	// reference.
	Name() string
	// Execute runs the turn. The returned string is the raw assistant
	// text; the engine parses its FILE_START/FILE_END markers.
	Execute(ctx context.Context, req Request) (string, error)
}
