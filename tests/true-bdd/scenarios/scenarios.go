//go:build bdd

// Package scenarios binds the harness to TrueBDD's State and definitions.
package scenarios

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/true-bdd/steps"
)

//nolint:gochecknoglobals // one suite per test binary; the door owns its lifecycle.
var suite = testkit.NewSuite("true-bdd", []string{"claude", "codex"}, steps.Register,
	func(run *testkit.ScenarioRun) *steps.State { return &steps.State{ScenarioRun: run} })

// Main returns after the door has torn down the suite's shared resources.
func Main(m *testing.M) int { return suite.Main(m) }

// New selects the Scenario a generated test drives.
func New(t *testing.T, scenarioID string) *bddgo.Run[steps.State] {
	t.Helper()

	return suite.New(t, scenarioID)
}

// CheckCoverage compares the Registry and generated tests without harness bring-up.
func CheckCoverage(t *testing.T) {
	t.Helper()
	suite.CheckCoverage(t)
}

// CheckFixtureTrees refuses a fixture no Scenario names.
func CheckFixtureTrees(t *testing.T) {
	t.Helper()
	suite.CheckFixtureTrees(t)
}
