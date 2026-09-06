//go:build bdd

package bddweb_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/bdd-web/scenarios"
)

// The guard does not bring the application up, so it answers in
// milliseconds on a machine with no node, no browser and no build.

// Every scenario the registry assigns this suite has exactly one
// generated test, in the file the registry names.
func TestScenarioCoverage(t *testing.T) {
	scenarios.CheckCoverage(t)
}
