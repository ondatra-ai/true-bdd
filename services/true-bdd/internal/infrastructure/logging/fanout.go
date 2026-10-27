package logging

import (
	"context"
	"fmt"
	"log/slog"
)

// fanout writes every record to the JSON file and the loud ones to the text
// stream. A file write that fails is dropped: slog discards a handler's error,
// and a log record must never take the program down with it.
type fanout struct {
	file slog.Handler
	text slog.Handler
}

// Enabled reports whether either sink wants the level.
func (h *fanout) Enabled(ctx context.Context, level slog.Level) bool {
	return h.file.Enabled(ctx, level) || h.text.Enabled(ctx, level)
}

// Handle sends the record to both sinks.
func (h *fanout) Handle(ctx context.Context, record slog.Record) error {
	_ = h.file.Handle(ctx, record)

	if !h.text.Enabled(ctx, record.Level) {
		return nil
	}

	err := h.text.Handle(ctx, record)
	if err != nil {
		return fmt.Errorf("text handler: %w", err)
	}

	return nil
}

// WithAttrs returns a handler carrying attrs on both sinks.
func (h *fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &fanout{file: h.file.WithAttrs(attrs), text: h.text.WithAttrs(attrs)}
}

// WithGroup returns a handler carrying the group on both sinks.
func (h *fanout) WithGroup(name string) slog.Handler {
	return &fanout{file: h.file.WithGroup(name), text: h.text.WithGroup(name)}
}
