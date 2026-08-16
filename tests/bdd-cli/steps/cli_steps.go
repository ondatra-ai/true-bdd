package steps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// Register binds every step this suite's scenarios can use.
//
// Six definitions cover all of them, which is not an accident of the
// current registry: a bdd-cli scenario IS "given this project, run the
// CLI once, and this is what it must exit with and print". A scenario
// needing a seventh is a scenario claiming something new, and that is
// exactly when a new definition is worth writing.
func Register(suite *bddgo.Suite[State]) {
	suite.Step(`^the "([^"]+)" project tree$`, prepareTree)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) runs "([^"]+)"$`, runCLI)
	suite.Step(`^the command exits with code (\d+)$`, assertExitCode)
	// Undelimited on purpose: several patterns carry double quotes of
	// their own (`msg="Refusing to start"`), and a quoting scheme would
	// force escaping into a document meant to be read.
	suite.Step(`^stdout matches (.+)$`, assertStdout)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) answers (.+)$`, noteAnswers)
	suite.Step(`^no file outside "([^"]+)" changed$`, assertNoChangesOutside)
}

// prepareTree loads the named fixture tree and resolves how this run
// reaches the AI CLIs. Nothing is executed yet — the tmpdir is built by
// runner.Execute, which needs the command the When step has not named.
func prepareTree(state *State, args []string) error {
	name := args[0]

	fixture, err := runner.LoadFixture(state.Harness.FixtureDir(name))
	if err != nil {
		return state.fail("loading the %q tree: %w", name, err)
	}

	state.Fixture = fixture

	proxy, err := state.resolveProxy(name)
	if err != nil {
		return err
	}

	state.Proxy = proxy

	return nil
}

// resolveProxy decides where this scenario's AI calls come from.
//
// Live returns the zero value: no shim, no env, byte-for-byte an
// unmediated run. Replay REFUSES a scenario with no recording rather
// than skipping it. Record writes into staging, which is published only
// once the whole scenario passes.
func (s *State) resolveProxy(name string) (ProxySetup, error) {
	if s.Harness.Mode == runner.ProxyModeLive {
		return ProxySetup{}, nil
	}

	cassettes, err := s.Harness.CassetteDir(name)
	if err != nil {
		return ProxySetup{}, s.fail("resolving cassettes dir: %w", err)
	}

	setup := ProxySetup{Cassettes: cassettes}

	if s.Harness.Mode == runner.ProxyModeReplay {
		_, statErr := os.Stat(setup.Cassettes)
		if statErr != nil {
			return ProxySetup{}, s.fail("%w: %s\n%s", ErrNoCassettes, name, RecordHint(name))
		}

		golden, goldenErr := runner.ReadGolden(setup.Cassettes)
		if goldenErr != nil {
			return ProxySetup{}, s.fail("%w: %s: %w\n%s",
				ErrNoGolden, name, goldenErr, RecordHint(name))
		}

		setup.Golden = golden
	}

	if s.Harness.Mode == runner.ProxyModeRecord {
		staging, stagingErr := s.Harness.prepareStaging(name)
		if stagingErr != nil {
			return ProxySetup{}, s.fail("preparing staging cassettes: %w", stagingErr)
		}

		setup.Staging = staging
		setup.Cassettes = staging
	}

	// Cursor state lives under the run's engine-scratch tmp/ — inside
	// the snapshot window but excluded from the graded diff, and wiped
	// with the run dir.
	setup.StateDir = filepath.Join(
		runner.RunDir(s.Harness.SessionRoot, name), "tmp", "aiproxy-state")
	setup.Env = runner.AIProxyEnv(
		s.Harness.Mode, s.Harness.ShimDir, setup.Cassettes, setup.StateDir)

	return setup, nil
}

