//go:build bdd

package bddcli_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/scenarios"
)

// Hand-written, not generated: a generated version of these two could be
// silenced by regenerating it. Neither brings up the harness, so both
// answer in well under a second.

// Every scenario the registry assigns this suite has exactly one
// generated test, in the file the registry names, and every generated
// test names a scenario that exists.
func TestScenarioCoverage(t *testing.T) {
	scenarios.CheckCoverage(t)
}

// Every fixture tree is named by a scenario. A tree nobody runs looks
// exactly like a tree that passes.
func TestFixtureTreesArePaired(t *testing.T) {
	scenarios.CheckFixtureTrees(t)
}
