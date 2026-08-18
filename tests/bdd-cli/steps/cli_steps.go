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

// Register binds every step this suite's scenarios can use.
//
// The shape underneath is still one shape: a bdd-cli scenario IS "given
// this project, run the CLI once, and this is what it must exit with,
// print and leave on disk". Every definition below is an answer to one
// of those four questions, and a scenario needing a new one is a
// scenario claiming something none of them can say — which is exactly
// when `build tests --fix` writes one.
func Register(suite *bddgo.Suite[State]) {
	suite.Step(`^the "([^"]+)" project tree$`, prepareTree)
	// Given preconditions the scenario states and the fixture materialises
	// during tmpdir assembly: the dependency install (`prep:`) and the
	// interactive answers piped to the fix loop. Implemented below.
	suite.Step(`^the project's (.+) dependencies are installed$`, assertDependenciesInstalled)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) answers "([^"]+)" to every prompt$`,
		assertAnswersEveryPrompt)
	// A Given precondition that a single named prompt of a checklist is the
	// only one that runs. The fixture trims the shipped checklist to that one
	// prompt via `checklist_prompts:`; this step verifies the selection is
	// real and singular. Implemented in cli_steps.go.
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
	// The two-directory twin: a scenario proves nothing moved outside
	// EITHER of two named scratch prefixes — how a `build tests --fix`
	// scenario proves the fix loop wrote only into the suite's steps tree
	// ("tests") and the engine scratch ("tmp"), and touched the registry
	// nowhere. Implemented in cli_steps.go.
	suite.Step(`^no file outside "([^"]+)" and "([^"]+)" changed$`, assertNoChangesOutsideTwo)
	// The tighter twin of the read-only check above: EXACTLY ONE file
	// changed outside the named prefix, and it is the named file — how a
	// `us refine … --fix` scenario proves the run rewrote its story and
	// nothing else. Implemented in cli_steps.go.
	suite.Step(`^the only file changed outside "([^"]+)" is "([^"]+)"$`, assertOnlyFileChangedOutside)
	// File-effect assertions read the run's structural diff — and, for a
	// line count, the file on disk — rather than stdout: what the fix loop
	// wrote, what it left alone, and how large a file it produced.
	// Implemented in file_steps.go.
	suite.Step(`^the file "([^"]+)" is created$`, assertFileCreated)
	suite.Step(`^the file "([^"]+)" is modified$`, assertFileModified)
	suite.Step(`^the file "([^"]+)" is unchanged$`, assertFileUnchanged)
	suite.Step(`^the file "([^"]+)" has exactly (\d+) lines?$`, assertFileLineCount)
	suite.Step(`^the file "([^"]+)" matches (.+)$`, assertFileMatches)
	// A count-of-files-matching-a-glob assertion, distinct from the named-path
	// `is created` above: the fix loop chooses the new file's name, so a
	// scenario can only pin how many .go files appeared under a directory.
	suite.Step(`^exactly (\d+) files? matching "([^"]+)" (?:is|are) created$`, assertFilesMatchingCreated)
	// The negative twin of the count assertion above: a scenario proves the
	// run created NO file matching a glob — here, that a relocated registry
	// left the old conventional docs/scenarios.yaml uncreated. Implemented
	// in file_steps.go.
	suite.Step(`^no files? matching "([^"]+)" (?:is|are) created$`, assertNoFileMatchingCreated)
	// Story-shape assertions read a story document a `us create … --fix`
	// run authored — the id it carries, how many acceptance criteria it
	// has, that each criterion is well-formed, and that a named clause
	// avoids a forbidden vocabulary. The story's filename is the fix loop's
	// to choose, so every one names the file by glob and resolves it to the
	// single matching file on disk. Implemented in story_steps.go.
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
	// The step-text twin of the two description assertions above: it reads
	// the wording of a NAMED criterion's Given/When/Then steps rather than
	// its description, and proves the fix loop drove a forbidden action verb
	// out of them until each step names an observable action. Implemented in
	// story_steps.go.
	suite.Step(`^the steps of acceptance criterion "([^"]+)" of the story "([^"]+)" do not match (.+)$`,
		assertStoryCriterionStepsNoMatch)
	// Registry-executability assertion: after a `build tests --fix` run, is
	// the inner project's named scenario now bound by definitions in its
	// suite's steps package? Reads the inner registry and the Go source the
	// fix loop wrote, both under the run's tmpdir. Implemented in
	// registry_steps.go.
	suite.Step(`^every step of scenario "([^"]+)" in "([^"]+)" is matched by a step definition under "([^"]+)"$`,
		assertScenarioStepsMatched)
	// Registry-origin assertions read the central registry a `us apply
	// … --fix` run writes and ask what LINEAGE each entry carries: how many
	// entries descend from one acceptance criterion, which story an entry
	// cites, and what an entry's serialized content says. Implemented in
	// registry_origin_steps.go.
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

// ErrNoDependencyInstall is returned when a scenario states the project's
// dependencies are installed but the fixture backing it declares no prep
// commands to install them — so nothing the run does actually satisfies
// the precondition the scenario asserts.
var ErrNoDependencyInstall = errors.New(
	"the fixture declares no prep commands to install the project's dependencies")

