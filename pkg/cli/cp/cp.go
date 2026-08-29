// Package cp is the `cp` command line, one of the typed wrappers pkg/shell
// may be reached through.
//
// One caller, one shape. It stays a spawn rather than becoming a Go tree walk
// because the bdd-web bundle it assembles is a Next.js standalone output,
// whose symlinks and modes cp already reproduces exactly.
package cp

import (
	"context"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "cp"

// Recursive copies src to dst. A src ending in /. copies its contents rather
// than the directory itself, which is cp's own rule and the caller's intent.
func Recursive(ctx context.Context, src, dst string, opt shell.Options) (shell.Result, error) {
	return shell.Run(ctx, []string{Bin, "-R", src, dst}, opt)
}
