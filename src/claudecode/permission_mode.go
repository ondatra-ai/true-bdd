package claudecode

import (
	"bdd-cli/src/claudecode/internal/shared"
)

// PermissionMode defines the permission handling mode.
type PermissionMode = shared.PermissionMode

// Re-export permission mode constants.
const (
	PermissionModeDefault           = shared.PermissionModeDefault
	PermissionModeAcceptEdits       = shared.PermissionModeAcceptEdits
	PermissionModePlan              = shared.PermissionModePlan
	PermissionModeBypassPermissions = shared.PermissionModeBypassPermissions
)
