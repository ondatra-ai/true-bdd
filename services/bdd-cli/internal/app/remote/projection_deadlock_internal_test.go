package remote

import (
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/store"
)

// TestProjectionConcurrentReadsDoNotDeadlock is the inverted regression for the
// reproduced second-DB deadlock (finding 6): the codex probe pinned all four
// reader connections with four outer `allRuns` cursors whose nested
// `hasPendingPrompt` queries then waited forever. The rewrite runs each
// projection in ONE read transaction and derives `answerable` from an EXISTS
// subquery — no nested query on a second connection — so far MORE concurrent
// readers than the pool size all complete promptly.
func TestProjectionConcurrentReadsDoNotDeadlock(t *testing.T) {
	t.Parallel()

	db, read, session, runID := newProjectionFixture(t)

	// A pending prompt makes the answerable EXISTS subquery non-trivial, and a
	// second run gives the history projection more than one row.
	db.AppendEvent(runID, store.EventInput{Type: eventTypeLockAcquired, Control: true})
	db.AppendEvent(runID, store.EventInput{
		Type: eventTypePrompt, PromptID: "p1", Kind: kindChoice,
		Payload: `{"actions":["apply","refine","exit"]}`, Control: true,
	})

	// Far more concurrent readers than maxReadConns(4): the reproduced deadlock
	// needed only four outer reads to pin the whole pool.
	const readers = 16

	done := make(chan struct{}, readers)

	for range readers {
		go func() {
			for range 25 {
				_ = read.sessionStatus(session)
				_, _, _ = read.runDetail(session, runID)
				_ = read.sessionStatus(session)
			}

			done <- struct{}{}
		}()
	}

	deadline := time.After(20 * time.Second)

	for finished := range readers {
		select {
		case <-done:
		case <-deadline:
			t.Fatalf("projection reads deadlocked: only %d/%d readers finished", finished, readers)
		}
	}
}
