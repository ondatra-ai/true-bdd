package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// chunkData is a small output payload reused across the outbox tests.
const chunkData = "chunk"

// fastOpts returns outbox options with a negligible backoff so the
// bounded-retry paths run quickly in tests.
func fastOpts() outboxOptions {
	opts := defaultOutboxOptions()
	opts.backoff = time.Millisecond

	return opts
}

// spoolExists reports whether the run's spool file is present.
func spoolExists(dir, runID string) bool {
	_, err := os.Stat(filepath.Join(dir, runID+spoolSuffix))

	return err == nil
}

// TestOutboxContiguousAckOverHTTP drives the outbox against a real stub
// HTTP server (plan §4.6) whose responses are dropped both BEFORE and
// AFTER commit. Retries plus server-side INSERT OR IGNORE yield a
// contiguous acked_through reaching the terminal, with the terminal
// transition applied exactly once.
func TestOutboxContiguousAckOverHTTP(t *testing.T) {
	t.Parallel()

	stub := newStubEvents()
	stub.decide = func(attempt int) postDecision {
		switch attempt {
		case 1:
			return decideDropPre // server never saw event 1
		case 2:
			return decideDropPost // server committed event 1, reply lost
		default:
			return decideOK
		}
	}

	server := httptest.NewServer(mountEvents(stub))
	defer server.Close()

	client := NewServerClient(server.URL)
	dir := t.TempDir()
	outbox := newOutbox(client, testSessionID, testRunID, dir, fastOpts())

	ctx := context.Background()
	for range 4 {
		mustSend(t, outbox, ctx, OutEvent{Type: eventTypeOutput, Stream: streamStdout, Data: chunkData})
	}

	mustSend(t, outbox, ctx, OutEvent{Type: eventTypeTerminal, Outcome: outcomeConverged})

	if got := stub.ackedThrough(testRunID); got != 5 {
		t.Fatalf("acked_through = %d, want 5", got)
	}

	if got := stub.terminalApplications(testRunID); got != 1 {
		t.Fatalf("terminal applied %d times, want exactly 1", got)
	}

	if spoolExists(dir, testRunID) {
		t.Fatal("spool should be deleted after terminal ack")
	}
}

// TestOutboxSerializedSender fires concurrent Sends and asserts the stub
// never sees two in-flight posts — the outbox is the sole serialized owner
// of the (run_id, seq) stream — while every seq lands exactly once and the
// watermark reaches the last event.
func TestOutboxSerializedSender(t *testing.T) {
	t.Parallel()

	const senders = 12

	stub := newStubEvents()
	stub.delay = 2 * time.Millisecond

	outbox := newOutbox(stub, testSessionID, testRunID, t.TempDir(), fastOpts())
	ctx := context.Background()

	var group sync.WaitGroup

	group.Add(senders)

	for range senders {
		go func() {
			defer group.Done()

			_ = outbox.Send(ctx, OutEvent{Type: eventTypeOutput, Stream: streamStdout, Data: "x"})
		}()
	}

	group.Wait()

	if stub.maxInFlight != 1 {
		t.Fatalf("max in-flight posts = %d, want 1 (sender not serialized)", stub.maxInFlight)
	}

	if got := stub.ackedThrough(testRunID); got != senders {
		t.Fatalf("acked_through = %d, want %d", got, senders)
	}
}

// TestOutboxAnswersConsumedRoundTrip proves the answers_consumed_through
// watermark round-trips: the outbox reports its locally-memoized consumed
// answer_seq and reads back the server's confirmation.
func TestOutboxAnswersConsumedRoundTrip(t *testing.T) {
	t.Parallel()

	stub := newStubEvents()
	outbox := newOutbox(stub, testSessionID, testRunID, t.TempDir(), fastOpts())
	ctx := context.Background()

	outbox.SetAnswersConsumed(3)
	mustSend(t, outbox, ctx, OutEvent{Type: eventTypeOutput, Data: "post-answer output"})

	if got := outbox.AnswersConsumedThrough(); got != 3 {
		t.Fatalf("answers_consumed_through = %d, want 3", got)
	}

	if got := stub.answersConsumedThrough(testRunID); got != 3 {
		t.Fatalf("server answers_consumed = %d, want 3", got)
	}
}

