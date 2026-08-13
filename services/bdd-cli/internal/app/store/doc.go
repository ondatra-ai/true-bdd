// Package store is the v2 CLI-owned state store: a per-project SQLite
// database at <folder>/tmp/true-bdd-state.db that owns ALL run/session
// state (plan §1). It replaces the v1 server-side better-sqlite3 store; the
// stateless relay holds no database.
//
// The implementation lands here in the store phase. The tests-first suite in
// this package (schema_ddl_test.go, dispatch_test.go, answer_test.go,
// writer_retention_test.go, reconcile_test.go, lock_protocol_test.go) pins
// the target contract via the in-test interface seam declared in
// seam_test.go — so the package compiles today (this marker file) and the
// tests light up when the implementation wires the seam.
package store
