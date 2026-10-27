// Package console is a program's ANSWER: bytes a caller reads verbatim, in a
// shape it fixed, plus the engine's prompts and the reader they are answered
// on. It is the only place that names the three standard descriptors.
//
// What a program says ABOUT its own run is not an answer — that is
// pkg/logging, and scripts/ is denied this package outright by depguard
// because it only ever narrates. Five files there are exempt and each holds a
// descriptor rather than a print: the lint hook's stdin and its JSON verdict,
// and three that wire a child process's streams.
//
// It is byte-plain forever — no colour, no prefixes, no spinners, no TTY
// detection — because these bytes are not only read by a person. The
// PostToolUse lint hook's stdout is parsed as JSON by Claude Code, and 180
// steps in docs/scenarios.yaml assert against the CLI's stdout with a regex.
// A decoration is a silent parser break, so there is no way to add one.
//
// Which bytes are emitted is a tested contract; which writer they reach is
// not. Routing an existing line through a different io.Writer is safe,
// changing the line is not.
package console

import (
	"fmt"
	"io"
	"os"
)

// Console writes one program's stdout. The descriptor is bound in main() and
// nowhere else, which is what makes every command testable against a buffer.
type Console struct {
	out io.Writer
}

// New returns a Console writing to out.
func New(out io.Writer) *Console {
	return &Console{out: out}
}

// Out is the process's stdout, and the one sanctioned naming of it. A
// subprocess's cmd.Stdout is wired to this rather than to os.Stdout.
func Out() *os.File {
	return os.Stdout
}

// Err is the process's stderr.
func Err() *os.File {
	return os.Stderr
}

// In is the process's stdin.
func In() *os.File {
	return os.Stdin
}

// Writer exposes the underlying writer for the one caller that must hand it to
// text/tabwriter. Nothing else should take it.
func (c *Console) Writer() io.Writer {
	return c.out
}

// Println writes with a newline.
func (c *Console) Println(a ...any) {
	_, _ = fmt.Fprintln(c.out, a...)
}

// std is the process's Console, package-level for log/slog's reason.
//
//nolint:gochecknoglobals // that is what a process-wide binding is.
var std = New(Out())

// SetDefault rebinds the process's Console. Called once, in main().
func SetDefault(c *Console) { std = c }

// Default returns the process's Console.
func Default() *Console { return std }

// Println writes with a newline to the process's Console.
func Println(a ...any) { std.Println(a...) }
