package truebdd

import (
	"context"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// ReleaseVersion verifies that the installed release binary can execute.
func ReleaseVersion(ctx context.Context, binary string, opt shell.Options) (shell.Result, error) {
	return shell.Run(ctx, []string{binary, "version"}, opt)
}

// InitHost installs the release's own host configuration and resources.
func InitHost(ctx context.Context, binary string, opt shell.Options) (shell.Result, error) {
	return shell.Run(ctx, []string{binary, "init"}, opt)
}

// RehearseCode reaches the first AI turn, or completes an unattended live repair.
func RehearseCode(ctx context.Context, binary string, live bool, opt shell.Options) (shell.Result, error) {
	args := []string{binary, buildVerb, "code", fixFlag}
	if live {
		args = append(args, "--yes")
	}

	return shell.Run(ctx, args, opt)
}
