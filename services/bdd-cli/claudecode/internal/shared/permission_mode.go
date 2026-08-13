package shared

// PermissionMode represents the different permission handling modes.
type PermissionMode string

const (
	// PermissionModeDefault is the standard permission handling mode.
	PermissionModeDefault PermissionMode = "default"
	// PermissionModeAcceptEdits automatically accepts all edit permissions.
	PermissionModeAcceptEdits PermissionMode = "acceptEdits"
	// PermissionModePlan enables plan mode for task execution.
	PermissionModePlan PermissionMode = "plan"
	// PermissionModeBypassPermissions bypasses all permission checks.
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
)
