package steps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// Register binds every step this suite's scenarios can use — each one
// answering what project ran, what it must exit with, print, or leave on
// disk. A scenario needing a new answer is what `build tests --fix` writes.
func Register(suite *bddgo.Suite[State]) {
	suite.Step(`^the "([^"]+)" project tree$`, prepareTree)
	// Given preconditions the scenario states and the fixture materialises
	// during tmpdir assembly: the dependency install (`prep:`) and the
	// interactive answers piped to the fix loop. Implemented below.
	suite.Step(`^the project's (.+) dependencies are installed$`, assertDependenciesInstalled)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) answers "([^"]+)" to every prompt$`,
		assertAnswersEveryPrompt)
	// A Given precondition that a checklist is narrowed to one named prompt;
	// the fixture selects it via `checklist_prompts:`, and this step verifies
	// the selection is real and singular.
	suite.Step(`^the "([^"]+)" checklist is narrowed to its "([^"]+)" prompt$`, assertChecklistNarrowed)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) runs "([^"]+)"$`, runCLI)
	suite.Step(`^the command exits with code (\d+)$`, assertExitCode)
	// Undelimited on purpose: several patterns carry double quotes of
	// their own (`msg="Refusing to start"`), and a quoting scheme would
	// force escaping into a document meant to be read.
	suite.Step(`^stdout matches (.+)$`, assertStdout)
	// The negative twin: a scenario proves an error banner never appeared —
	// the fix loop converged before it "Hit max apply attempts". Implemented
	// below.
	suite.Step(`^stdout does not match (.+)$`, assertStdoutNoMatch)
	suite.Step(`^no file outside "([^"]+)" changed$`, assertNoChangesOutside)
	// The two-directory twin of the read-only check: proves nothing moved
	// outside either of two named scratch prefixes, e.g. a `build tests
	// --fix` run touching only "tests" and "tmp", never the registry.
	suite.Step(`^no file outside "([^"]+)" and "([^"]+)" changed$`, assertNoChangesOutsideTwo)
	// The tighter twin of the read-only check: exactly one file changed
	// outside the named prefix, and it is the named file — how `us refine
	// … --fix` proves the run rewrote only its story.
	suite.Step(`^the only file changed outside "([^"]+)" is "([^"]+)"$`, assertOnlyFileChangedOutside)
	// File-effect assertions read the run's structural diff (and, for line
	// counts, the file itself) rather than stdout. Implemented in file_steps.go.
	suite.Step(`^the file "([^"]+)" is created$`, assertFileCreated)
	suite.Step(`^the file "([^"]+)" is modified$`, assertFileModified)
	suite.Step(`^the file "([^"]+)" is unchanged$`, assertFileUnchanged)
	suite.Step(`^the file "([^"]+)" has exactly (\d+) lines?$`, assertFileLineCount)
	suite.Step(`^the file "([^"]+)" matches (.+)$`, assertFileMatches)
	// A count-of-files-matching-a-glob assertion, distinct from the named-path
	// `is created` above: the fix loop chooses the new file's name, so a
	// scenario can only pin how many .go files appeared under a directory.
	suite.Step(`^exactly (\d+) files? matching "([^"]+)" (?:is|are) created$`, assertFilesMatchingCreated)
	// The negative twin of the count assertion above: proves the run created
	// NO file matching a glob. Implemented in file_steps.go.
	suite.Step(`^no files? matching "([^"]+)" (?:is|are) created$`, assertNoFileMatchingCreated)
	// Story-shape assertions read a story a `us create … --fix` run authored:
	// its id, acceptance-criteria count and shape, forbidden vocabulary.
	// Filenames are resolved by glob since the fix loop names the file. See story_steps.go.
	suite.Step(`^the story "([^"]+)" has id "([^"]+)"$`, assertStoryID)
	suite.Step(`^the story "([^"]+)" has at least (\d+) acceptance criteria$`, assertStoryCriteriaCount)
	suite.Step(`^the story "([^"]+)" has (\d+) acceptance criteria$`, assertStoryCriteriaExactCount)
	suite.Step(`^every acceptance criterion in the story "([^"]+)" has an id matching (.+) and a non-empty description$`,
		assertStoryCriteriaWellFormed)
	suite.Step(`^every acceptance criterion in the story "([^"]+)" has given, when and then steps$`,
		assertStoryCriteriaHaveSteps)
	suite.Step(`^the "([^"]+)" clause of the story "([^"]+)" does not match (.+)$`, assertStoryClauseNoMatch)
	suite.Step(`^the description of acceptance criterion "([^"]+)" of the story "([^"]+)" does not match (.+)$`,
		assertStoryCriterionDescNoMatch)
	suite.Step(`^the description of acceptance criterion "([^"]+)" of the story "([^"]+)" matches (.+)$`,
		assertStoryCriterionDescMatch)
	// The step-text twin of the description assertions above: checks a
	// named criterion's Given/When/Then wording instead of its description,
	// e.g. that steps name an observable action rather than a forbidden verb.
	suite.Step(`^the steps of acceptance criterion "([^"]+)" of the story "([^"]+)" do not match (.+)$`,
		assertStoryCriterionStepsNoMatch)
	// Registry-executability assertion: after `build tests --fix`, is the
	// inner project's named scenario now bound by step definitions? Reads
	// the inner registry and generated Go source under the tmpdir. See registry_steps.go.
	suite.Step(`^every step of scenario "([^"]+)" in "([^"]+)" is matched by a step definition under "([^"]+)"$`,
		assertScenarioStepsMatched)
	// Registry-origin assertions read the registry a `us apply … --fix` run
	// writes and ask its LINEAGE: which acceptance criterion an entry
	// descends from, which story it cites, its serialized content. See registry_origin_steps.go.
	suite.Step(`^exactly (\d+) scenarios? in "([^"]+)" comes? from acceptance criterion "([^"]+)"$`,
		assertRegistryScenarioCount)
	suite.Step(`^the scenario from acceptance criterion "([^"]+)" in "([^"]+)" cites the story "([^"]+)"$`,
		assertRegistryScenarioCitesStory)
	suite.Step(`^the scenario from acceptance criterion "([^"]+)" in "([^"]+)" matches (.+)$`,
		assertRegistryScenarioMatches)
	// The duplicate-collapse invariant stated positively: no two registry
	// entries carry the same merged_steps. Implemented in
	// registry_origin_steps.go.
	suite.Step(`^no two scenarios in "([^"]+)" share the same steps$`, assertNoDuplicateScenarioSteps)
	// Engine-behaviour assertions read the CLI's own structured log rather
	// than its stdout: which framework runners it spawned and how many, and
	// how many model turns it dispatched. Implemented in engine_log_steps.go.
	suite.Step(`^the engine spawned the test runner "([^"]+)"$`, assertTestRunnerSpawned)
	suite.Step(`^the engine spawned a test runner with arguments matching (.+)$`, assertTestRunnerArgs)
	suite.Step(`^the engine spawned exactly (\d+) test runners?$`, assertTestRunnerCount)
	suite.Step(`^the engine spawned no test runners?$`, assertNoTestRunnerSpawned)
	suite.Step(`^the engine dispatched (\d+|no) AI turns?$`, assertAITurnCount)
	// The independent outcome check: the harness itself runs a command the
	// scenario names, in a directory under the run's tmpdir — e.g. the host
	// suite the fix loop was meant to make pass. See command_steps.go.
	suite.Step(`^the command "([^"]+)" run in "([^"]+)" exits with code (\d+)$`, assertCommandExitCode)
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

