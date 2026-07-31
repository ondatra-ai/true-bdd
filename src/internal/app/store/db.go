package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo)
)

// errCorrupt is returned when the boot integrity check fails.
var errCorrupt = errors.New("store integrity check failed")

const (
	dirPerm  = 0o700
	filePerm = 0o600
	// byteBudget is the per-run logical output cap (256 KiB, UTF-8 bytes).
	byteBudget = 256 * 1024
	// terminalRetention keeps the latest N terminal runs per project.
	terminalRetention = 50
	// receiptRetention keeps the latest N dispatch receipts per owner.
	receiptRetention = 200
	// tokenMaxBytes bounds a dispatch client token.
	tokenMaxBytes = 512
	// maxOpenConns caps the SQLite pool: WAL allows concurrent readers with a
	// single writer, and the in-process write mutex serializes writers anyway.
	maxOpenConns = 8
)

// DB is the concrete v2 CLI-owned state store: a per-project SQLite database
// that owns ALL run/session state (plan §1). It is the production type behind
// the seam `Store` interface (wired via the test bridge). A single in-process
// write mutex serializes every write transaction so the writer-actor sequencing
// (unique, contiguous per-run seqs) and the dispatch/answer transactions never
// interleave; a per-connection busy timeout handles the cross-process case.
type DB struct {
	sql     *sql.DB
	dbPath  string
	writeMu sync.Mutex
}

// Open opens (creating + migrating if necessary) a per-project store at
// dbPath. The directory is 0700 and the db/WAL/SHM files 0600 (plan §1.1);
// foreign keys, WAL, and a busy timeout are set on EVERY connection via the
// DSN; a quick_check runs at boot and sequential checksummed migrations apply
// under an exclusive transaction.
func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)

	err := os.MkdirAll(dir, dirPerm)
	if err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}

	_ = os.Chmod(dir, dirPerm)

	dsn := "file:" + dbPath +
		"?_pragma=busy_timeout(10000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)

	database := &DB{sql: sqlDB, dbPath: dbPath}

	err = database.bootCheck()
	if err != nil {
		_ = sqlDB.Close()

		return nil, err
	}

	err = database.migrate()
	if err != nil {
		_ = sqlDB.Close()

		return nil, err
	}

	database.tightenPerms()

	return database, nil
}

// Close closes the underlying pool.
func (db *DB) Close() error {
	err := db.sql.Close()
	if err != nil {
		return fmt.Errorf("close store: %w", err)
	}

	return nil
}

// Exec runs a statement with no result rows (constraint-test surface).
func (db *DB) Exec(query string, args ...any) error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	ctx := context.Background()

	_, err := db.sql.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}

// Scalar runs a query returning a single integer (pragma / count surface).
func (db *DB) Scalar(query string, args ...any) (int64, error) {
	ctx := context.Background()

	var value int64

	err := db.sql.QueryRowContext(ctx, query, args...).Scan(&value)
	if err != nil {
		return 0, fmt.Errorf("scalar: %w", err)
	}

	return value, nil
}

// bootCheck fails visibly on a corrupt database (plan §1.1).
func (db *DB) bootCheck() error {
	ctx := context.Background()

	var result string
	// quick_check returns "ok" on a healthy db; a fresh (empty) file also
	// returns "ok".
	err := db.sql.QueryRowContext(ctx, `PRAGMA quick_check(1)`).Scan(&result)
	if err != nil {
		return fmt.Errorf("quick_check: %w", err)
	}

	if result != "ok" {
		return fmt.Errorf("%w: %s", errCorrupt, result)
	}

	return nil
}

// tightenPerms restricts the db + WAL/SHM sidecars to 0600.
func (db *DB) tightenPerms() {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Chmod(db.dbPath+suffix, filePerm)
	}
}

// now returns a monotonic-enough nanosecond timestamp for lifecycle ordering.
func now() int64 {
	return time.Now().UnixNano()
}
