package steps

import (
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
)

// hostCommandTimeout caps one scenario-declared command. Generous because
// the same step re-runs a browser suite against the stack the CLI run left
// standing; it exists to bound a hang, not to pace a passing run.
const hostCommandTimeout = 10 * time.Minute

// The outcome check apart from the engine: the suite the fix loop left behind passes on its own.
//
//truebdd:concept then "after-run shell check"
func assertCommandExitCode(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	command := args[0]
	dirRel := args[1]

	want, err := numberSlot(args[2])
	if err != nil {
		return state.fail("exit code %q is not a number: %w", args[2], err)
	}

	argv := strings.Fields(command)
	if len(argv) == 0 {
		return state.fail("the command to run in %q is empty", dirRel)
	}

	dir, err := state.containedDir(dirRel)
	if err != nil {
		return err
	}

	state.T.Logf("running %q in %q (timeout %s) — this can take several minutes",
		command, dirRel, hostCommandTimeout)

	// CLAUDECODE is stripped for the reason runner.phase strips it: a
	// re-run host suite may itself launch an agent CLI.
	finished, err := spec.Run(argv, cli.Options{
		Dir:     dir,
		Env:     cli.Inherit().Strip("CLAUDECODE").Set("GOWORK=off"),
		Timeout: hostCommandTimeout,
	})
	if err != nil {
		return state.fail("running %q in %q: %w", command, dirRel, err)
	}

	if finished.Code != want {
		return state.fail(
			"expected %q run in %q to exit with code %d, but it exited %d\n"+
				"--- stdout ---\n%s\n--- stderr ---\n%s",
			command, dirRel, want, finished.Code, finished.Stdout, finished.Stderr)
	}

	return nil
}
