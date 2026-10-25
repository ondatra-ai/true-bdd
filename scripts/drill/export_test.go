package drill

import (
	"context"
	"io"
)

// NewPromptWriter exposes the engine byte-stream seam only to tests.
func NewPromptWriter(ctx context.Context, runner Runner, record Record,
	run Attempt, input io.Writer,
) (io.Writer, func()) {
	ctx, cancel := context.WithCancel(ctx)
	writer := &promptWriter{ctx: ctx, cancel: cancel, runner: runner,
		directory: record.Directory, tag: run.Tag, tree: record.Cuts[run.Cut].Directory,
		log: run.Log, input: input}

	return writer, func() { cancel(); writer.answers.Wait() }
}
