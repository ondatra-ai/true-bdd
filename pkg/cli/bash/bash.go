// Package bash runs a fixture-authored command string, and exists so the
// pkg/shell ban has somewhere to send the three sites that need an
// interpreter rather than an argv.
//
// The string is the contract: a fixture's prep.sh and teardown.sh, and a
// materializer manifest's prep steps, are written by whoever designed the
// fixture and run verbatim. .alint.yml's no-shell rule governs shell FILES
// and has never governed `bash -c`, so this is not an exemption from it.
package bash

import (
	"context"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the interpreter this package spawns.
const Bin = "bash"

// Run executes one fixture-authored command string.
func Run(ctx context.Context, command string, opt shell.Options) (shell.Result, error) {
	return shell.Run(ctx, []string{Bin, "-c", command}, opt)
}
