package store

// Per-run DB-WRITER ACTOR (critique §9): all producers submit to one bounded
// channel; the writer allocates next_seq and inserts in one transaction, so
// concurrent producers get UNIQUE, CONTIGUOUS sequences, control events are
// never dropped, and terminal stays last. Plus local retention (critique
// §10): a 256 KiB logical cap with head+tail + ONE gap event, control events
// retained independently, and nonterminal runs never pruned.

import (
	"strings"
	"sync"
	"testing"
)

func newRun(t *testing.T, store Store, token string) string {
	t.Helper()

	res := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: cmdVersion, ClientToken: token, RequestHash: "h-" + token,
	})
	if res.Kind != dispatchCreated {
		t.Fatalf("dispatch: %+v", res)
	}

	return res.RunID
}

func TestWriterActorAllocatesUniqueContiguousSeqs(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	runID := newRun(t, store, "w")

	const producers = 32

	var mutex sync.Mutex

	seqs := make([]int, 0, producers)

	var waitGroup sync.WaitGroup

	for range producers {
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			res := store.AppendEvent(runID, Event{Type: evtOutput, Stream: "stdout", Data: "chunk\n"})
			if res.Rejected {
				return
			}

			mutex.Lock()
			defer mutex.Unlock()

			seqs = append(seqs, res.Seq)
		}()
	}

	waitGroup.Wait()

	if len(seqs) != producers {
		t.Fatalf("appended %d events, want %d", len(seqs), producers)
	}

	seen := map[int]bool{}
	maxSeq := 0

	for _, seq := range seqs {
		if seen[seq] {
			t.Fatalf("duplicate seq %d — the writer actor must serialize allocation", seq)
		}

		seen[seq] = true

		if seq > maxSeq {
			maxSeq = seq
		}
	}

	// Contiguous 1..producers (no holes) under concurrency.
	for seq := 1; seq <= producers; seq++ {
		if !seen[seq] {
			t.Fatalf("missing seq %d — sequence must be contiguous", seq)
		}
	}
}

func TestWriterControlEventsSynchronousAndTerminalLast(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	runID := newRun(t, store, "c")

	store.AppendEvent(runID, Event{Type: evtLockAcquired, Control: true})
	store.AppendEvent(runID, Event{Type: evtOutput, Data: "working\n"})
	store.AppendEvent(runID, Event{Type: evtPrompt, PromptID: "p1", Kind: kindChoice, Control: true})
	store.AppendEvent(runID, Event{Type: evtOutput, Data: "more\n"})

	term := store.AppendEvent(runID, Event{Type: evtTerminal, Outcome: "ok", Control: true})
	if term.Rejected {
		t.Fatal("terminal control event must never be dropped")
	}

	events := store.Events(runID)
	if len(events) == 0 {
		t.Fatal("no events recorded")
	}

	// Terminal stays LAST (after all output/prompt drains).
	if last := events[len(events)-1]; last.Type != evtTerminal {
		t.Fatalf("terminal must be the last event, got %q", last.Type)
	}

	// The two control events survive.
	for _, want := range []string{evtLockAcquired, evtPrompt, evtTerminal} {
		if !hasType(events, want) {
			t.Fatalf("control event %q must be retained", want)
		}
	}
}

func TestRetentionByteBudgetHeadTailAndGap(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	runID := newRun(t, store, "b")

	// A control event first, then far more than 256 KiB of output, then a
	// terminal. The middle output must collapse to ONE gap; controls survive.
	store.AppendEvent(runID, Event{Type: evtLockAcquired, Control: true})

	big := strings.Repeat("x", 8*1024)

	for range 64 { // 512 KiB > 256 KiB cap
		store.AppendEvent(runID, Event{Type: evtOutput, Stream: "stdout", Data: big})
	}

	store.AppendEvent(runID, Event{Type: evtTerminal, Outcome: "ok", Control: true})
	store.EnforceByteBudget(runID)

	events := store.Events(runID)
	gaps := 0

	var gap Event

	for _, ev := range events {
		if ev.Type == evtGap {
			gaps++
			gap = ev
		}
	}

	if gaps != 1 {
		t.Fatalf("byte-budget pruning must leave exactly ONE gap, got %d", gaps)
	}

	if gap.ThroughSeq == 0 || gap.DroppedBytes == 0 {
		t.Fatalf("gap must carry through_seq + dropped_bytes: %+v", gap)
	}

	// Control events are retained independently of output pruning.
	for _, want := range []string{evtLockAcquired, evtTerminal} {
		if !hasType(events, want) {
			t.Fatalf("control event %q must survive retention", want)
		}
	}
}

func TestRetentionNeverPrunesNonterminalRuns(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	live := newRun(t, store, "live") // stays nonterminal

	store.EnforceRetention()

	if _, ok := store.RunView(live); !ok {
		t.Fatal("a nonterminal run must never be pruned")
	}
}

func hasType(events []Event, typ string) bool {
	for _, ev := range events {
		if ev.Type == typ {
			return true
		}
	}

	return false
}
