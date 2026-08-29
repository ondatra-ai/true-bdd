// Package spec runs an argv carried as data rather than typed at the call
// site, and exists so the pkg/shell ban has somewhere to send those calls.
//
// Three kinds reach here: a host project's true-bdd.yaml commands, this
// repository's own gate table, and the record/replay shim standing in for an
// agent CLI. Typing any of them would defeat what they are for. What the
// package buys is that such a call is greppable and cannot be mistaken for a
// binary this repository names directly.
package spec

import (
	"context"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Run spawns an argv a caller assembled.
func Run(ctx context.Context, argv []string, opt shell.Options) (shell.Result, error) {
	return shell.Run(ctx, argv, opt)
}

// Start spawns an assembled argv and returns without waiting, for
// tests/libraries/aiproxy, which stands in for the real CLI and must pump its
// streams byte for byte.
func Start(ctx context.Context, argv []string, opt shell.Options) (*shell.Process, error) {
	return shell.Start(ctx, argv, opt)
}
