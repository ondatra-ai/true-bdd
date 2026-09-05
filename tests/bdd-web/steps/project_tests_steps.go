package steps

import (
	"errors"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoTestCommand is returned when the host's architectural spec declares
// neither a test command nor a framework this suite knows the default of.
var ErrNoTestCommand = errors.New("the project declares no test command")

const (
	// projectTestTimeout caps the host project's own suite: it compiles from
	// source the fix run has just rewritten.
	projectTestTimeout = 10 * time.Minute
	// goTestFramework is the framework whose default command this suite knows,
	// and goTestCommand that command.
	goTestFramework = "go-test"
	goTestCommand   = "go test ./..."
)

// registerProjectTestsSteps binds the outcome clause a build-code fix scenario
// closes with: the suite the loop was driving passes when this suite runs it.
func registerProjectTestsSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the project's own tests pass when run directly$`, assertProjectTestsPass)
}

// assertProjectTestsPass runs the host project's own suite in its tree and holds
// it to exiting zero — the only reading that says the fix loop fixed anything.
func assertProjectTestsPass(state *State, _ []string) error {
	command, err := projectTestCommand(state)
	if err != nil {
		return err
	}

	finished, err := spec.Run(strings.Fields(command), cli.Options{
		Dir:     state.Tree.Dir,
		Timeout: projectTestTimeout,
	})
	if err != nil {
		return state.fail("running %q in %s: %w", command, state.Tree.Dir, err)
	}

	if finished.Code != 0 {
		return state.fail("%q exited with code %d in %s, want 0\n%s\n%s",
			command, finished.Code, state.Tree.Dir, finished.Stdout, finished.Stderr)
	}

	return nil
}

// projectTestCommand is what the host's architectural spec says its tests are
// run by: the replay command it declares, or the default of the framework it
// names — so a fixture states the command rather than this suite guessing it.
func projectTestCommand(state *State) (string, error) {
	raw, err := fixtureFile(state, canonicalArchitectureRel)
	if err != nil {
		return "", err
	}

	declared, err := scalarField(raw, "replay")
	if err == nil {
		return declared, nil
	}

	framework, err := scalarField(raw, "framework")
	if err != nil || framework != goTestFramework {
		return "", state.fail("%w: %s declares no commands.replay, and its framework is "+
			"not %q", ErrNoTestCommand, canonicalArchitectureRel, goTestFramework)
	}

	return goTestCommand, nil
}
