package remote

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

// errStubDropped simulates a lost events response (server unreachable or a
// dropped reply). err113 is relaxed for test files.
var errStubDropped = errors.New("stub server dropped the response")

// postDecision scripts how the stub handles one events POST.
type postDecision int

const (
	// decideOK commits and replies.
	decideOK postDecision = iota
	// decideDropPre fails BEFORE committing (server never saw it).
	decideDropPre
	// decideDropPost commits THEN fails the reply (lost ack after commit).
	decideDropPost
)

// runLedger models the server's per-run event store with INSERT OR IGNORE
// semantics: an event's covered seq range is applied exactly once, so the
// terminal transition and the contiguous acked_through watermark are
// idempotent under ret/replay.
type runLedger struct {
	covered         map[int]bool
	applied         map[int]bool
	terminalApplied int
	answersConsumed int
}

func newRunLedger() *runLedger {
	return &runLedger{covered: make(map[int]bool), applied: make(map[int]bool)}
}

// apply folds a batch into the ledger, covering each new event's seq span
// exactly once (plan §3.3 INSERT OR IGNORE + §3.6 gap span coverage).
func (l *runLedger) apply(events []OutEvent, answersConsumed int) {
	for _, event := range events {
		if l.applied[event.Seq] {
			continue
		}

		l.applied[event.Seq] = true

		for seq := event.Seq; seq <= event.SpanEnd(); seq++ {
			l.covered[seq] = true
		}

		if event.Type == eventTypeTerminal {
			l.terminalApplied++
		}
	}

	if answersConsumed > l.answersConsumed {
		l.answersConsumed = answersConsumed
	}
}

// ackedThrough is the largest N with seqs 1..N all covered.
func (l *runLedger) ackedThrough() int {
	seq := 0
	for l.covered[seq+1] {
		seq++
	}

	return seq
}

// stubEvents is a deterministic events endpoint: an in-process eventPoster
// and an equivalent HTTP handler, sharing one set of run ledgers. A delay
// widens the in-flight window so a broken (non-serialized) sender would be
// caught by maxInFlight > 1.
type stubEvents struct {
	mu          sync.Mutex
	ledgers     map[string]*runLedger
	decide      func(attempt int) postDecision
	delay       time.Duration
	attempts    int
	inFlight    int
	maxInFlight int
}

func newStubEvents() *stubEvents {
	return &stubEvents{ledgers: make(map[string]*runLedger)}
}

// PostEvents implements eventPoster with the scripted drop policy.
func (s *stubEvents) PostEvents(_ context.Context, req EventsRequest) (EventsResponse, error) {
	s.enter()
	defer s.leave()

	if s.delay > 0 {
		time.Sleep(s.delay)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.attempts++
	decision := decideOK

	if s.decide != nil {
		decision = s.decide(s.attempts)
	}

	ledger := s.ledgerFor(req.RunID)

	if decision == decideDropPre {
		return EventsResponse{}, errStubDropped
	}

	ledger.apply(req.Events, req.AnswersConsumed)

	if decision == decideDropPost {
		return EventsResponse{}, errStubDropped
	}

	return EventsResponse{
		AckedThrough:           ledger.ackedThrough(),
		AnswersConsumedThrough: ledger.answersConsumed,
	}, nil
}

// ledgerFor returns (creating) the ledger for a run.
func (s *stubEvents) ledgerFor(runID string) *runLedger {
	ledger, ok := s.ledgers[runID]
	if !ok {
		ledger = newRunLedger()
		s.ledgers[runID] = ledger
	}

	return ledger
}

// enter/leave track the in-flight window to detect a non-serialized sender.
func (s *stubEvents) enter() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inFlight++
	if s.inFlight > s.maxInFlight {
		s.maxInFlight = s.inFlight
	}
}

func (s *stubEvents) leave() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inFlight--
}

// handler mounts the stub as an HTTP events endpoint for tests that need a
// real ServerClient over httptest (plan §4.6 "via stub HTTP server").
func (s *stubEvents) handler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var req EventsRequest

		err := json.NewDecoder(request.Body).Decode(&req)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)

			return
		}

		resp, postErr := s.PostEvents(request.Context(), req)
		if postErr != nil {
			writer.WriteHeader(http.StatusInternalServerError)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(resp)
	}
}

// ackedThrough exposes a run's contiguous watermark for assertions.
func (s *stubEvents) ackedThrough(runID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ledgerFor(runID).ackedThrough()
}

// terminalApplications reports how many times the terminal event was
// applied (must be 1 despite retries/replay).
func (s *stubEvents) terminalApplications(runID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ledgerFor(runID).terminalApplied
}

// answersConsumedThrough exposes the recorded answers watermark.
func (s *stubEvents) answersConsumedThrough(runID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ledgerFor(runID).answersConsumed
}
