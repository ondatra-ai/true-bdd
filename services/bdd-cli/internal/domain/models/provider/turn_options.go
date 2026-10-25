package provider

// TurnOptions controls execution independently of tool permissions.
type TurnOptions struct {
	// ClaudeWriter requests the isolated, unbounded build-code applier.
	ClaudeWriter bool
	// CodexWriter selects build-code replay forms and started-turn recovery, independently of apply confinement.
	CodexWriter bool
}
