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

// TestMain returns through scenarios.Main instead of calling os.Exit
// directly, so deferred teardowns run: os.Exit skips them, and the shim
// dir and built binary would leak on every run.
func TestMain(m *testing.M) {
	os.Exit(scenarios.Main(m))
}
