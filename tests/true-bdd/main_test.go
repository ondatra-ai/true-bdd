//go:build bdd

// Package truebdd_test is the true-bdd suite: one generated Go test per
// registry scenario whose `service:` is true-bdd, plus the three guards
// that keep the generated set honest.
package truebdd_test

import (
	"os"
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/true-bdd/scenarios"
)

// TestMain returns through scenarios.Main instead of calling os.Exit
// directly, so deferred teardowns run: os.Exit skips them, and the shim
// dir and built binary would leak on every run.
func TestMain(m *testing.M) {
	os.Exit(scenarios.Main(m))
}
