//go:build bdd

// Package bddcli_test is the bdd-cli suite: one generated Go test per
// registry scenario whose `service:` is bdd-cli, plus the three guards
// that keep the generated set honest.
package bddcli_test

import (
	"os"
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/scenarios"
)

// TestMain returns through scenarios.Main rather than exiting inside it,
// so the teardowns registered while the harness came up actually run —
// os.Exit skips deferred work, and the shim directory and the built
// binary would otherwise outlive every run.
func TestMain(m *testing.M) {
	os.Exit(scenarios.Main(m))
}
