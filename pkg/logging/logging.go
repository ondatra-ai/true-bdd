// Package logging is every log/slog record this repository emits, and the only
// place a handler is built.
//
// A program binds its own text stream in main(). The engine binds Stdout
// because 22 steps in docs/scenarios.yaml assert `stdout matches level=ERROR
// msg="Refusing to start"`, and docs/scenarios.yaml has no stderr step at all;
// a program whose stdout another program parses binds Stderr, so a log line
// can never corrupt the parsed channel.
//
// Every scripts/ program also appends to one shared docs/history/tools.log.json,
// each record naming its writer in `tool`. Concurrent writers are safe because
// this package emits one disk.Append per record and pkg/disk holds the parent
// directory for it; state.Init truncates the file, so it lives one Task.
//
// THE MESSAGE STRINGS AND ATTRIBUTE KEYS ARE A WIRE CONTRACT.
// tests/libraries/reporter/engine_log.go folds a run into turns on seven exact
// messages and pins about forty attribute keys as JSON tags. Renaming one
// breaks the report and the report server silently: nothing goes red, the
// turns just stop appearing.
package logging

import (
	"io"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Stream is which descriptor a program's log text lands on.
type Stream int

const (
	// Stdout is the engine's choice: scenario steps assert against it.
	Stdout Stream = iota
	// Stderr is for a program whose stdout another program parses.
	Stderr
)

// writer returns the descriptor s names.
func (s Stream) writer() io.Writer {
	if s == Stderr {
		return console.Err()
	}

	return console.Out()
}

// Install points this process's default slog at Handler's result. The file is
// created up front: an absent one must not read as "the run never started".
// An empty tool adds no attribute, keeping the engine's log shape unchanged.
func Install(text Stream, jsonPath, tool string) {
	if jsonPath != "" {
		_ = disk.Ensure(jsonPath, disk.Shared)
	}

	handler := Handler(text.writer(), jsonPath)
	if tool != "" {
		handler = handler.WithAttrs([]slog.Attr{slog.String("tool", tool)})
	}

	slog.SetDefault(slog.New(handler))
}

// Handler builds the sink without installing it, for the one caller that must
// wrap it: the harness taps every "AI turn usage" record on the way past. An
// empty jsonPath is text only, for a program nothing folds a run back out of.
func Handler(text io.Writer, jsonPath string) slog.Handler {
	stream := slog.NewTextHandler(
		text,
		&slog.HandlerOptions{Level: slog.LevelInfo},
	)

	if jsonPath == "" {
		return stream
	}

	return &fanout{
		file: slog.NewJSONHandler(
			appender{path: jsonPath},
			&slog.HandlerOptions{Level: slog.LevelDebug},
		),
		text: stream,
	}
}

// appender is the JSON sink. One disk.Append per record, because no file
// handle escapes pkg/disk; slog writes exactly one record per Write.
type appender struct {
	path string
}

// Write appends one record. slog's handler ends each with a newline, which
// disk.Append supplies itself.
func (a appender) Write(record []byte) (int, error) {
	trimmed := record
	if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\n' {
		trimmed = trimmed[:len(trimmed)-1]
	}

	err := disk.Append(a.path, trimmed, disk.Shared)
	if err != nil {
		return 0, err
	}

	return len(record), nil
}
