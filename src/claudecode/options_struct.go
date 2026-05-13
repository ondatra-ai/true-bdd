package claudecode

import (
	"bdd-cli/src/claudecode/internal/shared"
)

// Options contains configuration for Claude Code CLI interactions.
type Options = shared.Options

// Option configures Options using the functional options pattern.
type Option func(*Options)

// NewOptions creates Options with default values using functional options pattern.
func NewOptions(opts ...Option) *Options {
	// Create options with defaults from shared package
	options := shared.NewOptions()

	// Apply functional options
	for _, opt := range opts {
		opt(options)
	}

	return options
}
