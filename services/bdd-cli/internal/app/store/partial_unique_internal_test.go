package store

import "testing"

// TestPartialUniqueRejectsSecondNonterminalRun proves the restored partial
// UNIQUE index from migrations.go (schemaV1) rejects a second nonterminal run
// per owner at the DB level; a terminal run may still coexist with one.
func TestPartialUniqueRejectsSecondNonterminalRun(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)

	insert := func(id, state string) error {
		return store.Exec(
			`INSERT INTO runs(id, owner_id, command, fix, state, client_token, request_hash, next_seq, created_at, updated_at)
			 VALUES(?,?,?,?,?,?,?,?,?,?)`,
			id, testOwner1, cmdVersion, 0, state, "tok-"+id, "hash-"+id, 1, 1, 1,
		)
	}

	err := insert("ra", "running")
	if err != nil {
		t.Fatalf("first nonterminal insert must succeed: %v", err)
	}

	err = insert("rb", "running")
	if err == nil {
		t.Fatal("a second nonterminal run for one owner must be rejected by the partial unique index")
	}

	// A terminal run for the same owner is allowed — the predicate excludes it.
	err = insert("rc", stateTerminal)
	if err != nil {
		t.Fatalf("a terminal run must coexist with a nonterminal one: %v", err)
	}

	// Terminating the nonterminal run frees the slot for a new nonterminal run.
	err = store.Exec(`UPDATE runs SET state = 'terminal' WHERE id = 'ra'`)
	if err != nil {
		t.Fatalf("terminate ra: %v", err)
	}

	err = insert("rd", "running")
	if err != nil {
		t.Fatalf("after the prior run terminates, a fresh nonterminal run must be allowed: %v", err)
	}
}
