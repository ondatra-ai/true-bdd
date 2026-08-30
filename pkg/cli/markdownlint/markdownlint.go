// Package markdownlint is the `markdownlint-cli2` command line, one of the
// typed wrappers pkg/shell may be reached through.
//
// The config is never passed with --config: cli2's walk up from each linted
// file is what makes its `overrides:` and per-directory config work at all.
// That is knowledge about this binary, so the argv is built here and a caller
// names files, not flags.
package markdownlint

import (
	"context"
	"fmt"
	"io"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "markdownlint-cli2"

// Install is the line that gets it.
const Install = "brew install markdownlint-cli2"

// FixFlag asks cli2 to rewrite what it can.
const FixFlag = "--fix"

// Available reports whether markdownlint-cli2 is on PATH, naming the install
// line rather than leaving a caller with a mystery spawn error.
func Available() error {
	err := shell.Require(Bin)
	if err != nil {
		return fmt.Errorf("%w — install it with: %s", err, Install)
	}

	return nil
}

// Lint checks the named files, rewriting what it can when fix is set, and
// streams cli2's own findings to out. A non-zero Result.Code is a finding.
func Lint(out io.Writer, fix bool, files ...string) (shell.Result, error) {
	argv := []string{Bin}
	if fix {
		argv = append(argv, FixFlag)
	}

	return shell.Run(context.Background(), append(argv, files...),
		shell.Options{Output: shell.To(out)})
}
