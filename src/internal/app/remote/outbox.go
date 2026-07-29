package remote

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// errPartialAck is the static base for a stalled partial-ack flush (a
// successful response that advanced no further and left events pending).
var errPartialAck = errors.New("partial ack made no progress")

const (
	defaultSpoolBudget = 256 * 1024
	defaultSpoolHead   = 16
	defaultSpoolTail   = 16
	sendFlushAttempts  = 3
	finalFlushAttempts = 10
	outboxFlushBackoff = 200 * time.Millisecond
)

// eventPoster is the subset of ServerClient the outbox and spool replay
// depend on, so tests can drive them with a stub HTTP transport.
type eventPoster interface {
	PostEvents(ctx context.Context, req EventsRequest) (EventsResponse, error)
}

// outboxOptions tunes the spool byte budget, head+tail retention, and
// flush backoff (small values in tests).
type outboxOptions struct {
	budget  int
	head    int
	tail    int
	backoff time.Duration
}

// defaultOutboxOptions returns the production spool bounds.
func defaultOutboxOptions() outboxOptions {
	return outboxOptions{
		budget:  defaultSpoolBudget,
		head:    defaultSpoolHead,
		tail:    defaultSpoolTail,
		backoff: outboxFlushBackoff,
	}
}

// Outbox is the remote's sole owner of one run's (run_id, seq) stream
// (plan §3.3). It assigns the next contiguous seq under a mutex, journals
// every event to a durable spool BEFORE delivery, posts unacked events in
// one serialized in-flight batch, and advances a contiguous acked_through
// watermark from the server — trimming acked events and, on a long
// outage, collapsing the spool middle into a RANGE TOMBSTONE so the
// journal stays bounded (plan §3.6). The spool is deleted only once the
// terminal event is acked.
type Outbox struct {
	poster    eventPoster
	sessionID string
	runID     string
	spool     *spool
	opts      outboxOptions

	mu    sync.Mutex
	state spoolState
}

// newOutbox builds an outbox for one run, resuming any existing spool so
// a re-executed run continues its seq stream.
func newOutbox(poster eventPoster, sessionID, runID, dir string, opts outboxOptions) *Outbox {
	spoolFile := newSpool(dir, runID)
	state, _ := spoolFile.load()

	return &Outbox{
		poster:    poster,
		sessionID: sessionID,
		runID:     runID,
		spool:     spoolFile,
		opts:      opts,
		state:     state,
	}
}

// Send assigns the next seq to event, journals it durably, and attempts a
// bounded flush. On a transient failure the event stays spooled for a
// later flush or startup replay, so Send never blocks the caller long.
func (o *Outbox) Send(ctx context.Context, event OutEvent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.state.Seq++
	event.Seq = o.state.Seq
	o.state.Events = append(o.state.Events, event)
	o.state.Events = truncateUnacked(o.state.Events, o.opts.budget, o.opts.head, o.opts.tail)

	err := o.spool.save(o.state)
	if err != nil {
		return err
	}

	return o.flushLocked(ctx, sendFlushAttempts)
}

// Flush drives delivery of all still-unacked events with a longer bound —
// the executor calls it after the terminal event so a healthy server acks
// it and the spool is deleted; a persistent outage leaves the spool for
// startup replay.
func (o *Outbox) Flush(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.flushLocked(ctx, finalFlushAttempts)
}

// SetAnswersConsumed records the highest answer_seq written to the child's
// stdin (memoized BEFORE the write — plan §3.3) so the next flush reports
// it and the server never re-delivers a consumed answer.
func (o *Outbox) SetAnswersConsumed(seq int) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if seq > o.state.AnswersConsumed {
		o.state.AnswersConsumed = seq
		_ = o.spool.save(o.state)
	}
}

// AnswersConsumedThrough returns the server-confirmed answers-consumed
// watermark (the round-trip of SetAnswersConsumed).
func (o *Outbox) AnswersConsumedThrough() int {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.state.AnswersConsumedThrough
}

// flushLocked posts the unacked batch up to attempts times, folding each
// ack into the spool state. A PARTIAL ack — a successful response that
// advances acked_through without draining the batch (the terminal is not
// yet covered) — is treated as INCOMPLETE, not done (finding 1): the flush
// keeps retrying while progress is possible, so a lost terminal ack is
// recovered within the flush rather than deferred to startup replay.
// Exhaustion leaves the batch durably spooled and returns the last error.
func (o *Outbox) flushLocked(ctx context.Context, attempts int) error {
	var lastErr error

	for range attempts {
		drained, err := o.drainOrPost(ctx)
		if err == nil && drained {
			return nil
		}

		if err != nil {
			lastErr = err
		}

		if !drained && err == nil {
			// A partial ack that made progress: retry immediately.
			continue
		}

		waitErr := sleepCtx(ctx, o.opts.backoff)
		if waitErr != nil {
			return waitErr
		}
	}

	if len(o.state.Events) == 0 {
		return nil
	}

	return fmt.Errorf("flush run %s after %d attempts: %w", o.runID, attempts, lastErr)
}

// drainOrPost performs one flush step. It returns drained=true when the
// unacked batch is fully cleared (spool removed on terminal ack, saved
// otherwise). A partial ack that made progress returns (false, nil) so the
// caller retries immediately; a stalled ack returns (false, errPartialAck)
// so the caller backs off; a transport failure returns the transport error.
func (o *Outbox) drainOrPost(ctx context.Context) (bool, error) {
	if len(o.state.Events) == 0 {
		if o.state.TerminalAcked {
			o.spool.remove()

			return true, nil
		}

		return true, o.spool.save(o.state)
	}

	resp, err := o.poster.PostEvents(ctx, o.request())
	if err != nil {
		return false, err
	}

	before := o.state.AckedThrough
	applyAck(&o.state, resp)

	if o.state.TerminalAcked {
		o.spool.remove()

		return true, nil
	}

	saveErr := o.spool.save(o.state)
	if saveErr != nil {
		return true, saveErr
	}

	if len(o.state.Events) == 0 {
		return true, nil // fully drained but not terminal (an interim Send)
	}

	if o.state.AckedThrough > before {
		return false, nil // partial ack, progress made — retry now
	}

	return false, fmt.Errorf("%w: run %s acked_through %d, %d pending",
		errPartialAck, o.runID, o.state.AckedThrough, len(o.state.Events))
}

// request builds the EventsRequest for the current unacked batch.
func (o *Outbox) request() EventsRequest {
	return EventsRequest{
		SessionID:       o.sessionID,
		RunID:           o.runID,
		Events:          o.state.Events,
		AnswersConsumed: o.state.AnswersConsumed,
	}
}

// sleepCtx sleeps for d or until ctx is cancelled.
func sleepCtx(ctx context.Context, duration time.Duration) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("outbox wait cancelled: %w", ctx.Err())
	case <-time.After(duration):
		return nil
	}
}
