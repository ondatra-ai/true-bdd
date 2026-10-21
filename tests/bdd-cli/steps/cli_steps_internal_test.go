package steps

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// The exit-code and stdout steps note what the run was held to on the tree
// the Given step kept (#140): a run whose State keeps none is still graded.
func TestExitCodeAndStdoutStepsGradeARunWithoutAFixture(t *testing.T) {
	t.Parallel()

	state := &State{T: t, Result: &runner.RunResult{ExitCode: 1, Stdout: "REFINE COMPLETE"}}

	err := assertExitCode(state, []string{"1"})
	if err != nil {
		t.Fatalf("exit code step without a fixture: %v", err)
	}

	err = assertStdout(state, []string{"REFINE COMPLETE"})
	if err != nil {
		t.Fatalf("stdout step without a fixture: %v", err)
	}
}