// TestOutboxServerOutageSpooling holds the server down through several
// sends (events accumulate durably in the spool), then recovers and
// flushes — delivering every spooled event exactly once and deleting the
// spool only after the terminal ack (plan §3.6).
func TestOutboxServerOutageSpooling(t *testing.T) {
	t.Parallel()

	var down atomicBool

	down.set(true)

	stub := newStubEvents()
	stub.decide = func(int) postDecision {
		if down.get() {
			return decideDropPre
		}

		return decideOK
	}

	dir := t.TempDir()
	outbox := newOutbox(stub, testSessionID, testRunID, dir, fastOpts())
	ctx := context.Background()

	// While down: sends fail but the events are durably spooled.
	_ = outbox.Send(ctx, OutEvent{Type: eventTypeOutput, Data: "one"})
	_ = outbox.Send(ctx, OutEvent{Type: eventTypeOutput, Data: "two"})
	_ = outbox.Send(ctx, OutEvent{Type: eventTypeTerminal, Outcome: outcomeConverged})

	if !spoolExists(dir, testRunID) {
		t.Fatal("expected a durable spool while the server is down")
	}

	if stub.ackedThrough(testRunID) != 0 {
		t.Fatal("nothing should be acked while the server is down")
	}

	// Recover and flush; delivery is exactly-once and the spool is removed.
	down.set(false)
	mustFlush(t, outbox, ctx)

	if got := stub.ackedThrough(testRunID); got != 3 {
		t.Fatalf("acked_through after recovery = %d, want 3", got)
	}

	if got := stub.terminalApplications(testRunID); got != 1 {
		t.Fatalf("terminal applied %d times, want 1", got)
	}

	if spoolExists(dir, testRunID) {
		t.Fatal("spool should be deleted after terminal ack")
	}

	// A second flush after recovery is a no-op (nothing spooled).
	mustFlush(t, outbox, ctx)

	if got := stub.terminalApplications(testRunID); got != 1 {
		t.Fatalf("second flush re-applied terminal: %d", got)
	}
}

// TestOutboxSpoolHeadTailTruncation forces the spool over its byte budget
// during an outage so the middle collapses into a RANGE TOMBSTONE, then
// recovers: the gap participates in contiguous ack so acked_through still
// reaches the terminal, and the spool is deleted (plan §3.6).
func TestOutboxSpoolHeadTailTruncation(t *testing.T) {
	t.Parallel()

	var down atomicBool

	down.set(true)

	stub := newStubEvents()
	stub.decide = func(int) postDecision {
		if down.get() {
			return decideDropPre
		}

		return decideOK
	}

	opts := fastOpts()
	opts.budget = 8 // bytes — forces truncation almost immediately
	opts.head = 1
	opts.tail = 1

	dir := t.TempDir()
	outbox := newOutbox(stub, testSessionID, testRunID, dir, opts)
	ctx := context.Background()

	const total = 8
	for range total - 1 {
		_ = outbox.Send(ctx, OutEvent{Type: eventTypeOutput, Data: chunkData})
	}

	_ = outbox.Send(ctx, OutEvent{Type: eventTypeTerminal, Outcome: outcomeConverged})

	// The journal stayed bounded to head + gap + tail despite `total` sends.
	state := loadSpool(t, dir, testRunID)
	if len(state.Events) > opts.head+opts.tail+1 {
		t.Fatalf("spool held %d events; expected head+gap+tail bound", len(state.Events))
	}

	if !hasGap(state.Events) {
		t.Fatal("expected a RANGE TOMBSTONE after middle truncation")
	}

	down.set(false)
	mustFlush(t, outbox, ctx)

	if got := stub.ackedThrough(testRunID); got != total {
		t.Fatalf("acked_through = %d, want %d (gap must cover the dropped span)", got, total)
	}

	if spoolExists(dir, testRunID) {
		t.Fatal("spool should be deleted after terminal ack")
	}
}

