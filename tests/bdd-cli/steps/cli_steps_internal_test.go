package steps

import (
	"strings"
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

// A tree with no `prep.sh` cannot have installed what the step names, and
// the failure quotes the step as written, browser or not.
func TestTheDependenciesStepNamesTheDependenciesItStates(t *testing.T) {
	t.Parallel()

	for _, step := range []struct{ capture, wording string }{
		{capture: "", wording: "the project's test dependencies are installed"},
		{capture: "browser ", wording: "the project's browser test dependencies are installed"},
	} {
		err := assertDependenciesInstalled(&State{T: t, Fixture: &runner.Fixture{}}, []string{step.capture})
		if err == nil {
			t.Fatalf("%q on a tree with no prep: want an error", step.wording)
		}

		if !strings.Contains(err.Error(), step.wording) {
			t.Errorf("the failure %q does not quote %q", err, step.wording)
		}
	}
}
