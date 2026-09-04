package main

import (
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/console"
)

// Stdin collection tuning. Collection stops once the byte count reaches the
// recorded budget and holds stable, or after a quiet window with data (see
// stdinQuietPeriod); stdinReadTimeout is the ceiling for a request that never arrives.
const (
	stdinReadTimeout = 30 * time.Second
	// stdinQuietPeriod is the fallback for a request that never reaches the
	// recorded byte count — the COMMON case: recorded length reflects the
	// recording machine's paths, so e.g. a shorter pid at replay misses `size >= want` forever.
	stdinQuietPeriod  = 400 * time.Millisecond
	stdinStablePeriod = 200 * time.Millisecond
	stdinPollPeriod   = 50 * time.Millisecond

	// replayLingerTimeout backstops awaitClose against an engine that
	// never closes stdin; the fixture timeout is the real bound.
	replayLingerTimeout = 5 * time.Minute
)

// stdinCollector owns the shim's stdin for a replay: one background
// copier feeds the buffer for the whole process lifetime, so the
// request can be collected first and the pipe's close observed later.
type stdinCollector struct {
	buf lockedBuffer
	eof chan struct{}
}

func startStdinCollector() *stdinCollector {
	collector := &stdinCollector{eof: make(chan struct{})}

	go func() {
		_, _ = io.Copy(&collector.buf, console.In())

		close(collector.eof)
	}()

	return collector
}

// collect returns the incoming request: what arrives until the stop
// conditions documented at the tuning consts. A goroutine + timers, not
// SetReadDeadline — deadlines on an inherited stdin pipe aren't reliably supported.
func (c *stdinCollector) collect(want int) []byte {
	// A zero budget means the RECORDING had empty stdin, not that this call
	// does — see collectUnexpected for why late-arriving bytes still get hashed.
	if want == 0 {
		return c.collectUnexpected()
	}

	deadline := time.After(stdinReadTimeout)

	lastLen := 0
	stableSince := time.Now()

	for {
		select {
		case <-c.eof:
			return c.buf.Bytes()
		case <-deadline:
			return c.buf.Bytes()
		case <-time.After(stdinPollPeriod):
		}

		if size := c.buf.Len(); size != lastLen {
			lastLen = size
			stableSince = time.Now()

			continue
		}

		if collectedEnough(lastLen, want, time.Since(stableSince)) {
			return c.buf.Bytes()
		}
	}
}

// collectUnexpected waits out the quiet window for a call whose recording
// had no stdin: late-arriving bytes still get hashed and rejected rather
// than silently matching the empty-stdin cassette. EOF short-circuits the wait.
func (c *stdinCollector) collectUnexpected() []byte {
	deadline := time.After(stdinReadTimeout)

	var quiet <-chan time.Time

	for {
		select {
		case <-c.eof:
			return c.buf.Bytes()
		case <-deadline:
			return c.buf.Bytes()
		case <-quiet:
			return c.buf.Bytes()
		case <-time.After(stdinPollPeriod):
		}

		// The quiet window only starts once something has arrived — an empty
		// buffer waits for EOF or the hard timeout, since there's no way to
		// tell "no stdin" from "first byte not landed yet" without waiting.
		if quiet == nil && c.buf.Len() > 0 {
			quiet = time.After(stdinQuietPeriod)
		}
	}
}

// collectedEnough decides when the poll loop may stop on unchanged
// data: budget reached and briefly stable, or any data followed by a
// long quiet window (the shorter-than-recorded case).
func collectedEnough(size, want int, stable time.Duration) bool {
	if size >= want && stable >= stdinStablePeriod {
		return true
	}

	return size > 0 && stable >= stdinQuietPeriod
}

// awaitClose blocks until the parent closes the shim's stdin — the engine's
// end-of-turn signal: claude closes only after consuming the result frame;
// crush hands EOF immediately. Exiting early truncates the response.
func (c *stdinCollector) awaitClose() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	select {
	case <-c.eof:
	case <-sigs:
	case <-time.After(replayLingerTimeout):
	}
}
