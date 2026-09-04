package main

import (
	"testing"
	"time"
)

// The hole this closes: see collectUnexpected — a cassette recorded with
// EMPTY stdin must not swallow a request that arrives with bytes.
func TestCollectUnexpectedReturnsLateArrivals(t *testing.T) {
	t.Parallel()

	collector := &stdinCollector{eof: make(chan struct{})}

	// First byte lands AFTER the quiet window has already elapsed.
	go func() {
		time.Sleep(stdinQuietPeriod + 200*time.Millisecond)

		_, _ = collector.buf.Write([]byte("unexpected request"))
		close(collector.eof)
	}()

	if got := string(collector.collect(0)); got != "unexpected request" {
		t.Fatalf("collect(0) = %q, want the late arrival to be reported", got)
	}
}

// The real empty case: the engine sends nothing and closes stdin. EOF
// ends the wait immediately — the hard timeout is only ever paid by a
// caller that neither writes nor closes, which no engine path does.
func TestCollectUnexpectedEndsOnEOF(t *testing.T) {
	t.Parallel()

	collector := &stdinCollector{eof: make(chan struct{})}
	close(collector.eof)

	start := time.Now()

	if got := collector.collect(0); len(got) != 0 {
		t.Fatalf("collect(0) = %q, want empty", got)
	}

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("waited %s after EOF, want it to return at once", elapsed)
	}
}
