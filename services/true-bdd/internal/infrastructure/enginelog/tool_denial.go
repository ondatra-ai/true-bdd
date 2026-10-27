package enginelog

import (
	"log/slog"
	"strings"
)

// ToolDenial identifies a refused call without exposing unrelated input.
type ToolDenial struct {
	Tool   string
	Target string
}

// WarnDenials records refused calls with their turn identity.
func WarnDenials(label, role string, denials []ToolDenial) {
	if len(denials) == 0 {
		return
	}

	const targetLimit = 200

	denied := make([]string, 0, len(denials))
	for _, denial := range denials {
		call := denial.Tool
		if denial.Target != "" {
			target := []rune(denial.Target)
			if len(target) > targetLimit {
				target = append(target[:targetLimit], '…')
			}

			call += " " + string(target)
		}

		denied = append(denied, call)
	}

	slog.Warn("Tool calls were denied; this turn did less than it was asked to",
		"label", label, "role", role, "denied", strings.Join(denied, "; "))
}
