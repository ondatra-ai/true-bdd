package claude

import (
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/enginelog"
)

type permissionDenial struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
		Pattern  string `json:"pattern"`
	} `json:"tool_input"`
}

func warnDenials(denials []permissionDenial, opts Options) {
	calls := make([]enginelog.ToolDenial, 0, len(denials))
	for _, denial := range denials {
		target := denial.ToolInput.FilePath
		if target == "" {
			target = denial.ToolInput.Path
		}

		if denial.ToolName == "Glob" && denial.ToolInput.Pattern != "" {
			target = denial.ToolInput.Pattern
			if !filepath.IsAbs(target) && denial.ToolInput.Path != "" {
				target = filepath.Join(denial.ToolInput.Path, target)
			}
		}

		calls = append(calls, enginelog.ToolDenial{Tool: denial.ToolName, Target: target})
	}

	enginelog.WarnDenials(opts.Label, opts.Role, calls)
}