// assertDependenciesInstalled binds the Given precondition that the
// project's dependencies are installed before the CLI runs. The install
// itself is external scaffolding — an npm install, a browser download —
// that the fixture declares under `prep:` and runner.Execute runs while
// assembling the tmpdir, before the pre-run snapshot; there is no tmpdir
// yet at Given time for this step to install into. So the step verifies
// the precondition is real: the fixture backing this scenario declares
// the prep that installs them. The dependency kind ("browser test") is a
// capture group so one definition serves every scenario naming a
// different kind.
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

// everyPromptAnswers is how many copies of the answer the step supplies
// the interactive fix loop. "every prompt" is unbounded; the loop reads
// one line per prompt and stops when it is done, and surplus lines are
// harmless — EOF on stdin makes the CLI exit cleanly — so a generous
// fixed supply answers every prompt any single fix loop raises.
const everyPromptAnswers = 64

// assertAnswersEveryPrompt binds the Given step that a role answers the
// same choice to every interactive prompt. The scenario is the source of
// the run's interactive input: the step sets the CLI's stdin to a block
// of the captured answer, one line per prompt, so the `--fix` loop takes
// that option at every menu. It is set on the fixture before the When
// step runs the CLI, which is where runner.Execute pipes it to the
// subprocess's stdin. The role is captured and discarded — the scenario's
// statement of who acts, checked against the declared roles by the
// pattern itself. The answer is a capture group so one definition serves
// every scenario whatever choice it names.
func assertAnswersEveryPrompt(state *State, args []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	answer := args[1]

	state.Fixture.Stdin = []byte(strings.Repeat(answer+"\n", everyPromptAnswers))

	return nil
}

// ErrNoChecklistFilter is returned when a scenario states that only one
// prompt of a checklist runs, but the fixture backing it declares no
// checklist_prompts selection for that checklist — so nothing trims the
// shipped checklist down to that one prompt, and the precondition the
// scenario asserts is not actually established.
var ErrNoChecklistFilter = errors.New(
	"the fixture declares no checklist_prompts selection for this checklist")

// assertChecklistNarrowed binds the Given precondition that a checklist is
// narrowed to a single named prompt for this run. The narrowing itself is
// external scaffolding the fixture declares under `checklist_prompts:` and
// runner prep performs — it rewrites the overlaid checklist down to only the
// prompts whose Q text carries a declared snippet, so there is no narrowed
// checklist on disk yet at Given time for this step to inspect. So the step
// verifies the precondition is real and singular: the fixture backing this
// scenario selects exactly one prompt for the named checklist, and it is the
// named one. The checklist stem and the prompt are both capture groups so
// one definition serves every scenario naming a different checklist or
// prompt.
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

// assertStdoutNoMatch pins that the run's stdout does NOT match a regexp —
// the negative twin of assertStdout. A scenario uses it to prove an error
// banner never appeared: the fix loop converged before it "Hit max apply
// attempts". The pattern runs undelimited to the end of the line, exactly
// like the positive form, and is a capture group so one definition serves
// every scenario naming a different pattern. It is not recorded on the
// fixture's StdoutRegexes, which is a list of expected MATCHES; a negative
// clause is asserted directly against Result.Stdout instead.
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

// underPrefix reports whether a diff path is the named directory or
// something inside it.
//
// The separator is not optional. A raw string-prefix test also swallows a
// sibling whose name merely STARTS with the prefix — with the "tmp" every
// scope assertion uses, a fix applier that wrote `tmp-scratch.yaml` at the
// run root would be excused by exactly the steps written to catch it.
func underPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// changesOutside returns the run's diff entries under none of the named
// prefixes: their paths, and the "<kind> <path>" lines a failure quotes.
//
// One scope walk for all three assertions below, because the scope RULE
// is what they share. underPrefix exists to stop a sibling like
// `tmp-scratch.yaml` being excused by the `tmp` prefix, and a rule with
// that much reasoning behind it must not live in three loops where a
// later refinement can reach only one of them.
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

// assertNoChangesOutsideTwo pins that nothing moved outside EITHER of two
// named prefixes — the two-directory twin of assertNoChangesOutside. A
// `build tests --fix` scenario uses it to prove the fix loop wrote only
// into the suite's own steps tree ("tests") and the engine scratch
// ("tmp"), leaving every other path — the scenario registry above all —
// untouched. A change under neither prefix is an offender and is named,
// with its kind, in the failure. Both prefixes are capture groups so one
// definition serves every scenario naming a different pair of scratch
// directories rather than this scenario's literal line.
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

// assertOnlyFileChangedOutside pins that EXACTLY ONE file changed outside
// the named prefix, and that it is the named file. It is the tighter twin
// of assertNoChangesOutside: where that step proves a read-only invocation,
// this proves a run touched precisely one file beyond its scratch prefix —
// the story a `us refine … --fix` run rewrote, and nothing else. The change
// facts come from the run's structural diff, and the prefix and the file
// are both capture groups so one definition serves every scenario naming a
// different scratch prefix or target file.
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
