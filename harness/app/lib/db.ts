/**
 * SQLite access for the harness server (plan §3.1/§3.3). `openDatabase`
 * opens + migrates a connection at a given path; the process-wide
 * singleton `db()` opens one lazily (env TRUE_BDD_HARNESS_DB, default
 * harness/data/harness.db), switches it to WAL, migrates idempotently,
 * and arms retention (startup + periodic prune). Keeping `openDatabase`
 * separate lets the vitest suites open their OWN temp-file connections
 * (even two racing the same file) against the real schema and migrations
 * without a running Next server (plan §4.6).
 *
 * All statements are synchronous and short; none is held across an await
 * (better-sqlite3 is blocking, so this serialises naturally in Next's
 * single Node process). `serverExternalPackages: ["better-sqlite3"]` in
 * next.config.ts keeps the native addon required by Node at runtime.
 */

import fs from "node:fs";
import path from "node:path";

import Database from "better-sqlite3";

import { startRetention } from "./retention-runtime";

// Every DDL statement is idempotent (CREATE ... IF NOT EXISTS), so
// re-running MIGRATIONS against an existing DB — a server restart
// (P7/A8) or a repeated openDatabase in tests — is a safe no-op.
const MIGRATIONS = `
CREATE TABLE IF NOT EXISTS sessions (
  id               TEXT PRIMARY KEY,
  incarnation_id   TEXT NOT NULL UNIQUE,
  folder           TEXT NOT NULL,
  canonical_folder TEXT NOT NULL,
  pid              INTEGER NOT NULL,
  version          TEXT,
  created_at       INTEGER NOT NULL,
  last_poll_at     INTEGER NOT NULL,
  scan_epoch       INTEGER NOT NULL DEFAULT 0,
  want_inventory   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS runs (
  id               TEXT PRIMARY KEY,
  session_id       TEXT NOT NULL,
  command          TEXT NOT NULL,
  story_id         TEXT,
  fix              INTEGER NOT NULL DEFAULT 0,
  state            TEXT NOT NULL,
  outcome          TEXT,
  error_detail     TEXT,
  client_token     TEXT NOT NULL,
  answer_seq       INTEGER NOT NULL DEFAULT 0,
  answers_consumed INTEGER NOT NULL DEFAULT 0,
  created_at       INTEGER NOT NULL,
  updated_at       INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS runs_client_token ON runs(client_token);
CREATE UNIQUE INDEX IF NOT EXISTS runs_one_active_per_session
  ON runs(session_id) WHERE state != 'terminal';
CREATE INDEX IF NOT EXISTS runs_by_session ON runs(session_id);

CREATE TABLE IF NOT EXISTS events (
  run_id     TEXT NOT NULL,
  seq        INTEGER NOT NULL,
  type       TEXT NOT NULL,
  data       TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (run_id, seq)
);

CREATE TABLE IF NOT EXISTS prompts (
  run_id     TEXT NOT NULL,
  prompt_id  TEXT NOT NULL,
  kind       TEXT NOT NULL,
  payload    TEXT NOT NULL,
  seq        INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (run_id, prompt_id)
);

CREATE TABLE IF NOT EXISTS answers (
  run_id     TEXT NOT NULL,
  prompt_id  TEXT NOT NULL,
  value      TEXT NOT NULL,
  answer_seq INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (run_id, prompt_id)
);

CREATE TABLE IF NOT EXISTS inventory (
  canonical_folder TEXT PRIMARY KEY,
  generation       INTEGER NOT NULL DEFAULT 0,
  scan_epoch       INTEGER NOT NULL DEFAULT 0,
  snapshot         TEXT NOT NULL,
  client_token     TEXT,
  updated_at       INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS folder_epochs (
  canonical_folder TEXT PRIMARY KEY,
  issued_epoch     INTEGER NOT NULL DEFAULT 0
);

-- Remote gap-tombstone coverage stored SEPARATELY from the events primary
-- key (plan §3.6 / finding 1): a range tombstone [from_seq, to_seq] whose
-- from_seq collides with an already-committed event row would be lost to
-- INSERT OR IGNORE, permanently stranding the contiguous ACK. Recording
-- the interval here lets contiguousAck cross the collapsed span regardless
-- of which event row occupies the colliding seq.
CREATE TABLE IF NOT EXISTS gap_coverage (
  run_id   TEXT NOT NULL,
  from_seq INTEGER NOT NULL,
  to_seq   INTEGER NOT NULL,
  PRIMARY KEY (run_id, from_seq)
);

-- Compact terminal receipts that OUTLIVE run pruning (plan §3.6 / finding
-- 1): once a run is terminal, its final acked_through watermark is kept
-- here so a late replay of a terminal whose commit response was lost is
-- still acknowledged (and the remote's spool deletes) even after the run
-- row itself has been pruned.
CREATE TABLE IF NOT EXISTS terminal_receipts (
  run_id                   TEXT PRIMARY KEY,
  acked_through            INTEGER NOT NULL,
  answers_consumed_through INTEGER NOT NULL,
  updated_at               INTEGER NOT NULL
);

-- Server-issued inventory scan tickets (plan §3.3 / finding 2): every
-- epoch handed to a session (on register or a want_inventory poll) is
-- persisted with the (folder, session) it was issued to, so an upload can
-- be proven to carry a ticket that THIS session was granted for THIS
-- folder rather than an arbitrary attacker-chosen epoch.
CREATE TABLE IF NOT EXISTS inventory_tickets (
  canonical_folder TEXT NOT NULL,
  session_id       TEXT NOT NULL,
  scan_epoch       INTEGER NOT NULL,
  issued_at        INTEGER NOT NULL,
  PRIMARY KEY (canonical_folder, scan_epoch)
);

-- Inventory upload ledger (plan §3.3 / finding 2): the (folder, token)
-- record of every accepted upload, so an EXACT retry returns its recorded
-- generation and a mismatched retry (same token, different epoch/payload)
-- is rejected rather than silently promoting a new generation.
CREATE TABLE IF NOT EXISTS inventory_uploads (
  canonical_folder TEXT NOT NULL,
  client_token     TEXT NOT NULL,
  scan_epoch       INTEGER NOT NULL,
  generation       INTEGER NOT NULL,
  payload_hash     TEXT NOT NULL,
  updated_at       INTEGER NOT NULL,
  PRIMARY KEY (canonical_folder, client_token)
);
`;

