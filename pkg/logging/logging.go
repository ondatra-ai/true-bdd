// Package logging is every log/slog record this repository emits, and the only
// place a handler is built.
//
// A program binds its own text stream in main(). The engine binds Stdout
// because 22 steps in docs/scenarios.yaml assert `stdout matches level=ERROR
// msg="Refusing to start"`, and docs/scenarios.yaml has no stderr step at all;
// a program whose stdout another program parses binds Stderr, so a log line
// can never corrupt the parsed channel.
//
// Every scripts/ program also appends to one shared log per Task, under
// docs/history/task_logs/, each record naming its writer in `tool` and its
// process in `run`. Concurrent writers are safe because this package emits one
// disk.Append per record and pkg/disk holds the parent directory for it.
//
// `run` is what makes that shared file parseable: scripts/report folds one
// run's tree out of it, and merge's fix agents spawn `go run
// ./scripts/cmd/gates run` as a separate process appending to the same file.
// Without filtering to one `run` first, the interleaving corrupts the tree
// rather than merely cluttering it.
//
// THE MESSAGE STRINGS AND ATTRIBUTE KEYS ARE A WIRE CONTRACT.
// tests/libraries/reporter/engine_log.go folds a run into turns on seven exact
// messages and pins about forty attribute keys as JSON tags. Renaming one
// breaks the report and the report server silently: nothing goes red, the
// turns just stop appearing.
package logging

import (
	"crypto/rand"
	"encoding/hex"
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
// created up front: E2E-019 reads an engine log with zero records, so a
// missing one is not an empty one.
func Install(text Stream, jsonPath, tool string) {
	if jsonPath != "" {
		_ = disk.Ensure(jsonPath, disk.Shared)
	}

	handler := Handler(text.writer(), jsonPath)

	// The engine passes no tool, and 22 scenario steps regex its Stdout lines
	// against goldens a per-process token would break.
	if tool != "" {
		handler = handler.WithAttrs([]slog.Attr{
			slog.String("tool", tool),
			slog.String("run", runID),
		})
	}

	slog.SetDefault(slog.New(handler))
}

// runIDBytes is the width of the per-process run id.
const runIDBytes = 4

// runID names this process among the shared Task log's writers.
//
//nolint:gochecknoglobals // one process is one run: process-wide is the scope.
var runID = mintRunID()

// Run is this process's run id, for a program asking for its own report.
func Run() string { return runID }

// mintRunID returns 8 hex characters, once per process, so that two Install
// calls cannot disagree. crypto/rand.Read fills the buffer or panics.
func mintRunID() string {
	buffer := make([]byte, runIDBytes)
	_, _ = rand.Read(buffer)

	return hex.EncodeToString(buffer)
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
