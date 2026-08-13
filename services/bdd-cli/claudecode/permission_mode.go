package claudecode

import (
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/claudecode/internal/shared"
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
