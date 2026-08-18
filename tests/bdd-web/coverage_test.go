//go:build bdd

package bddweb_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/bdd-web/scenarios"
)

// Neither guard brings the application up, so both answer in
// milliseconds on a machine with no node, no browser and no build —
// which is what lets `build tests` ask the second one on every walk.

// Every scenario the registry assigns this suite has exactly one
// generated test, in the file the registry names.
func TestScenarioCoverage(t *testing.T) {
	scenarios.CheckCoverage(t)
}

// Every step of every owned scenario binds to exactly one definition.
func TestStepCoverage(t *testing.T) {
	scenarios.CheckStepCoverage(t)
}
