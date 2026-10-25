package provider

// TurnOptions controls execution independently of tool permissions.
type TurnOptions struct {
	// ClaudeWriter requests the isolated, unbounded build-code applier.
	ClaudeWriter bool
	// CodexWriter confines only build code's codex apply turn.
	CodexWriter bool
}