// Additive columns on `runs` (plan §3.2/§3.3 / findings 1, 7). Each ALTER
// is idempotent: SQLite rejects a duplicate column, which we swallow, so a
// server restart (P7/A8) or a repeated openDatabase re-runs safely.
//
// - applied_through: the contiguous seq watermark whose state transitions
//   have already been applied, so a redelivered event never re-applies a
//   transition (finding 1).
// - engine_outcome / finalization_ok / exit_code / signal: the full
//   remote-synthesized terminal envelope kept alongside the `outcome`
//   classification (finding 7); the badge stays `outcome`, the diagnostic
//   fields ride along for the run view.
const RUN_COLUMNS: { name: string; ddl: string }[] = [
  { name: "applied_through", ddl: "ALTER TABLE runs ADD COLUMN applied_through INTEGER NOT NULL DEFAULT 0" },
  { name: "engine_outcome", ddl: "ALTER TABLE runs ADD COLUMN engine_outcome TEXT" },
  { name: "finalization_ok", ddl: "ALTER TABLE runs ADD COLUMN finalization_ok INTEGER" },
  { name: "exit_code", ddl: "ALTER TABLE runs ADD COLUMN exit_code INTEGER" },
  { name: "signal", ddl: "ALTER TABLE runs ADD COLUMN signal TEXT" },
];

/** Applies the additive `runs` columns, ignoring already-present ones. */
function migrateRunColumns(db: Database.Database): void {
  for (const column of RUN_COLUMNS) {
    try {
      db.exec(column.ddl);
    } catch {
      // Duplicate column — the migration already ran. Idempotent no-op.
    }
  }
}

// Additive columns on `sessions` (plan §1a / finding 1). Idempotent like the
// run columns.
//
// - inventory_unavailable: the OUT-OF-BAND terminal cannot-fit reason (e.g.
//   `limit_too_small`) the remote reports on its poll when the server's
//   inventory limit is below the minimum viable request, so the state
//   reaches the server even though no inventory upload can ever be accepted.
const SESSION_COLUMNS: { name: string; ddl: string }[] = [
  { name: "inventory_unavailable", ddl: "ALTER TABLE sessions ADD COLUMN inventory_unavailable TEXT" },
];

/** Applies the additive `sessions` columns, ignoring already-present ones. */
function migrateSessionColumns(db: Database.Database): void {
  for (const column of SESSION_COLUMNS) {
    try {
      db.exec(column.ddl);
    } catch {
      // Duplicate column — the migration already ran. Idempotent no-op.
    }
  }
}

/**
 * Opens (creating parent dirs), configures, and migrates a connection at
 * `dbPath`. Does NOT arm retention — that is the singleton's job, so
 * tests keep full control over the prune timer.
 */
export function openDatabase(dbPath: string): Database.Database {
  fs.mkdirSync(path.dirname(path.resolve(dbPath)), { recursive: true });

  const opened = new Database(dbPath);
  opened.pragma("journal_mode = WAL");
  opened.pragma("busy_timeout = 5000");
  opened.pragma("foreign_keys = ON");
  opened.exec(MIGRATIONS);
  migrateRunColumns(opened);
  migrateSessionColumns(opened);

  return opened;
}

let database: Database.Database | undefined;

/** Resolves the SQLite file path (env override, else data/harness.db). */
function resolveDbPath(): string {
  const fromEnv = process.env.TRUE_BDD_HARNESS_DB;
  if (fromEnv && fromEnv.trim() !== "") {
    return fromEnv;
  }

  return path.join(process.cwd(), "data", "harness.db");
}

/** The process-wide connection, opened + migrated + retention-armed once. */
export function db(): Database.Database {
  if (database !== undefined) {
    return database;
  }

  database = openDatabase(resolveDbPath());
  startRetention(database);

  return database;
}

/** Monotonic wall-clock milliseconds used for created/updated stamps. */
export function now(): number {
  return Date.now();
}
