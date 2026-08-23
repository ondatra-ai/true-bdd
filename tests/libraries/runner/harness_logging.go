package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// msgAITurnUsage is the record src/adapters/ai/claude_provider.go emits
// after every claude turn. In THIS process only the judge can emit it —
// see InstallHarnessLogging.
const msgAITurnUsage = "AI turn usage"

// HarnessLogFile is where the TEST PROCESS's own records land, one file
// per `go test` invocation. A file at the session root, where the run
// report's directory scan will never mistake it for a fixture.
const HarnessLogFile = "harness.log.json"

// usageTokenKeys are the counters the claude CLI reports, mirroring
// logTurnUsage in src/adapters/ai/claude_provider.go — cache reads and
// writes are priced differently from ordinary input, so kept apart.
func usageTokenKeys() []string {
	return []string{
		"input_tokens",
		"output_tokens",
		"cache_read_input_tokens",
		"cache_creation_input_tokens",
	}
}

// usageEntry is one stamped usage record.
type usageEntry struct {
	at     time.Time
	cost   float64
	tokens map[string]int
}

// UsageSink collects every "AI turn usage" record this process emits,
// stamped, so a recorder can claim the ones inside its judge window.
type UsageSink struct {
	mu      sync.Mutex
	records []usageEntry
}

// Between totals the usage stamped inside [from, until] — exact in a
// way pairing by index was not; see TestUsageOutsideJudgeWindowIsNotBilled.
func (s *UsageSink) Between(from, until time.Time) (float64, map[string]int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cost := 0.0
	tokens := map[string]int{}

	for _, entry := range s.records {
		if entry.at.Before(from) || entry.at.After(until) {
			continue
		}

		cost += entry.cost

		for key, count := range entry.tokens {
			tokens[key] += count
		}
	}

	return cost, tokens
}

// add files one usage record.
func (s *UsageSink) add(record slog.Record) {
	entry := usageEntry{at: record.Time, tokens: map[string]int{}}

	record.Attrs(func(attr slog.Attr) bool {
		switch {
		case attr.Key == "cost_usd":
			entry.cost = numberOf(attr.Value)
		case slices.Contains(usageTokenKeys(), attr.Key):
			entry.tokens[attr.Key] = int(numberOf(attr.Value))
		}

		return true
	})

	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = append(s.records, entry)
}

// numberOf reads a counter regardless of how it reached slog. Usage
// counts decode from JSON as float64, but cost is logged from a
// *float64 directly, and a future field could arrive as a plain int.
func numberOf(value slog.Value) float64 {
	switch value.Kind() {
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindInt64:
		return float64(value.Int64())
	case slog.KindUint64:
		return float64(value.Uint64())
	case slog.KindAny, slog.KindBool, slog.KindDuration,
		slog.KindString, slog.KindTime, slog.KindGroup, slog.KindLogValuer:
		return 0
	default:
		return 0
	}
}

// harnessHandler fans every record out to the session's JSON log and to
// the console, and files the judge's usage into the sink on the way past.
type harnessHandler struct {
	file    slog.Handler
	console slog.Handler
	sink    *UsageSink
}

func (h *harnessHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.file.Enabled(ctx, level) || h.console.Enabled(ctx, level)
}

func (h *harnessHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Message == msgAITurnUsage {
		h.sink.add(record)
	}

	_ = h.file.Handle(ctx, record)

	if !h.console.Enabled(ctx, record.Level) {
		return nil
	}

	err := h.console.Handle(ctx, record)
	if err != nil {
		return fmt.Errorf("console handler: %w", err)
	}

	return nil
}

func (h *harnessHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &harnessHandler{
		file:    h.file.WithAttrs(attrs),
		console: h.console.WithAttrs(attrs),
		sink:    h.sink,
	}
}

func (h *harnessHandler) WithGroup(name string) slog.Handler {
	return &harnessHandler{
		file:    h.file.WithGroup(name),
		console: h.console.WithGroup(name),
		sink:    h.sink,
	}
}

// InstallHarnessLogging points this process's default slog at the
// session, returning the billing sink and a log closer. The engine runs
// as a SUBPROCESS with its own slog — this sink sees only the harness's own records.
func InstallHarnessLogging(sessionRoot string) (*UsageSink, func(), error) {
	path := filepath.Join(sessionRoot, HarnessLogFile)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, spawnLogFilePerm)
	if err != nil {
		return nil, nil, fmt.Errorf("open harness log %s: %w", path, err)
	}

	sink := &UsageSink{}

	slog.SetDefault(slog.New(&harnessHandler{
		file:    slog.NewJSONHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug}),
		console: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}),
		sink:    sink,
	}))

	return sink, func() { _ = file.Close() }, nil
}