// runCLI executes the invocation the scenario names, once.
//
// The role is captured and discarded: it is the scenario's own statement
// of who does this, checked against the product document's role list by
// the pattern itself, and there is nothing for the harness to do with it
// beyond refusing a role nobody declared.
func runCLI(state *State, args []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	command := args[1]

	err := state.Fixture.UseCommand(command)
	if err != nil {
		return state.fail("%w", err)
	}

	timeout := state.Harness.Timeout
	if state.Fixture.Timeout > 0 {
		timeout = state.Fixture.Timeout
	}

	state.T.Logf("running %q (%s, timeout %s) — this can take several minutes",
		command, state.Fixture.Name, timeout)

	result, err := runner.Execute(context.Background(), state.Fixture,
		state.Harness.BinPath, state.Harness.SessionRoot, timeout, state.Proxy.Env...)

	// Observed before the branch: a run that errored still has a diff
	// worth recording, and the tmpdir path is what makes it findable.
	state.Recorder.ObserveRun(result, err)
	state.Result = result

	if err != nil {
		state.dumpRun()

		return state.fail("executing %q: %w", command, err)
	}

	return nil
}

// ErrNoTreePrepared is returned when a scenario runs the CLI without
// naming a project tree first.
var ErrNoTreePrepared = errors.New("no Given step prepared a project tree")

// ErrNoRun is returned when a Then step asserts on a run that never
// happened.
var ErrNoRun = errors.New("no When step ran the CLI")

func assertExitCode(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	want, err := strconv.Atoi(args[0])
	if err != nil {
		return state.fail("exit code %q is not a number: %w", args[0], err)
	}

	// Carried onto the fixture so the run's manifest snapshot — and
	// therefore the report's expected-vs-actual column — states what this
	// run was actually held to.
	state.Fixture.ExpectedExitCode = want

	err = runner.CheckExitCode(state.Result.ExitCode, want)
	if err != nil {
		state.dumpRun()

		return state.fail("%w", err)
	}

	return nil
}

func assertStdout(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	pattern, err := regexp.Compile(args[0])
	if err != nil {
		return state.fail("stdout pattern %q does not compile: %w", args[0], err)
	}

	state.Fixture.StdoutRegexes = append(state.Fixture.StdoutRegexes, pattern)

	err = runner.CheckStdout(state.Result.Stdout, pattern)
	if err != nil {
		return state.fail("%w", err)
	}

	return nil
}

// noteAnswers records that a scenario drives the interactive fix loop.
//
// The keystrokes themselves stay in fixture.yaml's `answers:` — they are
// a transcript, not a claim, and a scenario reading "answers 1, 1, 1, 1,
// 1, 1, 1, 1" says nothing a reader can check. What the step does say,
// and what belongs in the registry, is that this run is interactive at
// all.
func noteAnswers(state *State, args []string) error {
	if len(state.Fixture.Stdin) == 0 {
		return state.fail("the scenario says the %s answers %s, but the tree pipes no stdin",
			args[0], args[1])
	}

	return nil
}

// assertNoChangesOutside pins a read-only invocation: nothing outside
// the named prefix moved.
func assertNoChangesOutside(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	prefix := args[0]

	var offenders []string

	for _, change := range state.Result.Diff {
		if change.Path == prefix || len(change.Path) > len(prefix) &&
			change.Path[:len(prefix)] == prefix {
			continue
		}

		offenders = append(offenders, fmt.Sprintf("%s %s", change.Kind, change.Path))
	}

	if len(offenders) > 0 {
		return state.fail("%d file(s) changed outside %q: %v", len(offenders), prefix, offenders)
	}

	return nil
}

// dumpRun prints where the run's evidence lives. Called on the failures
// worth investigating, not on every assertion: the transcript files
// outlive the test process, and the path to them is the useful half.
func (s *State) dumpRun() {
	if s.Result == nil {
		return
	}

	s.T.Logf("tmpdir preserved at: %s", s.Result.TmpDir)
	s.T.Logf("exit code: %d", s.Result.ExitCode)

	if s.Result.StdoutFile != "" {
		s.T.Logf("cli stdout: %s (%d bytes)", s.Result.StdoutFile, len(s.Result.Stdout))
	}

	if s.Result.StderrFile != "" {
		s.T.Logf("cli stderr: %s (%d bytes)", s.Result.StderrFile, len(s.Result.Stderr))
	}

	if s.Result.Stderr != "" {
		s.T.Logf("stderr:\n%s", s.Result.Stderr)
	}

	s.T.Logf("file diff (%d entries):", len(s.Result.Diff))

	for _, change := range s.Result.Diff {
		s.T.Logf("  %s %s (%d bytes)", change.Kind, change.Path, len(change.After))
	}
}
