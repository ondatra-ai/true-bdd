package main

import (
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Stdin collection tuning. The engine writes the whole request in one
// burst right after spawning, so collection stops as soon as the byte
// count reaches the recorded budget and stays stable
// (stdinStablePeriod), or — when the incoming request is shorter than
// the recording, e.g. a narrower pid in a normalized path, or a changed
// prompt — after a quiet window with data (stdinQuietPeriod).
// stdinReadTimeout is the hard ceiling for a request that never
// arrives; the hash check right after turns every short read into a
// loud stale-cassette failure instead of a hang.
const (
	stdinReadTimeout  = 30 * time.Second
	stdinQuietPeriod  = 2 * time.Second
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
		_, _ = io.Copy(&collector.buf, os.Stdin)

		close(collector.eof)
	}()

	return collector
}

// collect returns the incoming request: what arrives until the stop
// conditions documented at the tuning consts. A goroutine + timers
// rather than SetReadDeadline: deadlines on an inherited stdin pipe are
// not reliably supported, and a silent no-op deadline turns a changed
// request into a hang instead of a stale-cassette failure.
func (c *stdinCollector) collect(want int) []byte {
	// A zero budget means the RECORDING had empty stdin — it does not
	// mean this call does. Returning early here would hash nil, match
	// the empty-stdin cassette, and serve it to a request that carried
	// bytes: the one way a changed request could pass the hash check the
	// whole design rests on. So the quiet window is still observed, and
	// anything that arrives is returned to be hashed and rejected.
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

// collectUnexpected waits out the quiet window for a call whose
// recording had no stdin, and returns whatever arrived anyway.
//
// Usually nothing does, and this costs one quiet period. When something
// does, returning it is the point: the caller hashes it, the hash misses
// the empty-stdin cassette, and the run fails loudly instead of
// replaying a response recorded for a different request. EOF short-
// circuits the wait, which is the common case for a CLI whose prompt
// travels in argv.
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

		// The quiet window only starts once something has arrived. An
		// empty buffer waits for EOF or the hard timeout instead: there
		// is no way to tell "this call sends no stdin" from "the first
		// byte has not landed yet" without waiting, and stopping early
		// on the guess is exactly how a late request would get hashed as
		// empty and matched against the wrong cassette.
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

// awaitClose blocks until the parent closes the shim's stdin (or
// signals it) — the engine's own end-of-turn signal: the claude
// transport closes stdin only after it has CONSUMED the result frame,
// and crush's engine hands the shim EOF immediately. Exiting before
// this races the engine's pipe drain — process-exit teardown cancels
// the stream reader with replayed frames still in the pipe buffer,
// truncating the response the engine sees.
func (c *stdinCollector) awaitClose() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	select {
	case <-c.eof:
	case <-sigs:
	case <-time.After(replayLingerTimeout):
	}
}