// resolveProxy decides where this scenario's AI calls come from: live is
// unmediated, replay REFUSES a scenario with no recording (never skips),
// and record writes to staging, published only once the scenario passes.
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
			return ProxySetup{}, s.fail("%w: %s\n%s", ErrNoCassettes, name, RecordHint(s.Scenario))
		}

		golden, goldenErr := runner.ReadGolden(setup.Cassettes)
		if goldenErr != nil {
			return ProxySetup{}, s.fail("%w: %s: %w\n%s",
				ErrNoGolden, name, goldenErr, RecordHint(s.Scenario))
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

// runCLI executes the invocation the scenario names, once. The captured
// role is discarded — validated by the pattern itself, with nothing else
// for the harness to do with it.
func runCLI(state *State, args []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	command := args[1]

	err := state.Fixture.UseCommand(command)
	if err != nil {
		return state.fail("%w", err)
	}

	timeout := state.runTimeout()

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

// ErrNoDependencyInstall means a scenario states the project's
// dependencies are installed, but the fixture declares no prep commands
// to install them.
var ErrNoDependencyInstall = errors.New(
	"the fixture declares no prep commands to install the project's dependencies")

// assertDependenciesInstalled verifies the Given precondition is real: at
// Given time there is no tmpdir yet to install into (prep runs later,
// during tmpdir assembly), so this only checks the fixture declares prep.
func assertDependenciesInstalled(state *State, args []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	kind := args[0]

	if len(state.Fixture.PrepCmds) == 0 {
		return state.fail(
			"the scenario says the project's %s dependencies are installed, but %w",
			kind, ErrNoDependencyInstall)
	}

	return nil
}

// everyPromptAnswers is how many copies of the answer the step feeds the
// fix loop; picked generously since surplus lines are harmless (EOF exits
// the CLI cleanly).
const everyPromptAnswers = 64

// assertAnswersEveryPrompt sets the fixture's stdin to the given answer,
// repeated, before the When step runs the CLI and pipes it to the
// subprocess. The captured role is discarded — validated by the pattern.
func assertAnswersEveryPrompt(state *State, args []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	answer := args[1]

	state.Fixture.Stdin = []byte(strings.Repeat(answer+"\n", everyPromptAnswers))

	return nil
}

// ErrNoChecklistFilter means a scenario says a checklist is narrowed to
// one prompt, but the fixture declares no checklist_prompts selection
// for it — so the precondition is not actually established.
var ErrNoChecklistFilter = errors.New(
	"the fixture declares no checklist_prompts selection for this checklist")

// assertChecklistNarrowed verifies the Given precondition is real: at
// Given time there is no narrowed checklist on disk yet (prep rewrites it
// later), so this only checks the fixture selects exactly the named prompt.
func assertChecklistNarrowed(state *State, args []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	stem := args[0]
	prompt := args[1]

	snippets, ok := state.Fixture.ChecklistPrompts[stem]
	if !ok || len(snippets) == 0 {
		return state.fail(
			"the scenario says the %q checklist is narrowed to its %q prompt, but %w",
			stem, prompt, ErrNoChecklistFilter)
	}

	if len(snippets) != 1 {
		return state.fail(
			"expected the %q checklist to be narrowed to exactly the %q prompt, but the fixture "+
				"selects %d prompts: %v", stem, prompt, len(snippets), snippets)
	}

	if snippets[0] != prompt {
		return state.fail(
			"expected the %q checklist to be narrowed to the %q prompt, but the fixture selects %q",
			stem, prompt, snippets[0])
	}

	return nil
}

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

// assertStdoutNoMatch is the negative twin of assertStdout: proves stdout
// does NOT match a pattern. Unlike assertStdout it does not append to
// fixture.StdoutRegexes — that list holds expected matches, not exclusions.
func assertStdoutNoMatch(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	pattern, err := regexp.Compile(args[0])
	if err != nil {
		return state.fail("stdout pattern %q does not compile: %w", args[0], err)
	}

	if pattern.MatchString(state.Result.Stdout) {
		return state.fail(
			"expected stdout not to match %q, but it did:\n%s", args[0], state.Result.Stdout)
	}

	return nil
}

// underPrefix reports whether a path is the named dir or something inside
// it. The trailing separator matters: a raw prefix test would let a
// sibling like `tmp-scratch.yaml` be wrongly excused by the "tmp" prefix.
func underPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// changesOutside returns the run's diff entries under none of the named
// prefixes, shared by all three scope assertions below so their common
// rule (see underPrefix) lives in one loop, not three.
func changesOutside(state *State, prefixes ...string) ([]string, []string) {
	var paths, detail []string

	for _, change := range state.Result.Diff {
		if slices.ContainsFunc(prefixes, func(prefix string) bool {
			return underPrefix(change.Path, prefix)
		}) {
			continue
		}

		paths = append(paths, change.Path)
		detail = append(detail, fmt.Sprintf("%s %s", change.Kind, change.Path))
	}

	return paths, detail
}

// assertNoChangesOutside pins a read-only invocation: nothing outside
// the named prefix moved.
func assertNoChangesOutside(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	prefix := args[0]

	_, offenders := changesOutside(state, prefix)
	if len(offenders) > 0 {
		return state.fail("%d file(s) changed outside %q: %v", len(offenders), prefix, offenders)
	}

	return nil
}

// assertNoChangesOutsideTwo is the two-prefix twin of
// assertNoChangesOutside: nothing moved outside EITHER named prefix, e.g.
// a `build tests --fix` run touching only "tests" and "tmp".
func assertNoChangesOutsideTwo(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	first := args[0]
	second := args[1]

	_, offenders := changesOutside(state, first, second)
	if len(offenders) > 0 {
		return state.fail(
			"%d file(s) changed outside %q and %q: %v",
			len(offenders), first, second, offenders)
	}

	return nil
}

// assertOnlyFileChangedOutside is the tighter twin of
// assertNoChangesOutside: exactly one file changed outside the named
// prefix, and it is the named file — e.g. the story `us refine … --fix` rewrote.
func assertOnlyFileChangedOutside(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	prefix := args[0]
	want := args[1]

	outsidePaths, outsideDetail := changesOutside(state, prefix)

	switch {
	case len(outsidePaths) == 1 && outsidePaths[0] == want:
		return nil
	case len(outsidePaths) == 0:
		return state.fail(
			"expected the only file changed outside %q to be %q, but nothing outside it changed",
			prefix, want)
	default:
		return state.fail(
			"expected the only file changed outside %q to be %q, but %d file(s) changed outside it: %v",
			prefix, want, len(outsideDetail), outsideDetail)
	}
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
