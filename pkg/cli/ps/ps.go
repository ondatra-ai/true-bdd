// Package ps is the `ps` command line, one of the typed wrappers pkg/shell
// may be reached through.
//
// Two callers, both in the remote supervisor, and both read the output as a
// table rather than a verdict: one asks when a pid started, so a recycled pid
// cannot be mistaken for the process that held it; the other lists a process
// group so the supervisor can wait for it to drain.
package ps

import (
	"context"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "ps"

// Output runs ps and returns its stdout untrimmed, because both callers parse
// it line by line and a trailing newline is part of that shape.
func Output(ctx context.Context, args ...string) (string, error) {
	result, err := shell.Run(ctx, append([]string{Bin}, args...), shell.Options{})
	if err != nil {
		return "", err
	}

	return result.Stdout, result.Err()
}
