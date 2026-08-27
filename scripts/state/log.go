package state

import (
	"fmt"
	"log/slog"
	"os"
)

// Capture's off switch, mirrored from scripts/history: one variable turns
// both the transcript and this log off together.
const (
	roleEnv = "CLAUDE_HISTORY_ROLE"
	offRole = "0"
)

// appendWriter gives every record its own O_APPEND write — the same
// discipline as Set, so concurrent processes share one log with no lock, and
// the file appears with the first record instead of with the Task.
type appendWriter struct {
	path string
}

func (w appendWriter) Write(payload []byte) (int, error) {
	handle, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return 0, fmt.Errorf("opening %s: %w", w.path, err)
	}

	defer handle.Close() //nolint:errcheck // the write below reports the failure that matters.

	written, err := handle.Write(payload)
	if err != nil {
		return written, fmt.Errorf("appending to %s: %w", w.path, err)
	}

	return written, nil
}

// Log writes docs/history/<task>.log.json in the shape the engine's slog
// emits, so tests/libraries/reporter reads both. It discards when capture is
// off or no Task is open: a duration has nowhere to be attributed.
func Log(repo string) *slog.Logger {
	task := Get(repo, TaskKey)
	if task == "" || os.Getenv(roleEnv) == offRole {
		return slog.New(slog.DiscardHandler)
	}

	err := os.MkdirAll(HistoryDir(repo), dirMode)
	if err != nil {
		return slog.New(slog.DiscardHandler)
	}

	return slog.New(slog.NewJSONHandler(
		appendWriter{path: LogFile(repo, task)}, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
