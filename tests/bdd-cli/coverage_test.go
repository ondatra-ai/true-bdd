//go:build bdd

package bddcli_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/scenarios"
)

// Hand-written, and that is the point: these three are what catch a
// generated set that stopped matching the registry, so a GENERATED
// version of them would be one the generator could silence by
// regenerating it.
//
// None of them brings the harness up, so all three answer in well under
// a second — which is what makes them affordable to run on every commit
// and, for the third, on every `build tests` walk.

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

// Every step of every owned scenario binds to exactly one definition.
// `build tests` runs this test to decide which scenarios need a fix
// turn, so its answer and the run's are the same answer by construction.
func TestStepCoverage(t *testing.T) {
	scenarios.CheckStepCoverage(t)
}
