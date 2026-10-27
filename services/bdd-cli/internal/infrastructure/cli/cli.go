// Package cli exposes the engine's subprocess vocabulary without granting
// direct shell access. Typed wrappers own spawning; callers hold aliases of
// the shell's options, results and environment.
package cli

import (
	"io"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/shell"
)

// The spawn vocabulary, aliased so a caller names it without importing the
// package that owns spawning.
type (
	// Options tune one command.
	Options = shell.Options
	// Result is one finished command.
	Result = shell.Result
	// Env is a child's environment.
	Env = shell.Env
	// Sink is where a child's output goes.
	Sink = shell.Sink
)

// NotStarted is Result.Code when the process never ran.
const NotStarted = shell.NotStarted

// The errors a caller distinguishes, re-exported alongside the types.
var (
	// ErrExit reports a non-zero exit.
	ErrExit = shell.ErrExit
	// ErrTimeout reports that the command outlived its deadline.
	ErrTimeout = shell.ErrTimeout
	// ErrNotStarted reports that the command never ran.
	ErrNotStarted = shell.ErrNotStarted
	// ErrNotOnPath reports a binary absent from PATH.
	ErrNotOnPath = shell.ErrNotOnPath
)

// Inherit is the parent environment, unchanged.
func Inherit() Env { return shell.Inherit() }

// Exact is a complete environment, ignoring the parent's.
func Exact(entries []string) Env { return shell.Env{}.Exact(entries) }

// Require reports the first named binary missing from PATH.
func Require(names ...string) error {
	return shell.Require(names...)
}

// Find resolves a binary on PATH.
func Find(name string) (string, error) {
	return shell.Find(name)
}

// Capture keeps the two streams apart in Result, and is the zero value.
func Capture() Sink { return shell.Capture() }

// Combined merges both streams into Result.Stdout.
func Combined() Sink { return shell.Combined() }

// Console passes both streams to the terminal.
func Console() Sink { return shell.Console() }

// To sends both streams to one writer.
func To(writer io.Writer) Sink { return shell.To(writer) }

// Streams sends each stream to its own writer.
func Streams(out, err io.Writer) Sink { return shell.Streams(out, err) }

// Discard throws both streams away.
func Discard() Sink { return shell.Discard() }

// Pipe hands the streams to the caller as pipes on Process.
func Pipe() Sink { return shell.Pipe() }
