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
