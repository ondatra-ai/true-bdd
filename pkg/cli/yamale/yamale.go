// Package yamale is the `yamale` command line, one of the typed wrappers
// pkg/shell may be reached through.
//
// It validates one YAML document against one schema and says so itself, so
// nothing here reads its output for meaning: the exit code is the verdict.
// Install is here because a PATH tool's absence is only useful to a caller
// with the line that fixes it, and that line is knowledge about this binary.
package yamale

import (
	"context"
	"fmt"
	"io"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "yamale"

// Install is the line that gets it.
const Install = "pip install yamale"

// Available reports whether yamale is on PATH, naming the install line rather
// than leaving a caller with a mystery spawn error.
func Available() error {
	err := shell.Require(Bin)
	if err != nil {
		return fmt.Errorf("%w — install it with: %s", err, Install)
	}

	return nil
}

// Validate checks one document against one schema, streaming yamale's own
// report to out. A non-zero Result.Code is a failed validation, not an error.
func Validate(out io.Writer, schema, document string) (shell.Result, error) {
	return shell.Run(context.Background(), []string{Bin, "-s", schema, document},
		shell.Options{Output: shell.To(out)})
}
