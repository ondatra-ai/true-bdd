package store

import (
	"testing"
)

// runsInsertSQL is the full-column runs INSERT reused by the NOT NULL cases;
// split across lines so it stays within the line-length budget (SQL treats the
// added whitespace as insignificant).
const runsInsertSQL = `INSERT INTO runs
	(id, owner_id, command, fix, state, client_token, request_hash, next_seq, created_at, updated_at)
	VALUES(?,?,?,?,?,?,?,?,?,?)`

// seedAgentAndRuns inserts one agent and two runs via raw SQL so the FK
// parents exist for the prompt/event constraint checks.
func seedAgentAndRuns(t *testing.T, store Store) {
	t.Helper()

	must := func(err error) {
		t.Helper()

		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	must(store.Exec(
		`INSERT INTO agents(owner_id, pid, start_identity, created_at, last_local_seen) VALUES(?,?,?,?,?)`,
		testOwner1, 1000, "id-1", 1, 1,
	))

	for _, run := range []string{"r1", "r2"} {
		// r2 is terminal so ONE owner can hold both rows under the restored
		// partial UNIQUE(owner_id) WHERE state != 'terminal' index (plan §1.1).
		// The per-run event_seq / NOT NULL checks below are indifferent to run
		// state — this is the single flagged DDL-fixture change.
		state := "running"
		if run == "r2" {
			state = stateTerminal
		}

		must(store.Exec(runsInsertSQL,
			run, testOwner1, cmdVersion, 0, state, "tok-"+run, "hash-"+run, 3, 1, 1,
		))
		// One prompt event at seq 1 in EACH run — the per-run event sequence.
		must(store.Exec(
			`INSERT INTO events(run_id, seq, type, payload) VALUES(?,?,?,?)`,
			run, 1, evtPrompt, "{}",
		))
	}
}

func TestSchemaPromptEventSeqIsPerRun(t *testing.T) {
	path := tempDBPath(t)
	connA := requireStore(t, path)
	connB := requireStore(t, path) // a SECOND independent connection to the same file

	seedAgentAndRuns(t, connA)

	// A prompt bound to (r1, event_seq=1) is accepted.
	err := connA.Exec(
		`INSERT INTO prompts(run_id, prompt_id, event_seq, kind) VALUES(?,?,?,?)`,
		"r1", "p1", 1, kindChoice,
	)
	if err != nil {
		t.Fatalf("first prompt insert should succeed: %v", err)
	}

	// The SAME event_seq (1) in a DIFFERENT run is accepted (sequences are
	// per-run; global uniqueness would collide across runs).
	err = connB.Exec(
		`INSERT INTO prompts(run_id, prompt_id, event_seq, kind) VALUES(?,?,?,?)`,
		"r2", "p2", 1, kindChoice,
	)
	if err != nil {
		t.Fatalf("equal event_seq across a different run should succeed: %v", err)
	}

	// A duplicate (r1, event_seq=1) is REJECTED (UNIQUE(run_id, event_seq)).
	err = connB.Exec(
		`INSERT INTO prompts(run_id, prompt_id, event_seq, kind) VALUES(?,?,?,?)`,
		"r1", "p3", 1, kindChoice,
	)
	if err == nil {
		t.Fatal("duplicate event_seq within one run must be rejected")
	}
}

func TestSchemaNotNullDiscipline(t *testing.T) {
	path := tempDBPath(t)
	store := requireStore(t, path)

	seedAgentAndRuns(t, store)

	cases := []struct {
		name  string
		query string
		args  []any
	}{
		{
			name:  "null owner_id on runs",
			query: runsInsertSQL,
			args:  []any{"rn", nil, cmdVersion, 0, "running", "t", "h", 0, 1, 1},
		},
		{
			name:  "null state on runs",
			query: runsInsertSQL,
			args:  []any{"rn", testOwner1, cmdVersion, 0, nil, "t", "h", 0, 1, 1},
		},
		{
			name:  "null client_token on dispatch_receipts",
			query: `INSERT INTO dispatch_receipts(owner_id, client_token, request_hash, run_id, created_at) VALUES(?,?,?,?,?)`,
			args:  []any{testOwner1, nil, "h", "r1", 1},
		},
		{
			name:  "null seq on events",
			query: `INSERT INTO events(run_id, seq, type, payload) VALUES(?,?,?,?)`,
			args:  []any{"r1", nil, evtOutput, "{}"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := store.Exec(tc.query, tc.args...)
			if err == nil {
				t.Fatalf("%s must be rejected by NOT NULL", tc.name)
			}
		})
	}
}

func TestSchemaForeignKeysAndPragmasOn(t *testing.T) {
	path := tempDBPath(t)
	store := requireStore(t, path)

	// Foreign keys must be ON on every connection (plan §1.1) — otherwise the
	// composite FK/PK safety silently does nothing.
	foreignKeys, err := store.Scalar(`PRAGMA foreign_keys`)
	if err != nil {
		t.Fatalf("pragma foreign_keys: %v", err)
	}

	if foreignKeys != 1 {
		t.Fatalf("foreign_keys must be ON, got %d", foreignKeys)
	}

	// A migration ledger row exists (sequential checksummed migrations, not
	// bare CREATE IF NOT EXISTS).
	n, err := store.Scalar(`SELECT COUNT(*) FROM schema_migrations`)
	if err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}

	if n < 1 {
		t.Fatal("expected at least one recorded migration")
	}
}

func TestSchemaMigrationChecksumMismatchFailsOpen(t *testing.T) {
	path := tempDBPath(t)

	// First open applies + records the migrations.
	store := requireStore(t, path)

	err := store.Exec(`UPDATE schema_migrations SET checksum = ? WHERE version = ?`, "tampered", 1)
	if err != nil {
		t.Fatalf("tamper checksum: %v", err)
	}

	_ = store.Close()

	// Re-opening a DB whose recorded checksum no longer matches the embedded
	// migration must fail visibly (never silently recreate state).
	reopened, err := Open(path)
	if err == nil {
		if reopened != nil {
			_ = reopened.Close()
		}

		t.Fatal("re-open with a tampered migration checksum must fail")
	}
}
