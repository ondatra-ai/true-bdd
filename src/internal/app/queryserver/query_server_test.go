package queryserver_test

// Query-server client + worker pool (plan §2, critique §5). Typed HTTP
// errors, re-register with the SAME client-owned session id on poll
// 404/stale-epoch, monotonic epoch, and the bounded worker pool (mutations
// serialized, ≤4 concurrent reads, 1 inventory scan, priority answer >
// dispatch > reads).

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/app/queryserver"
)

func TestClassifyTypedStatuses(t *testing.T) {
	cases := map[int]string{
		204: "",
		200: "",
		404: "session_gone",
		409: "", // a CLI domain conflict is not a reconnect signal
		413: "reply_too_large",
		502: "invalid_reply",
		503: "capacity",
		429: "capacity",
	}
	for status, want := range cases {
		if got := queryserver.ClassifyStatus(status); got != want {
			t.Fatalf("ClassifyStatus(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestReconnectReRegistersWithSameSessionId(t *testing.T) {
	// Poll 404 / stale epoch ⇒ re-register using the SAME client-owned
	// session id (critique §5).
	for _, signal := range []string{"session_gone", "stale_epoch"} {
		decision := queryserver.ReconnectFor(signal)
		if !decision.ReRegister || !decision.SameSessionID {
			t.Fatalf("ReconnectFor(%q) = %+v, want re-register with the same session id", signal, decision)
		}
	}

	// A domain conflict / normal completion does NOT trigger a re-register.
	if decision := queryserver.ReconnectFor(""); decision.ReRegister {
		t.Fatalf("a normal completion must not re-register: %+v", decision)
	}

	// A transport failure re-registers with backoff (capped full-jitter).
	if decision := queryserver.ReconnectFor("transport"); !decision.ReRegister || !decision.Backoff {
		t.Fatalf("transport failure must re-register with backoff: %+v", decision)
	}
}

func TestEpochStrictlyMonotonic(t *testing.T) {
	e1 := queryserver.NextEpoch(0)
	e2 := queryserver.NextEpoch(e1)

	if e2 <= e1 || e1 <= 0 {
		t.Fatalf("epoch must be strictly monotonic: 0 -> %d -> %d", e1, e2)
	}
}

func TestWorkerPoolBoundsReadsAndInventory(t *testing.T) {
	pool := queryserver.NewWorkerPool(queryserver.PoolSettings{MaxReads: 4, MaxInventory: 1})

	for i := range 6 {
		pool.Submit(readID(i), queryserver.ClassRead)
	}

	pool.Submit("inv-1", queryserver.ClassInventory)
	pool.Submit("inv-2", queryserver.ClassInventory)

	batch := pool.RunnableNow()

	reads, invs := 0, 0

	for _, id := range batch {
		switch id[:3] {
		case "rd-":
			reads++
		case "inv":
			invs++
		}
	}

	if reads > 4 {
		t.Fatalf("at most 4 concurrent reads, got %d", reads)
	}

	if invs > 1 {
		t.Fatalf("at most 1 concurrent inventory scan, got %d", invs)
	}
}

func TestWorkerPoolMutationsSerializedWithAnswerPriority(t *testing.T) {
	pool := queryserver.NewWorkerPool(queryserver.PoolSettings{MaxReads: 4, MaxInventory: 1})

	// A read, a dispatch, and an answer are all pending.
	pool.Submit("rd-0", queryserver.ClassRead)
	pool.Submit("dispatch-1", queryserver.ClassDispatch)
	pool.Submit("answer-1", queryserver.ClassAnswer)

	batch := pool.RunnableNow()

	// Exactly ONE mutation runs at a time, and the ANSWER wins over the
	// dispatch (priority answer > dispatch > reads).
	if !contains(batch, "answer-1") {
		t.Fatalf("the answer mutation must be scheduled: %v", batch)
	}

	if contains(batch, "dispatch-1") {
		t.Fatalf("mutations are serialized — the dispatch must wait behind the answer: %v", batch)
	}

	// Completing the answer frees the mutation slot for the dispatch.
	pool.Complete("answer-1")

	if next := pool.RunnableNow(); !contains(next, "dispatch-1") {
		t.Fatalf("after the answer completes, the dispatch must run: %v", next)
	}
}

func TestWorkerPoolDocAndChatAreSeparateLanesFromRunMutations(t *testing.T) {
	// A slow chat turn or a doc write must NEVER occupy the run-mutation slot
	// (plan Slice 0/5, r3 #6): submit one of each class and confirm all four
	// run concurrently — not serialized behind each other.
	pool := queryserver.NewWorkerPool(queryserver.PoolSettings{MaxReads: 4, MaxInventory: 1})

	pool.Submit("dispatch-1", queryserver.ClassDispatch)
	pool.Submit("doc-1", queryserver.ClassDoc)
	pool.Submit("chat-1", queryserver.ClassChat)

	batch := pool.RunnableNow()

	for _, id := range []string{"dispatch-1", "doc-1", "chat-1"} {
		if !contains(batch, id) {
			t.Fatalf("%s must run concurrently with the others (separate lanes): %v", id, batch)
		}
	}

	// A second chat/doc item waits behind its OWN lane bound (default 1 chat,
	// 4 docs), independent of the dispatch/answer mutation lane.
	pool.Submit("chat-2", queryserver.ClassChat)

	if contains(pool.RunnableNow(), "chat-2") {
		t.Fatalf("a second concurrent chat turn must wait behind the chat lane bound")
	}

	pool.Complete("chat-1")

	if !contains(pool.RunnableNow(), "chat-2") {
		t.Fatalf("completing chat-1 must free the chat lane for chat-2")
	}
}

func readID(i int) string {
	return "rd-" + string(rune('0'+i))
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}

	return false
}
