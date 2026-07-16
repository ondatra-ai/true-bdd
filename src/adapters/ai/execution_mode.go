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
			// Sub-agent tools. Every prompt here is a single-turn
			// `claude -p` call; delegating to a sub-agent and awaiting it
			// ends the turn with no output (the parent cannot resume),
			// which silently yields an empty fix prompt. Force inline work.
			"Agent",
			"Task",
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
			// See GetThinkMode: sub-agent delegation breaks single-turn
			// `claude -p` calls, so keep it disallowed here too.
			"Agent",
			"Task",
		},
	}
}
