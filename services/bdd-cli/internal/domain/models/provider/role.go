package provider

// Role is the AI turn a model tier is being resolved for. Each role has its
// own engine-level default, so an unnamed tier means a different model for
// the turn that writes a file than for the one that validates a question.
type Role string

const (
	// RolePrompt runs the validation turn that answers `Q:`.
	RolePrompt Role = "prompt"
	// RoleFix runs the turn that converts a failure into a fix prompt.
	RoleFix Role = "fix"
	// RoleApply runs the turn that writes the fix to disk.
	RoleApply Role = "apply"
)

// Roles lists every role the engine defines, in the order a checklist's
// `engine:` block declares them so validation errors are deterministic.
func Roles() []Role {
	return []Role{RolePrompt, RoleFix, RoleApply}
}

// ConfigKey names the true-bdd.yaml key holding this role's default
// tier. Derived rather than tabulated so the config surface cannot drift
// away from the roles it configures.
func (r Role) ConfigKey() string {
	return "engine.default_" + string(r) + "_model"
}
