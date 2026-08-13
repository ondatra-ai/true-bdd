package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// migration is one sequential, checksummed schema step (plan §1.1): the
// checksum is recorded so a later open with a tampered migration fails
// visibly rather than silently recreating state.
type migration struct {
	version int
	sql     string
}

// migrations is the ordered list applied under an exclusive transaction. The
// DDL is EXECUTABLE (explicit types, NOT NULL on every identity/state/
// sequence/receipt-key column, CHECK + conflict actions, meaningful FKs) —
// the constraints ARE the safety.
//
//nolint:gochecknoglobals // the embedded, checksummed migration ledger is package-wide by design
var migrations = []migration{
	{version: 1, sql: schemaV1},
}

const schemaV1 = `
CREATE TABLE agents (
  owner_id        TEXT    NOT NULL PRIMARY KEY,
  pid             INTEGER NOT NULL,
  start_identity  TEXT    NOT NULL,
  created_at      INTEGER NOT NULL,
  last_local_seen INTEGER NOT NULL
) STRICT;

CREATE TABLE runs (
  id                   TEXT    NOT NULL PRIMARY KEY,
  owner_id             TEXT    NOT NULL REFERENCES agents(owner_id),
  command              TEXT    NOT NULL,
  story_id             TEXT,
  fix                  INTEGER NOT NULL DEFAULT 0,
  state                TEXT    NOT NULL,
  engine_outcome       TEXT,
  finalization_ok      INTEGER,
  exit_code            INTEGER,
  signal               TEXT,
  classification       TEXT,
  error_detail         TEXT,
  client_token         TEXT    NOT NULL,
  request_hash         TEXT    NOT NULL,
  next_seq             INTEGER NOT NULL DEFAULT 1,
  child_pgid           INTEGER,
  child_start_identity TEXT,
  created_at           INTEGER NOT NULL,
  updated_at           INTEGER NOT NULL
) STRICT;

-- One-nonterminal-run-per-owner (plan §1.1/§1.4): a partial UNIQUE index is the
-- DB-level safety backing the Dispatch admission's SELECT-then-INSERT — it
-- catches the cross-connection race two live CLIs on a shared DB could slip past
-- the app-level COUNT check, and never depends on the losing writer's commit
-- timing. NULL owner/state are rejected by NOT NULL, so the predicate is total.
CREATE UNIQUE INDEX one_nonterminal_run_per_owner
  ON runs(owner_id) WHERE state != 'terminal';

CREATE TABLE events (
  run_id  TEXT    NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  seq     INTEGER NOT NULL,
  type    TEXT    NOT NULL,
  payload TEXT    NOT NULL DEFAULT '{}',
  PRIMARY KEY(run_id, seq)
) STRICT;

CREATE TABLE prompts (
  run_id              TEXT    NOT NULL,
  prompt_id           TEXT    NOT NULL,
  event_seq           INTEGER NOT NULL,
  kind                TEXT    NOT NULL,
  payload             TEXT,
  answer              TEXT,
  answer_seq          INTEGER,
  accepted_at         INTEGER,
  delivery_started_at INTEGER,
  consumed_at         INTEGER,
  delivery_error      TEXT,
  PRIMARY KEY(run_id, prompt_id),
  UNIQUE(run_id, event_seq),
  FOREIGN KEY(run_id) REFERENCES runs(id) ON DELETE CASCADE,
  FOREIGN KEY(run_id, event_seq) REFERENCES events(run_id, seq)
) STRICT;

CREATE TABLE dispatch_receipts (
  owner_id     TEXT    NOT NULL,
  client_token TEXT    NOT NULL,
  request_hash TEXT    NOT NULL,
  run_id       TEXT    NOT NULL,
  created_at   INTEGER NOT NULL,
  PRIMARY KEY(owner_id, client_token)
) STRICT;

CREATE TABLE reconcile_lease (
  id              INTEGER NOT NULL PRIMARY KEY CHECK(id = 1),
  holder_owner_id TEXT    NOT NULL,
  fencing_token   INTEGER NOT NULL,
  acquired_at     INTEGER NOT NULL,
  expires_at      INTEGER NOT NULL
) STRICT;
`

func checksum(s string) string {
	sum := sha256.Sum256([]byte(s))

	return hex.EncodeToString(sum[:])
}

// migrate applies unrecorded migrations under an exclusive transaction and
// verifies the checksums of already-recorded ones. A recorded checksum that no
// longer matches the embedded migration fails the open (plan §1.1).
func (db *DB) migrate() error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	conn, err := db.sql.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	_, err = conn.ExecContext(ctx, `BEGIN EXCLUSIVE`)
	if err != nil {
		return fmt.Errorf("migrate: begin exclusive: %w", err)
	}

	err = db.migrateLocked(ctx, conn)
	if err != nil {
		_, _ = conn.ExecContext(ctx, `ROLLBACK`)

		return err
	}

	_, err = conn.ExecContext(ctx, `COMMIT`)
	if err != nil {
		return fmt.Errorf("migrate: commit: %w", err)
	}

	return nil
}

func (db *DB) migrateLocked(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
      version  INTEGER NOT NULL PRIMARY KEY,
      checksum TEXT    NOT NULL
    ) STRICT`)
	if err != nil {
		return fmt.Errorf("migrate: ensure ledger: %w", err)
	}

	recorded, err := readLedger(ctx, conn)
	if err != nil {
		return err
	}

	for _, mig := range migrations {
		want := checksum(mig.sql)

		if got, ok := recorded[mig.version]; ok {
			if got != want {
				return fmt.Errorf("%w: version %d (recorded %q, embedded %q)", errChecksumMismatch, mig.version, got, want)
			}

			continue
		}

		err = applyMigration(ctx, conn, mig, want)
		if err != nil {
			return err
		}
	}

	return nil
}

// readLedger loads the recorded migration checksums keyed by version.
func readLedger(ctx context.Context, conn *sql.Conn) (map[int]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrate: read ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()

	recorded := map[int]string{}

	for rows.Next() {
		var (
			ver int
			sum string
		)

		err = rows.Scan(&ver, &sum)
		if err != nil {
			return nil, fmt.Errorf("migrate: scan ledger: %w", err)
		}

		recorded[ver] = sum
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("migrate: ledger rows: %w", err)
	}

	return recorded, nil
}

// applyMigration applies one migration's SQL and records its checksum.
func applyMigration(ctx context.Context, conn *sql.Conn, mig migration, sum string) error {
	_, err := conn.ExecContext(ctx, mig.sql)
	if err != nil {
		return fmt.Errorf("migrate: apply version %d: %w", mig.version, err)
	}

	_, err = conn.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, checksum) VALUES(?, ?)`, mig.version, sum)
	if err != nil {
		return fmt.Errorf("migrate: record version %d: %w", mig.version, err)
	}

	return nil
}

// errChecksumMismatch fails an open whose recorded migration was tampered.
var errChecksumMismatch = errors.New("migrate: checksum mismatch — refusing to open")

// errNoRows is a convenience for callers distinguishing missing rows.
var errNoRows = errors.New("no rows")
