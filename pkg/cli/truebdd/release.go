package truebdd

import (
	"context"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// InitHost installs the release's own host configuration and resources.
func InitHost(ctx context.Context, binary string, opt shell.Options) (shell.Result, error) {
	return shell.Run(ctx, []string{binary, "init"}, opt)
}

// RehearseCode runs the release's build-code fix loop with the caller's stdin.
func RehearseCode(ctx context.Context, binary string, opt shell.Options) (shell.Result, error) {
 return shell.Run(ctx, []string{binary, buildVerb, "code", fixFlag}, opt)
}
