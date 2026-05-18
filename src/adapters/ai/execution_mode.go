package ai

import "bdd-cli/src/internal/infrastructure/config"

// ExecutionMode defines tool permissions for AI execution.
type ExecutionMode struct {
	AllowedTools    []string
	DisallowedTools []string
}

// ModeFactory creates execution modes with configured paths.
type ModeFactory struct {
	config *config.ViperConfig
}

// NewModeFactory creates a new mode factory.
func NewModeFactory(config *config.ViperConfig) *ModeFactory {
	return &ModeFactory{config: config}
}

// GetThinkMode returns ThinkMode with configured paths.
func (f *ModeFactory) GetThinkMode() ExecutionMode {
	tmpGlob := f.config.GetString("paths.tmp_glob")

	return ExecutionMode{
		[]string{
			"Read(**)",
			"Write(" + tmpGlob + ")",
			"Glob(**)",
			"Grep(**)",
		},
		[]string{
			"Bash",
			"Edit(**)",
			"MultiEdit(**)",
		},
	}
}

// GetEditMode returns a mode that additionally allows Edit and
// MultiEdit against the configured tmp glob, so callers whose F:
// handlers mutate the scratch registry in place (e.g. us apply) can
// actually run their prompts. ThinkMode disallows Edit globally,
// which is correct for handlers that emit FILE_START/FILE_END
// markers (us create / us refine), but wrong for us apply.
func (f *ModeFactory) GetEditMode() ExecutionMode {
	tmpGlob := f.config.GetString("paths.tmp_glob")

	return ExecutionMode{
		[]string{
			"Read(**)",
			"Write(" + tmpGlob + ")",
			"Edit(" + tmpGlob + ")",
			"MultiEdit(" + tmpGlob + ")",
			"Glob(**)",
			"Grep(**)",
		},
		[]string{
			"Bash",
		},
	}
}