// TestOutboxCrossLayerRecovery is the demanded cross-layer scenario
// (finding 1): an early event is committed but its reply is LOST
// (post-commit loss), a later PRE-commit outage window strands several
// events, the spool byte budget forces a RANGE TOMBSTONE truncation, and
// the terminal is delivered on recovery. The already-committed event is
// re-delivered idempotently, the gap carries the contiguous ack across the
// collapsed span to the terminal seq, and the spool is deleted exactly
// once — no event or terminal is applied twice.
func TestOutboxCrossLayerRecovery(t *testing.T) {
	t.Parallel()

	var down atomicBool

	down.set(true)

	stub := newStubEvents()
	stub.decide = func(attempt int) postDecision {
		switch {
		case attempt == 1:
			return decideDropPost // event 1 committed on the server, reply lost
		case down.get():
			return decideDropPre // pre-commit outage window
		default:
			return decideOK
		}
	}

	opts := fastOpts()
	opts.budget = 8 // bytes — forces truncation during the outage
	opts.head = 1
	opts.tail = 1

	dir := t.TempDir()
	outbox := newOutbox(stub, testSessionID, testRunID, dir, opts)
	ctx := context.Background()

	// Event 1 is committed-but-reply-lost, then the outage strands 2..7.
	const total = 8
	for range total - 1 {
		_ = outbox.Send(ctx, OutEvent{Type: eventTypeOutput, Data: chunkData})
	}

	_ = outbox.Send(ctx, OutEvent{Type: eventTypeTerminal, Outcome: outcomeConverged})

	state := loadSpool(t, dir, testRunID)
	if !hasGap(state.Events) {
		t.Fatal("expected a RANGE TOMBSTONE after middle truncation during the outage")
	}

	// Recover: the flush re-delivers the committed event 1 idempotently and
	// the gap carries the ack across the collapsed span to the terminal.
	down.set(false)
	mustFlush(t, outbox, ctx)

	if got := stub.ackedThrough(testRunID); got != total {
		t.Fatalf("acked_through after recovery = %d, want %d", got, total)
	}

	if got := stub.terminalApplications(testRunID); got != 1 {
		t.Fatalf("terminal applied %d times, want exactly 1", got)
	}

	if spoolExists(dir, testRunID) {
		t.Fatal("spool should be deleted after the terminal ack")
	}
}

// TestReplayUnackedSpools proves startup replay: an unacked spool left by a
// prior incarnation is delivered to the server and removed, and a second
// replay pass is a no-op (the spool is gone), so the terminal transition
// is applied exactly once.
func TestReplayUnackedSpools(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	spoolFile := newSpool(dir, "run-9")

	saveSpool(t, spoolFile, spoolState{
		Seq: 2,
		Events: []OutEvent{
			{Seq: 1, Type: eventTypeOutput, Data: "left unacked"},
			{Seq: 2, Type: eventTypeTerminal, Outcome: outcomeInterrupted},
		},
	})

	stub := newStubEvents()
	ctx := context.Background()

	err := replayUnackedSpools(ctx, stub, "session-2", dir)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if spoolExists(dir, "run-9") {
		t.Fatal("spool should be removed after terminal ack on replay")
	}

	if got := stub.ackedThrough("run-9"); got != 2 {
		t.Fatalf("acked_through after replay = %d, want 2", got)
	}

	if got := stub.terminalApplications("run-9"); got != 1 {
		t.Fatalf("terminal applied %d times on replay, want 1", got)
	}

	// A second startup replay finds nothing and does not re-apply.
	err = replayUnackedSpools(ctx, stub, "session-2", dir)
	if err != nil {
		t.Fatalf("second replay: %v", err)
	}

	if got := stub.terminalApplications("run-9"); got != 1 {
		t.Fatalf("second replay re-applied terminal: %d", got)
	}
}

// mustSend fails the test on a Send error.
func mustSend(t *testing.T, outbox *Outbox, ctx context.Context, event OutEvent) {
	t.Helper()

	err := outbox.Send(ctx, event)
	if err != nil {
		t.Fatalf("Send(%s): %v", event.Type, err)
	}
}

// mustFlush fails the test on a Flush error.
func mustFlush(t *testing.T, outbox *Outbox, ctx context.Context) {
	t.Helper()

	err := outbox.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

// loadSpool reads a run's persisted spool state.
func loadSpool(t *testing.T, dir, runID string) spoolState {
	t.Helper()

	state, err := newSpool(dir, runID).load()
	if err != nil {
		t.Fatalf("load spool: %v", err)
	}

	return state
}

// saveSpool persists a spool state for a test scenario.
func saveSpool(t *testing.T, spoolFile *spool, state spoolState) {
	t.Helper()

	err := spoolFile.save(state)
	if err != nil {
		t.Fatalf("save spool: %v", err)
	}
}

// hasGap reports whether a batch carries a RANGE TOMBSTONE.
func hasGap(events []OutEvent) bool {
	for _, event := range events {
		if event.Type == eventTypeGap {
			return true
		}
	}

	return false
}

// mountEvents mounts the stub's events handler at the agent events route.
func mountEvents(stub *stubEvents) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/events", stub.handler())

	return mux
}

// atomicBool is a tiny mutex-guarded flag for outage toggling.
type atomicBool struct {
	mu    sync.Mutex
	value bool
}

func (b *atomicBool) set(value bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.value = value
}

func (b *atomicBool) get() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.value
}
