// Package linters is the external linters the gates shell out to, one of the
// typed wrappers pkg/shell may be reached through.
//
// Three tools, one shape: each prints its own findings and its exit code is
// the verdict, so nothing here reads their output for meaning. Two are PATH
// tools a contributor installs (yamale, markdownlint-cli2), which is why
// Available exists separately from running them. golangci-lint has its own
// package: its chatter needs filtering, which is knowledge about that binary.
package linters

import (
	"context"
	"io"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// The binaries this package spawns.
const (
	Alint            = "alint"
	Yamale           = "yamale"
	MarkdownLintCLI2 = "markdownlint-cli2"
)

// Available reports whether a linter is on PATH, so a gate can name the
// install line rather than failing as a mystery spawn error.
func Available(name string) error {
	return shell.Require(name)
}

// Run streams a linter's own output to out and reports its exit code. Stdin
// is left closed: none of the three reads any.
func Run(ctx context.Context, out io.Writer, name string, args ...string) (shell.Result, error) {
	return shell.Run(ctx, append([]string{name}, args...),
		shell.Options{Output: shell.To(out)})
}
