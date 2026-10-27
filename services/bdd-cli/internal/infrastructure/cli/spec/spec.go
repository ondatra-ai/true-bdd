// Package spec executes a host's architecture-declared argv without shell
// reinterpretation, through the engine's shell channel.
package spec

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/shell"
)

// Run spawns an argv a caller assembled.
func Run(argv []string, opt shell.Options) (shell.Result, error) {
	return shell.Run(context.Background(), argv, opt)
}
