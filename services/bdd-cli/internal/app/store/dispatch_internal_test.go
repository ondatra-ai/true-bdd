package store

// Dispatch transaction semantics: see Dispatch's doc in dispatch.go. This
// file also covers receipt retention (latest 200) and the run_pruned
// contract.

import (
	"fmt"
	"testing"
)

func seedOwner(t *testing.T, store Store, ownerID string) {
	t.Helper()

	err := store.Exec(
		`INSERT INTO agents(owner_id, pid, start_identity, created_at, last_local_seen) VALUES(?,?,?,?,?)`,
		ownerID, 2000, "id-"+ownerID, 1, 1,
	)
	if err != nil {
		t.Fatalf("seed owner: %v", err)
	}
}

func terminate(t *testing.T, store Store, runID string) {
	t.Helper()

	res := store.AppendEvent(runID, Event{Type: evtTerminal, Outcome: "ok", Control: true})
	if res.Rejected {
		t.Fatalf("terminal append rejected for %s", runID)
	}
}

func TestDispatchCreatedDedupedAndConflict(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)

	created := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: cmdVersion, ClientToken: "T", RequestHash: "H1",
	})
	if created.Kind != dispatchCreated || created.RunID == "" {
		t.Fatalf("first dispatch: got %+v, want created with a run id", created)
	}

	// Exact retry (same owner, token, hash) ⇒ the ORIGINAL run, deduped.
	dedup := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: cmdVersion, ClientToken: "T", RequestHash: "H1",
	})
	if dedup.Kind != "deduped" || dedup.RunID != created.RunID {
		t.Fatalf("exact retry: got %+v, want deduped run %s", dedup, created.RunID)
	}

	// Token reuse with DIFFERENT args (hash) ⇒ conflict.
	badRetry := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: cmdVersion, ClientToken: "T", RequestHash: "H2",
	})
	if badRetry.Kind != "conflict" {
		t.Fatalf("token reuse with different args: got %+v, want conflict", badRetry)
	}
}

func TestDispatchPartialUniqueOneNonterminalPerOwner(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)

	first := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: cmdVersion, ClientToken: "A", RequestHash: "HA",
	})
	if first.Kind != dispatchCreated {
		t.Fatalf("first: %+v", first)
	}

	// A distinct token while a nonterminal run exists ⇒ conflict (one
	// nonterminal run per owner — the partial UNIQUE(owner_id) WHERE state !=
	// 'terminal').
	second := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: cmdUsRefine, ClientToken: "B", RequestHash: "HB",
	})
	if second.Kind != "conflict" {
		t.Fatalf("second (distinct token, run still active): %+v, want conflict", second)
	}

	// A SIBLING owner is never blocked by another owner's active run.
	seedOwner(t, store, testOwner2)

	sibling := store.Dispatch(DispatchRequest{
		OwnerID: testOwner2, Command: cmdVersion, ClientToken: "C", RequestHash: "HC",
	})
	if sibling.Kind != dispatchCreated {
		t.Fatalf("sibling owner should not be blocked: %+v", sibling)
	}

	// Once the first run terminates, the owner may dispatch again.
	terminate(t, store, first.RunID)

	third := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: cmdVersion, ClientToken: "D", RequestHash: "HD",
	})
	if third.Kind != dispatchCreated {
		t.Fatalf("after terminal, a fresh dispatch should be created: %+v", third)
	}
}

func TestDispatchRejectsInvalidCommand(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)

	bad := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: "version; echo pwned", ClientToken: "T", RequestHash: "H",
	})
	if bad.Kind != "invalid" {
		t.Fatalf("shell-ish command: %+v, want invalid", bad)
	}
}

func TestDispatchReceiptRetentionAndRunPruned(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)

	// Create the first run and remember its token; then churn enough terminal
	// runs that its RUN ROW is retention-pruned (latest 50) while its DISPATCH
	// RECEIPT is retained (latest 200).
	firstToken := "tok-0"

	first := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: cmdVersion, ClientToken: firstToken, RequestHash: "h-0",
	})
	if first.Kind != dispatchCreated {
		t.Fatalf("run 0: %+v", first)
	}

	terminate(t, store, first.RunID)

	for runNum := 1; runNum <= 52; runNum++ {
		tok := fmt.Sprintf("tok-%d", runNum)

		res := store.Dispatch(DispatchRequest{
			OwnerID: testOwner1, Command: cmdVersion, ClientToken: tok, RequestHash: "h-" + tok,
		})
		if res.Kind != dispatchCreated {
			t.Fatalf("run %d: %+v", runNum, res)
		}

		terminate(t, store, res.RunID)
	}

	store.EnforceRetention()

	// The oldest run row is gone but its receipt survives: an EXACT retry of
	// its token returns the ORIGINAL run id, flagged Pruned.
	retry := store.Dispatch(DispatchRequest{
		OwnerID: testOwner1, Command: cmdVersion, ClientToken: firstToken, RequestHash: "h-0",
	})
	if retry.RunID != first.RunID {
		t.Fatalf("pruned-run exact retry: got run %s, want original %s", retry.RunID, first.RunID)
	}

	if !retry.Pruned {
		t.Fatal("exact retry of a pruned run must report Pruned=true (run_pruned read then 404s)")
	}

	// Retention kept the latest 50 terminal runs.
	if got := store.TerminalRunCount(); got != 50 {
		t.Fatalf("terminal run retention: got %d, want 50", got)
	}
}
