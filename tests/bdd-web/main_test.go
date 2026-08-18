//go:build bdd

// Package bddweb_test is the bdd-web suite: one generated Go test per
// registry scenario whose `service:` is bdd-web, plus the guards that
// keep the generated set honest.
package bddweb_test

import (
	"os"
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/bdd-web/scenarios"
)

// TestMain returns through scenarios.Main rather than exiting inside it,
// so the browser and the server the harness started are actually torn
// down — os.Exit skips deferred work.
func TestMain(m *testing.M) {
	os.Exit(scenarios.Main(m))
}
