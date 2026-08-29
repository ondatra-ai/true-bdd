package shell

import (
	"bytes"
	"io"
	"os/exec"

	"github.com/ondatra-ai/true-bdd/pkg/console"
)

// Sink is where a child's output goes. The zero value captures both streams
// into Result; every other mode sends the bytes elsewhere, which leaves
// Result.Stdout and Result.Stderr empty.
type Sink struct {
	mode sinkMode
	out  io.Writer
	err  io.Writer
}

type sinkMode int

const (
	sinkCapture sinkMode = iota
	sinkCombined
	sinkConsole
	sinkWriters
	sinkDiscard
	sinkPipe
)

// Capture keeps the two streams apart in Result, and is the zero value.
func Capture() Sink { return Sink{mode: sinkCapture} }

// Combined merges both streams into Result.Stdout the way CombinedOutput
// does, leaving Result.Stderr empty.
func Combined() Sink { return Sink{mode: sinkCombined} }

// Console passes both streams to the terminal, for a command whose progress
// is the point rather than its output.
func Console() Sink { return Sink{mode: sinkConsole} }

// To sends both streams to one writer.
func To(writer io.Writer) Sink { return Sink{mode: sinkWriters, out: writer, err: writer} }

// Streams sends each stream to its own writer.
func Streams(out, err io.Writer) Sink { return Sink{mode: sinkWriters, out: out, err: err} }

// Discard throws both streams away.
func Discard() Sink { return Sink{mode: sinkDiscard} }

// Pipe hands the streams to the caller as pipes on Process. Only Start
// honours it: Run has nobody to read them and would deadlock.
func Pipe() Sink { return Sink{mode: sinkPipe} }

// wire points cmd's streams at this sink, and returns the buffers Result is
// read back out of. Both stay empty unless the mode captures.
func (s Sink) wire(cmd *exec.Cmd) (*bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer

	switch s.mode {
	case sinkCapture:
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
	case sinkCombined:
		cmd.Stdout, cmd.Stderr = &stdout, &stdout
	case sinkConsole:
		cmd.Stdout, cmd.Stderr = console.Out(), console.Err()
	case sinkWriters:
		cmd.Stdout, cmd.Stderr = s.out, s.err
	case sinkDiscard:
		cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	case sinkPipe:
		// Start attaches the pipes; leaving them nil here is what lets it.
	}

	return &stdout, &stderr
}
