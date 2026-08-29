package report

import (
	"log/slog"
	"time"
)

// Open logs the marker that opens a node and returns the one that closes it,
// so `defer Open(name)()` at the top of an operation is the whole contract. A
// node left open never finished: dief's os.Exit skips every defer.
func Open(name string, attrs ...any) func(...any) {
	slog.Info(name, append([]any{KeyTree, TreeStart}, attrs...)...)

	return func(closing ...any) {
		slog.Info(name, append([]any{KeyTree, TreeEnd}, closing...)...)
	}
}

// Leaf logs one measured thing under whichever node is open: just its time,
// plus whatever the caller wants said about it.
func Leaf(name string, started time.Time, attrs ...any) {
	slog.Info(name, append(
		[]any{KeyDurationMs, time.Since(started).Milliseconds()}, attrs...)...)
}
