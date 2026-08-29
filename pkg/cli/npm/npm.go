// Package npm is the Node toolchain the bdd-web suite drives — `npm` and
// `node` — and is one of the typed wrappers pkg/shell may be reached through.
//
// StartNode returns a handle rather than a result because the application
// under test outlives the call that spawns it: the suite probes it over HTTP
// and stops it in teardown.
package npm

import (
	"context"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// The binaries this package spawns.
const (
	Bin     = "npm"
	NodeBin = "node"
)

// Install fetches a package into prefix without touching a lockfile, which is
// how the suite stages a driver it does not want recorded as a dependency.
func Install(ctx context.Context, prefix, pkg string, opt shell.Options) (shell.Result, error) {
	return shell.Run(ctx,
		[]string{Bin, "install", "--no-save", "--no-package-lock", "--prefix", prefix, pkg},
		opt)
}

// RunScript runs one of a package.json's scripts.
func RunScript(ctx context.Context, name string, opt shell.Options) (shell.Result, error) {
	return shell.Run(ctx, []string{Bin, "run", name}, opt)
}

// NodePath is where node resolves on PATH, for the caller that symlinks it
// rather than spawning it.
func NodePath() (string, error) {
	return shell.Find(NodeBin)
}

// StartNode spawns node and returns without waiting, for a server whose
// lifetime belongs to the caller's teardown.
func StartNode(ctx context.Context, args []string, opt shell.Options) (*shell.Process, error) {
	return shell.Start(ctx, append([]string{NodeBin}, args...), opt)
}
